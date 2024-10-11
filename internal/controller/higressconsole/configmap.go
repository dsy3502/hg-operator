package higressconsole

import (
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	operatorv1alpha1 "github.com/alibaba/higress/higress-operator/api/v1alpha1"
)

func initConsoleConfigMap(cm *apiv1.ConfigMap, instance *operatorv1alpha1.HigressController) (*apiv1.ConfigMap, error) {
	*cm = apiv1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance.Spec.Console.Name,
			Namespace: instance.Namespace,
			Labels:    instance.Labels,
		},
	}
	cm.Data = make(map[string]string)
	if _, err := updateConsoleConfigMapSpec(cm, instance); err != nil {
		return nil, err
	}

	return cm, nil
}

func updateConsoleConfigMapSpec(cm *apiv1.ConfigMap, instance *operatorv1alpha1.HigressController) (*apiv1.ConfigMap, error) {
	var (
		err error
	)
	if err != nil {
		return nil, err
	}
	for k, v := range instance.Spec.Console.ConfigMapSpec {
		cm.Data[k] = v
	}
	cm.Data["ingressClass"] = instance.Spec.IngressClass
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
