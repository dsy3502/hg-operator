package higressconsole

import (
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	operatorv1alpha1 "github.com/alibaba/higress/higress-operator/api/v1alpha1"
)

const (
	HigressControllerServiceName = "higress-controller"
)

func initService(svc *apiv1.Service, instance *operatorv1alpha1.HigressController) *apiv1.Service {
	*svc = apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance.Spec.Console.Name,
			Namespace: instance.Namespace,
			Labels:    instance.Labels,
		},
	}
	updateServiceSpec(svc, instance)
	return svc
}

func updateServiceSpec(svc *apiv1.Service, instance *operatorv1alpha1.HigressController) {
	if svc.Spec.Type == "" {
		svc.Spec.Type = apiv1.ServiceTypeClusterIP
		svc.Spec.Selector = instance.Spec.Console.SelectorLabels
		svc.Spec.Ports = []apiv1.ServicePort{
			{
				Name:     "http",
				Protocol: apiv1.ProtocolTCP,
				Port:     8080,
			},
		}
	}
	ports := []apiv1.ServicePort{}
	if s := instance.Spec.Service; s != nil {
		if s.Type != "" {
			svc.Spec.Type = apiv1.ServiceType(s.Type)
		}
		ports = s.Ports
	}

	set := make(map[string]struct{})
	for _, port := range svc.Spec.Ports {
		set[port.Name] = struct{}{}
	}
	for _, port := range ports {
		if _, ok := set[port.Name]; !ok {
			svc.Spec.Ports = append(svc.Spec.Ports, port)
		}
	}

}

func muteService(svc *apiv1.Service, instance *operatorv1alpha1.HigressController) controllerutil.MutateFn {
	return func() error {
		updateServiceSpec(svc, instance)
		return nil
	}
}
