package higresscontroller

import (
	operatorv1alpha1 "github.com/alibaba/higress/higress-operator/api/v1alpha1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func initIngressclass(ingressclass *networkingv1.IngressClass, instance *operatorv1alpha1.HigressController) (*networkingv1.IngressClass, error) {
	*ingressclass = networkingv1.IngressClass{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance.Spec.IngressClass,
			Namespace: instance.Namespace,
			Labels:    instance.Labels,
		},
	}

	if _, err := updateIngressclassSpec(ingressclass, instance); err != nil {
		return nil, err
	}

	return ingressclass, nil
}

func updateIngressclassSpec(ingressclass *networkingv1.IngressClass, instance *operatorv1alpha1.HigressController) (*networkingv1.IngressClass, error) {
	ingressclass.Spec = networkingv1.IngressClassSpec{
		Controller: "icks.io/" + instance.Spec.IngressClass,
	}
	return ingressclass, nil
}

func muteIngressclass(ingressclass *networkingv1.IngressClass, instance *operatorv1alpha1.HigressController) controllerutil.MutateFn {
	return func() error {
		updateIngressclassSpec(ingressclass, instance)
		return nil
	}
}
