package higresscontroller

import (
	"fmt"
	"strconv"

	"gopkg.in/yaml.v2"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	operatorv1alpha1 "github.com/alibaba/higress/higress-operator/api/v1alpha1"
)

func initGatewayConfigMap(cm *apiv1.ConfigMap, instance *operatorv1alpha1.HigressController) (*apiv1.ConfigMap, error) {
	*cm = apiv1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cm-" + instance.Name,
			Namespace: instance.Namespace,
			Labels:    instance.Labels,
		},
	}

	if _, err := updateGatewayConfigMapSpec(cm, instance); err != nil {
		return nil, err
	}

	return cm, nil
}

func updateGatewayConfigMapSpec(cm *apiv1.ConfigMap, instance *operatorv1alpha1.HigressController) (*apiv1.ConfigMap, error) {
	var (
		data               = map[string]string{}
		err                error
		meshNetworksBytes  []byte
		networksBytes      []byte
		meshConfigBytes    []byte
		higressConfigBytes []byte
	)

	if networksBytes, err = yaml.Marshal(instance.Spec.MeshNetworks); err == nil {
		networksMap := make(map[string]string)
		networksMap["networks"] = string(networksBytes)
		if meshNetworksBytes, err = yaml.Marshal(networksMap); err == nil {
			data["meshNetworks"] = string(meshNetworksBytes)
		}
	}

	// rootNamespace
	meshConfig := instance.Spec.MeshConfig
	if instance.Spec.EnableHigressIstio {
		if meshConfig.RootNamespace == "" {
			meshConfig.RootNamespace = instance.Spec.IstioNamespace
		}
	} else {
		meshConfig.RootNamespace = instance.Namespace
	}

	// configSources
	meshConfig.ConfigSources = append(meshConfig.ConfigSources, operatorv1alpha1.ConfigSource{
		Address: "k8s://"})

	// defaultConfig.tracing
	// defaultConfig.discoveryAddress
	if instance.Spec.MeshConfig.DefaultConfig.DiscoveryAddress == "" {
		if instance.Spec.EnableHigressIstio {
			instance.Spec.MeshConfig.DefaultConfig.DiscoveryAddress =
				fmt.Sprintf("%s.%s.svc:15012", "istiod", instance.Namespace)
		} else {
			instance.Spec.MeshConfig.DefaultConfig.DiscoveryAddress =
				fmt.Sprintf("%s.%s.svc:15012", instance.Name, instance.Namespace)
		}
	}

	if meshConfigBytes, err = yaml.Marshal(meshConfig); err == nil {
		data["mesh"] = string(meshConfigBytes)
	}

	if err != nil {
		return nil, err
	}
	hgConfig := instance.Spec.HigressConfig
	higress := HigressConfig{
		Tracing: &Tracing{
			Enable:   hgConfig.TracingEnable,
			Sampling: hgConfig.TracingSampling,
			Skywalking: &Skywalking{
				Service: hgConfig.SkywalkingAddr,
				Port:    strconv.Itoa(int(hgConfig.SkywalkingPort)),
			},
		},
		Gzip: &Gzip{
			Enable:              hgConfig.Gzip,
			MinContentLength:    1024,
			ContentType:         []string{"text/html", "application/json", "text/css", "application/javascript", "application/xhtml+xml", "image/svg+xml"},
			DisableOnEtagHeader: true,
			MemoryLevel:         5,
			WindowBits:          12,
			ChunkSize:           4096,
			CompressionLevel:    "BEST_COMPRESSION",
			CompressionStrategy: "DEFAULT_STRATEGY",
		},
		Downstream: &Downstream{
			IdleTimeout:            hgConfig.DownstreamIdleTimeout,
			MaxRequestHeadersKb:    60,
			ConnectionBufferLimits: hgConfig.DownstreamConnectionBufferLimits,
		},
		Upstream: &Upstream{
			IdleTimeout:            hgConfig.UpstreamIdleTimeout,
			ConnectionBufferLimits: hgConfig.UpstreamConnectionBufferLimits,
		},
		DisableXEnvoyHeaders: hgConfig.DisableXEnvoyHeaders,
		AddXRealIpHeader:     hgConfig.AddXRealIpHeader,
	}
	if higressConfigBytes, err = yaml.Marshal(higress); err == nil {
		data["higress"] = string(higressConfigBytes)
	}
	cm.Data = data
	return cm, nil
}

func muteConfigMap(cm *apiv1.ConfigMap, instance *operatorv1alpha1.HigressController,
	fn func(*apiv1.ConfigMap, *operatorv1alpha1.HigressController) (*apiv1.ConfigMap, error)) controllerutil.MutateFn {
	return func() error {
		if _, err := fn(cm, instance); err != nil {
			return err
		}
		return nil
	}
}

type HigressConfig struct {
	Tracing              *Tracing    `json:"tracing,omitempty"`
	Gzip                 *Gzip       `json:"gzip,omitempty"`
	Downstream           *Downstream `json:"downstream,omitempty"`
	Upstream             *Upstream   `json:"upstream,omitempty"`
	DisableXEnvoyHeaders bool        `json:"disableXEnvoyHeaders,omitempty"`
	AddXRealIpHeader     bool        `json:"addXRealIpHeader,omitempty"`
}

// Downstream configures the behavior of the downstream connection.
type Downstream struct {
	// IdleTimeout limits the time that a connection may be idle and stream idle.
	IdleTimeout uint32 `json:"idleTimeout"`
	// MaxRequestHeadersKb limits the size of request headers allowed.
	MaxRequestHeadersKb uint32 `json:"maxRequestHeadersKb,omitempty"`
	// ConnectionBufferLimits configures the buffer size limits for connections.
	ConnectionBufferLimits uint32 `json:"connectionBufferLimits,omitempty"`
	// Http2 configures HTTP/2 specific options.
	Http2 *Http2 `json:"http2,omitempty"`
	//RouteTimeout limits the time that timeout for the route.
	RouteTimeout uint32 `json:"routeTimeout"`
}

// Http2 configures HTTP/2 specific options.
type Http2 struct {
	// MaxConcurrentStreams limits the number of concurrent streams allowed.
	MaxConcurrentStreams uint32 `json:"maxConcurrentStreams,omitempty"`
	// InitialStreamWindowSize limits the initial window size of stream.
	InitialStreamWindowSize uint32 `json:"initialStreamWindowSize,omitempty"`
	// InitialConnectionWindowSize limits the initial window size of connection.
	InitialConnectionWindowSize uint32 `json:"initialConnectionWindowSize,omitempty"`
}

// Upstream configures the behavior of the upstream connection.
type Upstream struct {
	// IdleTimeout limits the time that a connection may be idle on the upstream.
	IdleTimeout uint32 `json:"idleTimeout"`
	// ConnectionBufferLimits configures the buffer size limits for connections.
	ConnectionBufferLimits uint32 `json:"connectionBufferLimits,omitempty"`
}
type Tracing struct {
	// Flag to control trace
	Enable bool `json:"enable,omitempty"`
	// The percentage of requests (0.0 - 100.0) that will be randomly selected for trace generation,
	// if not requested by the client or not forced. Default is 100.0.
	Sampling float64 `json:"sampling,omitempty"`
	// The timeout for the gRPC request. Default is 500ms
	Timeout int32 `json:"timeout,omitempty"`
	// The tracer implementation to be used by Envoy.
	//
	// Types that are assignable to Tracer:
	Zipkin        *Zipkin        `json:"zipkin,omitempty"`
	Skywalking    *Skywalking    `json:"skywalking,omitempty"`
	OpenTelemetry *OpenTelemetry `json:"opentelemetry,omitempty"`
}

// Zipkin defines configuration for a Zipkin tracer.
type Zipkin struct {
	// Address of the Zipkin service (e.g. _zipkin:9411_).
	Service string `json:"service,omitempty"`
	Port    string `json:"port,omitempty"`
}

// Skywalking Defines configuration for a Skywalking tracer.
type Skywalking struct {
	// Address of the Skywalking tracer.
	Service string `json:"service,omitempty"`
	Port    string `json:"port,omitempty"`
	// The access token
	AccessToken string `json:"access_token,omitempty"`
}

// OpenTelemetry Defines configuration for a OpenTelemetry tracer.
type OpenTelemetry struct {
	// Address of OpenTelemetry tracer.
	Service string `json:"service,omitempty"`
	Port    string `json:"port,omitempty"`
}
type Gzip struct {
	// Flag to control gzip
	Enable              bool     `json:"enable,omitempty"`
	MinContentLength    int32    `json:"minContentLength,omitempty"`
	ContentType         []string `json:"contentType,omitempty"`
	DisableOnEtagHeader bool     `json:"disableOnEtagHeader,omitempty"`
	// Value from 1 to 9 that controls the amount of internal memory used by zlib.
	// Higher values use more memory, but are faster and produce better compression results. The default value is 5.
	MemoryLevel int32 `json:"memoryLevel,omitempty"`
	//  Value from 9 to 15 that represents the base two logarithmic of the compressor’s window size.
	//  Larger window results in better compression at the expense of memory usage.
	//  The default is 12 which will produce a 4096 bytes window
	WindowBits int32 `json:"windowBits,omitempty"`
	// Value for Zlib’s next output buffer. If not set, defaults to 4096.
	ChunkSize int32 `json:"chunkSize,omitempty"`
	// A value used for selecting the zlib compression level.
	// From COMPRESSION_LEVEL_1 to COMPRESSION_LEVEL_9
	// BEST_COMPRESSION == COMPRESSION_LEVEL_9 , BEST_SPEED == COMPRESSION_LEVEL_1
	CompressionLevel string `json:"compressionLevel,omitempty"`
	// A value used for selecting the zlib compression strategy which is directly related to the characteristics of the content.
	// Most of the time “DEFAULT_STRATEGY”
	// Value is one of DEFAULT_STRATEGY, FILTERED, HUFFMAN_ONLY, RLE, FIXED
	CompressionStrategy string `json:"compressionStrategy,omitempty"`
}
