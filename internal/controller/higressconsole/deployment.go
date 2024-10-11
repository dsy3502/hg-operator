package higressconsole

import (
	"fmt"
	operatorv1alpha1 "github.com/alibaba/higress/higress-operator/api/v1alpha1"
	"github.com/alibaba/higress/higress-operator/internal/controller"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	HigressCoreName = "higress-core"
	DiscoveryName   = "discovery"
)

func initDeployment(deploy *appsv1.Deployment, instance *operatorv1alpha1.HigressController) *appsv1.Deployment {

	*deploy = appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance.Spec.Console.Name,
			Namespace: instance.Namespace,
			Labels:    instance.Labels,
		},
	}
	updateDeploymentSpec(deploy, instance)
	return deploy
}

func updateDeploymentSpec(deploy *appsv1.Deployment, instance *operatorv1alpha1.HigressController) {
	deploy.Spec.Selector = &metav1.LabelSelector{MatchLabels: instance.Spec.SelectorLabels}

	deploy.Spec.Replicas = instance.Spec.Replicas

	controller.UpdateObjectMeta(&deploy.Spec.Template.ObjectMeta, instance, instance.Spec.SelectorLabels)
	deploy.Spec.Template.Name = instance.Spec.Console.Name
	deploy.Spec.Template.Spec.ServiceAccountName = instance.Spec.Console.Name
	exist := false
	for _, c := range deploy.Spec.Template.Spec.Containers {
		if c.Name == "higressconsole" {
			c.Image = genImage(instance.Spec.Console.Image.Repository, instance.Spec.Console.Image.Tag)
			c.ImagePullPolicy = instance.Spec.Controller.Image.ImagePullPolicy
			c.Env = genConsoleEnv(instance)
			c.Ports = genConsolePorts(instance)
			exist = true
			break
		}
	}
	if !exist {
		deploy.Spec.Template.Spec.Containers = append(deploy.Spec.Template.Spec.Containers, apiv1.Container{
			Name:            "higressconsole",
			Image:           genImage(instance.Spec.Console.Image.Repository, instance.Spec.Console.Image.Tag),
			ImagePullPolicy: instance.Spec.Controller.Image.ImagePullPolicy,
			//Args:            genControllerArgs(instance),
			Ports:           genConsolePorts(instance),
			SecurityContext: genConsoleSecurityContext(instance),
			Env:             genConsoleEnv(instance),
			VolumeMounts:    genConsoleVolumeMounts(instance),
		})
	}
	deploy.Spec.Template.Spec.Volumes = genVolumes(instance)
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

func genConsoleSecurityContext(instance *operatorv1alpha1.HigressController) *apiv1.SecurityContext {
	if instance.Spec.Console.SecurityContext != nil {
		return instance.Spec.Console.SecurityContext
	}
	return &apiv1.SecurityContext{}
}

func genConsoleEnv(instance *operatorv1alpha1.HigressController) []apiv1.EnvVar {
	envs := []apiv1.EnvVar{}

	for k, v := range instance.Spec.Controller.Env {
		envs = append(envs, apiv1.EnvVar{Name: k, Value: v})
	}

	return envs
}

func genConsolePorts(instance *operatorv1alpha1.HigressController) []apiv1.ContainerPort {
	if len(instance.Spec.Controller.Ports) != 0 {
		return instance.Spec.Console.Ports
	}

	return []apiv1.ContainerPort{
		{
			ContainerPort: 8080,
			Protocol:      apiv1.ProtocolTCP,
			Name:          "http",
		},
	}
}

func genConsoleVolumeMounts(instance *operatorv1alpha1.HigressController) []apiv1.VolumeMount {
	return []apiv1.VolumeMount{
		{
			Name:      "access-token",
			MountPath: "/var/run/secrets/access-token",
		},
	}
}

func genVolumes(instance *operatorv1alpha1.HigressController) []apiv1.Volume {

	defaultMode := int32(420)
	expireTime := int64(3600)
	volumes := []apiv1.Volume{
		{
			Name: "access-token",
			VolumeSource: apiv1.VolumeSource{
				Projected: &apiv1.ProjectedVolumeSource{
					Sources: []apiv1.VolumeProjection{
						{ServiceAccountToken: &apiv1.ServiceAccountTokenProjection{
							Audience:          "istio-ca",
							ExpirationSeconds: &expireTime,
							Path:              "token",
						}},
					},
					DefaultMode: &defaultMode,
				},
			},
		},
	}
	return volumes
}
