package higressconsole

import (
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	operatorv1alpha1 "github.com/alibaba/higress/higress-operator/api/v1alpha1"
)

func initServiceAccount(sa *apiv1.ServiceAccount, instance *operatorv1alpha1.HigressController) *apiv1.ServiceAccount {
	*sa = apiv1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   instance.Namespace,
			Name:        instance.Spec.Console.Name,
			Labels:      instance.Labels,
			Annotations: instance.Spec.ServiceAccount.Annotations,
		},
	}

	return sa
}
