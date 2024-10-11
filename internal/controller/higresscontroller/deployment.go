package higresscontroller

import (
	"fmt"
	"github.com/alibaba/higress/higress-operator/internal/controller"
	"strconv"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	operatorv1alpha1 "github.com/alibaba/higress/higress-operator/api/v1alpha1"
)

const (
	HigressCoreName = "higress-core"
	DiscoveryName   = "discovery"
)

func initDeployment(deploy *appsv1.Deployment, instance *operatorv1alpha1.HigressController) *appsv1.Deployment {
	newdeploy := deploy.DeepCopyObject().(*appsv1.Deployment)
	if newdeploy.Name == "" {
		newdeploy = &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      instance.Name,
				Namespace: instance.Namespace,
				Labels:    instance.Labels,
			},
		}
	}

	updateDeploymentSpec(newdeploy, instance)
	return newdeploy
}

func int32Ptr(i int32) *int32 { return &i }

func updateDeploymentSpec(deploy *appsv1.Deployment, instance *operatorv1alpha1.HigressController) {
	deploy.Spec.Selector = &metav1.LabelSelector{MatchLabels: instance.Spec.SelectorLabels}
	if instance.Spec.Replicas == nil {
		deploy.Spec.Replicas = int32Ptr(1)
	} else {
		deploy.Spec.Replicas = instance.Spec.Replicas
	}
	deploy.Spec.ProgressDeadlineSeconds = int32Ptr(600)
	deploy.Spec.RevisionHistoryLimit = int32Ptr(10)
	controller.UpdateObjectMeta(&deploy.Spec.Template.ObjectMeta, instance, instance.Spec.SelectorLabels)

	deploy.Spec.Template.Spec.ServiceAccountName = getServiceAccount(instance)
	deploy.Spec.Template.Spec.Volumes = genVolumes(instance)
	controllerexist := false
	pilotexist := false
	containers := []apiv1.Container{}
	for _, c := range deploy.Spec.Template.Spec.Containers {
		if c.Name == genControllerName(instance) {
			controllerexist = true
			container := c.DeepCopy()
			container.Image = genImage(instance.Spec.Controller.Image.Repository, instance.Spec.Controller.Image.Tag)
			container.Image = genImage(instance.Spec.Controller.Image.Repository, instance.Spec.Controller.Image.Tag)
			container.Env = genControllerEnv(instance, c.Env)
			container.Ports = genControllerPorts(instance)
			container.Args = genControllerArgs(instance)
			container.Resources = *instance.Spec.Controller.Resources
			container.VolumeMounts = genControllerVolumeMounts(instance)
			container.SecurityContext = genControllerSecurityContext(instance)
			containers = append(containers, *container)
		} else if c.Name == genPilotName(instance) {
			pilotexist = true
			container := c.DeepCopy()
			container.Image = genImage(instance.Spec.Pilot.Image.Repository, instance.Spec.Pilot.Image.Tag)
			container.Env = genPilotEnv(instance, c.Env)
			container.Ports = genPilotPorts(instance)
			container.Resources = *instance.Spec.Controller.Resources
			container.Args = genPilotArgs(instance)
			container.VolumeMounts = genPilotVolumeMounts(instance)
			container.SecurityContext = genPilotSecurityContext(instance)
			containers = append(containers, *container)
		} else {
			containers = append(containers, c)
		}
	}
	if !controllerexist {
		containers = append(containers, apiv1.Container{
			Name:            genControllerName(instance),
			Image:           genImage(instance.Spec.Controller.Image.Repository, instance.Spec.Controller.Image.Tag),
			ImagePullPolicy: instance.Spec.Controller.Image.ImagePullPolicy,
			Args:            genControllerArgs(instance),
			Ports:           genControllerPorts(instance),
			Resources:       *instance.Spec.Controller.Resources,
			SecurityContext: genControllerSecurityContext(instance),
			Env:             genControllerEnv(instance, []apiv1.EnvVar{}),
			VolumeMounts:    genControllerVolumeMounts(instance),
		})
	}
	if !pilotexist {
		containers = append(containers, apiv1.Container{
			Name:            genPilotName(instance),
			Image:           genImage(instance.Spec.Pilot.Image.Repository, instance.Spec.Pilot.Image.Tag),
			ImagePullPolicy: instance.Spec.Controller.Image.ImagePullPolicy,
			Args:            genPilotArgs(instance),
			Resources:       *instance.Spec.Pilot.Resources,
			Ports:           genPilotPorts(instance),
			SecurityContext: genPilotSecurityContext(instance),
			Env:             genPilotEnv(instance, []apiv1.EnvVar{}),
			VolumeMounts:    genPilotVolumeMounts(instance),
		})
	}

	deploy.Spec.Template.Spec.Containers = containers
}

func muteDeployment(deploy *appsv1.Deployment, instance *operatorv1alpha1.HigressController) controllerutil.MutateFn {
	return func() error {
		updateDeploymentSpec(deploy, instance)
		return nil
	}
}

func genImage(repository string, tag string) string {
	return fmt.Sprintf("%v:%v", repository, tag)
}

func genPilotName(instance *operatorv1alpha1.HigressController) string {
	if instance.Spec.Pilot.Name != "" {
		return instance.Spec.Pilot.Name
	}
	return DiscoveryName
}

func genPilotProbe(instance *operatorv1alpha1.HigressController) *apiv1.Probe {
	pilot := instance.Spec.Pilot

	if pilot.ReadinessProbe != nil {
		return pilot.ReadinessProbe
	}

	return &apiv1.Probe{
		TimeoutSeconds:      5,
		PeriodSeconds:       3,
		InitialDelaySeconds: 1,
		ProbeHandler: apiv1.ProbeHandler{
			HTTPGet: &apiv1.HTTPGetAction{
				Path: "/ready",
				Port: intstr.FromInt(8080),
			},
		},
	}
}

func genPilotEnv(instance *operatorv1alpha1.HigressController, oldEnvs []apiv1.EnvVar) []apiv1.EnvVar {
	pilot := instance.Spec.Pilot
	envMap := map[string]string{}
	for _, e := range oldEnvs {
		envMap[e.Name] = e.Value
	}
	envs := []apiv1.EnvVar{
		{
			Name:  "HIGRESS_CONTROLLER_SVC",
			Value: "127.0.0.1",
		},
		{
			Name:  "HIGRESS_CONTROLLER_PORT",
			Value: "15051",
		},
		{
			Name:  "REVISION",
			Value: "default",
		},
		{
			Name:  "JWT_POLICY",
			Value: instance.Spec.JwtPolicy,
		},
		{
			Name:  "PILOT_CERT_PROVIDER",
			Value: "istiod",
		},
		{
			Name: "POD_NAMESPACE",
			ValueFrom: &apiv1.EnvVarSource{
				FieldRef: &apiv1.ObjectFieldSelector{
					FieldPath: "metadata.namespace",
				},
			},
		},
		{
			Name: "POD_NAME",
			ValueFrom: &apiv1.EnvVarSource{
				FieldRef: &apiv1.ObjectFieldSelector{
					FieldPath: "metadata.name",
				},
			},
		},
		{
			Name: "SERVICE_ACCOUNT",
			ValueFrom: &apiv1.EnvVarSource{
				FieldRef: &apiv1.ObjectFieldSelector{
					FieldPath: "spec.serviceAccountName",
				},
			},
		},
		{
			Name:  "KUBECONFIG",
			Value: "/var/run/secrets/remote/config",
		},
		{
			Name:  "PRIORITIZED_LEADER_ELECTION",
			Value: "false",
		},
		{
			Name:  "INJECT_ENABLE",
			Value: "false",
		},
		{
			Name:  "PILOT_ENABLE_PROTOCOL_SNIFFING_FOR_OUTBOUND",
			Value: strconv.FormatBool(pilot.EnableProtocolSniffingForOutbound),
		},
		{
			Name:  "PILOT_ENABLE_PROTOCOL_SNIFFING_FOR_INBOUND",
			Value: strconv.FormatBool(pilot.EnableProtocolSniffingForInbound),
		},
	}

	if pilot.TraceSampling != "" {
		envs = append(envs, apiv1.EnvVar{Name: "PILOT_TRACE_SAMPLING", Value: pilot.TraceSampling})
	}

	istioAddr := fmt.Sprintf("istiod.%s.svc:15012", instance.Namespace)
	if instance.Spec.Revision != "" {
		istioAddr = fmt.Sprintf("istiod-%s.%s.svc:15012", instance.Spec.Revision, instance.Namespace)
	}
	envs = append(envs, apiv1.EnvVar{Name: "ISTIOD_ADDR", Value: istioAddr})

	if istiod := instance.Spec.Istiod; istiod != nil {
		envs = append(envs, apiv1.EnvVar{
			Name:  "PILOT_ENABLE_ANALYSIS",
			Value: strconv.FormatBool(instance.Spec.Istiod.EnableAnalysis),
		})
	}

	clusterId := "Kubernetes"
	if multiCluster := instance.Spec.MultiCluster; multiCluster != nil && multiCluster.Enable {
		clusterId = instance.Spec.MultiCluster.ClusterName
	}

	envs = append(envs, apiv1.EnvVar{Name: "CLUSTER_ID", Value: clusterId})

	envs = append(envs, apiv1.EnvVar{Name: "HIGRESS_ENABLE_ISTIO_API", Value: strconv.FormatBool(instance.Spec.EnableIstioAPI)})

	if !instance.Spec.EnableHigressIstio {
		envs = append(envs, apiv1.EnvVar{Name: "CUSTOM_CA_CERT_NAME", Value: "higress-ca-root-cert"})
	}

	for k, v := range instance.Spec.Pilot.Env {
		envs = append(envs, apiv1.EnvVar{Name: k, Value: v})
	}

	for _, e := range envs {
		if envMap[e.Name] != e.Value {
			return envs
		}
	}
	return oldEnvs
}

func genPilotSecurityContext(instance *operatorv1alpha1.HigressController) *apiv1.SecurityContext {
	pilot := instance.Spec.Pilot

	if pilot.SecurityContext != nil {
		return pilot.SecurityContext
	}

	readOnlyRootFilesystem := true
	runAsGroup := int64(1337)
	runAsUser := int64(1337)
	runAsNoRoot := true
	return &apiv1.SecurityContext{
		ReadOnlyRootFilesystem: &readOnlyRootFilesystem,
		RunAsGroup:             &runAsGroup,
		RunAsUser:              &runAsUser,
		RunAsNonRoot:           &runAsNoRoot,
		Capabilities: &apiv1.Capabilities{
			Drop: []apiv1.Capability{"ALL"},
		},
	}
}

func genPilotPorts(instance *operatorv1alpha1.HigressController) []apiv1.ContainerPort {
	pilot := instance.Spec.Pilot

	if len(pilot.Ports) != 0 {
		return pilot.Ports
	}

	return []apiv1.ContainerPort{
		{
			ContainerPort: 8080,
			Protocol:      apiv1.ProtocolTCP,
		},
		{
			ContainerPort: 15017,
			Protocol:      apiv1.ProtocolTCP,
		},
		{
			ContainerPort: 15010,
			Protocol:      apiv1.ProtocolTCP,
		},
	}
}

func genPilotArgs(instance *operatorv1alpha1.HigressController) []string {
	pilot := instance.Spec.Pilot

	var args []string
	args = append(args, "discovery")
	args = append(args, fmt.Sprintf("--monitoringAddr=:15014"))
	args = append(args, fmt.Sprintf("--domain=%v", pilot.ClusterDomain))
	args = append(args, fmt.Sprintf("--keepaliveMaxServerConnectionAge=%v", pilot.KeepaliveMaxServerConnectionAge))

	if pilot.LogLevel != "" {
		args = append(args, fmt.Sprintf("--log_output_level=%v", pilot.LogLevel))
	}
	if pilot.LogAsJson {
		args = append(args, fmt.Sprintf("--log_as_json"))
	}
	if pilot.OneNamespace {
		args = append(args, fmt.Sprintf("-a=%v", instance.Namespace))
	}
	if len(pilot.Plugins) > 0 {
		args = append(args, fmt.Sprintf("--plugins=%v", strings.Join(pilot.Plugins, ",")))
	}

	return args
}

func genPilotVolumeMounts(instance *operatorv1alpha1.HigressController) []apiv1.VolumeMount {
	vms := []apiv1.VolumeMount{
		{
			Name:      "config",
			MountPath: "/etc/istio/config",
		},
		{
			Name:      "local-certs",
			MountPath: "/var/run/secrets/istio-dns",
		},
		{
			Name:      "cacerts",
			MountPath: "/etc/cacerts",
			ReadOnly:  true,
		},
		{
			Name:      "istio-kubeconfig",
			MountPath: "/var/run/secrets/remote",
			ReadOnly:  true,
		},
	}
	pilot := instance.Spec.Pilot
	if instance.Spec.JwtPolicy == "third-party-jwt" {
		vms = append(vms, apiv1.VolumeMount{
			Name:      "istio-token",
			MountPath: "/var/run/secrets/tokens",
			ReadOnly:  true,
		})
	}
	if pilot.JwksResolveExtraRootCA != "" {
		vms = append(vms, apiv1.VolumeMount{
			Name:      "extracacerts",
			MountPath: "/cacerts",
		})
	}

	return vms
}

func genControllerName(instance *operatorv1alpha1.HigressController) string {
	if instance.Spec.Controller.Name != "" {
		return instance.Spec.Controller.Name
	}
	return HigressCoreName
}

func genControllerArgs(instance *operatorv1alpha1.HigressController) []string {
	var args []string

	args = append(args, "serve")
	args = append(args, fmt.Sprintf("--gatewaySelectorKey=higress"))
	args = append(args, fmt.Sprintf("--gatewaySelectorValue=%v-%v", instance.Namespace, instance.Spec.Controller.GatewayName))
	args = append(args, fmt.Sprintf("--ingressClass=%v", instance.Spec.IngressClass))
	//args = append(args, fmt.Sprintf("--ingressClass=%v", "icks.io/"+instance.Spec.IngressClass))
	if instance.Spec.Controller.LogLevel != "" {
		args = append(args, fmt.Sprintf("--log_output_level=%v", instance.Spec.Controller.LogLevel))
	}
	if !instance.Spec.EnableStatus {
		args = append(args, fmt.Sprintf("--enableStatus=%v", instance.Spec.EnableStatus))
	}
	if instance.Spec.Controller.WatchNamespace != "" {
		args = append(args, fmt.Sprintf("--watchNamespace=%v", instance.Spec.Controller.WatchNamespace))
	}

	return args
}

func genControllerSecurityContext(instance *operatorv1alpha1.HigressController) *apiv1.SecurityContext {
	if instance.Spec.Controller.SecurityContext != nil {
		return instance.Spec.Controller.SecurityContext
	}
	return &apiv1.SecurityContext{}
}

func genControllerEnv(instance *operatorv1alpha1.HigressController, oldEnvs []apiv1.EnvVar) []apiv1.EnvVar {

	envMap := map[string]string{}
	for _, e := range oldEnvs {
		envMap[e.Name] = e.Value
	}
	envs := []apiv1.EnvVar{
		{
			Name: "POD_NAMESPACE",
			ValueFrom: &apiv1.EnvVarSource{
				FieldRef: &apiv1.ObjectFieldSelector{
					FieldPath: "metadata.namespace",
				},
			},
		},
		{
			Name: "POD_NAME",
			ValueFrom: &apiv1.EnvVarSource{
				FieldRef: &apiv1.ObjectFieldSelector{
					FieldPath: "metadata.name",
				},
			},
		},
	}

	for k, v := range instance.Spec.Controller.Env {
		envs = append(envs, apiv1.EnvVar{Name: k, Value: v})
	}

	for _, e := range envs {
		if envMap[e.Name] != e.Value {
			return envs
		}
		if e.Name == "POD_NAMESPACE" || e.Name == "POD_NAME" {
			if _, ok := envMap[e.Name]; !ok {
				return envs
			}
		}
	}
	return oldEnvs
}

func genControllerPorts(instance *operatorv1alpha1.HigressController) []apiv1.ContainerPort {
	if len(instance.Spec.Controller.Ports) != 0 {
		return instance.Spec.Controller.Ports
	}

	return []apiv1.ContainerPort{
		{
			ContainerPort: 8888,
			Protocol:      apiv1.ProtocolTCP,
			Name:          "http",
		},
		{
			ContainerPort: 15051,
			Protocol:      apiv1.ProtocolTCP,
			Name:          "grpc",
		},
	}
}

func genControllerVolumeMounts(instance *operatorv1alpha1.HigressController) []apiv1.VolumeMount {
	return []apiv1.VolumeMount{
		{
			Name:      "log",
			MountPath: "/var/log",
		},
	}
}

func genVolumes(instance *operatorv1alpha1.HigressController) []apiv1.Volume {
	optional := true
	defaultMode := int32(420)
	volumes := []apiv1.Volume{
		{
			Name: "log",
			VolumeSource: apiv1.VolumeSource{
				EmptyDir: &apiv1.EmptyDirVolumeSource{},
			},
		},
		{
			Name: "local-certs",
			VolumeSource: apiv1.VolumeSource{
				EmptyDir: &apiv1.EmptyDirVolumeSource{
					Medium: apiv1.StorageMediumMemory,
				},
			},
		},
		{
			Name: "cacerts",
			VolumeSource: apiv1.VolumeSource{
				Secret: &apiv1.SecretVolumeSource{
					SecretName:  "cacerts",
					Optional:    &optional,
					DefaultMode: &defaultMode,
				},
			},
		},
		{
			Name: "istio-kubeconfig",
			VolumeSource: apiv1.VolumeSource{
				Secret: &apiv1.SecretVolumeSource{
					SecretName:  "istio-kubeconfig",
					Optional:    &optional,
					DefaultMode: &defaultMode,
				},
			},
		},
	}

	if !instance.Spec.EnableHigressIstio {
		volumes = append(volumes, apiv1.Volume{
			Name: "config",
			VolumeSource: apiv1.VolumeSource{
				ConfigMap: &apiv1.ConfigMapVolumeSource{
					LocalObjectReference: apiv1.LocalObjectReference{
						Name: "cm-" + instance.Name,
					},
					DefaultMode: int32Ptr(420),
				},
			},
		})
	}

	expirationSeconds := int64(43200)
	if instance.Spec.JwtPolicy == "third-party-jwt" {
		volumes = append(volumes, apiv1.Volume{
			Name: "istio-token",
			VolumeSource: apiv1.VolumeSource{
				Projected: &apiv1.ProjectedVolumeSource{
					Sources: []apiv1.VolumeProjection{
						{
							ServiceAccountToken: &apiv1.ServiceAccountTokenProjection{
								Audience:          instance.Spec.Controller.SDSTokenAud,
								ExpirationSeconds: &expirationSeconds,
								Path:              "istio-token",
							},
						},
					},
					DefaultMode: int32Ptr(420),
				},
			},
		})
	}

	if instance.Spec.Pilot.JwksResolveExtraRootCA != "" {
		volumes = append(volumes, apiv1.Volume{
			Name: "extracacerts",
			VolumeSource: apiv1.VolumeSource{
				ConfigMap: &apiv1.ConfigMapVolumeSource{
					LocalObjectReference: apiv1.LocalObjectReference{
						Name: "pilot-jwks-extra-cacerts" + instance.Spec.Revision,
					},
				},
			},
		})
	}

	return volumes
}
