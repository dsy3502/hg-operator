package higressconsole

import (
	"reflect"

	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	operatorv1alpha1 "github.com/alibaba/higress/higress-operator/api/v1alpha1"
)

const (
	role        = "higress-console"
	clusterRole = "higress-console"
)

func defaultRules() []rbacv1.PolicyRule {
	rules := []rbacv1.PolicyRule{
		// ingress controller
		{
			Verbs:     []string{"get", "list", "watch", "create", "update", "delete"},
			APIGroups: []string{""},
			Resources: []string{"secrets", "services"},
		},
		{
			Verbs:     []string{"get", "list", "watch", "create", "update", "delete"},
			APIGroups: []string{""},
			Resources: []string{"configmaps"},
		},
	}
	return rules
}

func initClusterRole(cr *rbacv1.ClusterRole, instance *operatorv1alpha1.HigressController) *rbacv1.ClusterRole {
	if cr == nil {
		return nil
	}

	*cr = rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance.Spec.Console.Name + "-" + instance.Namespace,
			Namespace: instance.Namespace,
		},
		Rules: []rbacv1.PolicyRule{
			// ingress controller
			{
				Verbs:     []string{"*"},
				APIGroups: []string{"networking.k8s.io", "extensions"},
				Resources: []string{"ingresses", "ingressclasses"},
			},
			{
				Verbs:     []string{"*"},
				APIGroups: []string{"networking.k8s.io", "extensions"},
				Resources: []string{"ingresses/status"},
			},
			// Needed for multi-cluster secret reading, possibly ingress certs in the future
			{
				Verbs:     []string{"get", "list", "watch", "create", "update", "delete"},
				APIGroups: []string{""},
				Resources: []string{"services"},
			},
			// required for CA's namespace controller
			{
				Verbs:     []string{"get", "list", "watch", "update", "create"},
				APIGroups: []string{""},
				Resources: []string{"configmaps"},
			},
			// Use for Kubernetes Service APIs
			{
				APIGroups: []string{"networking.x-k8s.io", "gateway.networking.k8s.io"},
				Resources: []string{"*"},
				Verbs:     []string{"get", "watch", "list", "update"},
			},
			{
				Verbs:     []string{"get", "create", "watch", "list", "update", "patch"},
				APIGroups: []string{"extensions.higress.io"},
				Resources: []string{"wasmplugins"},
			},
			{
				Verbs:     []string{"get", "create", "watch", "list", "update", "patch"},
				APIGroups: []string{"networking.higress.io"},
				Resources: []string{"http2rpcs", "mcpbridges"},
			},
		},
	}
	return cr
}

func muteClusterRole(cr *rbacv1.ClusterRole, instance *operatorv1alpha1.HigressController) controllerutil.MutateFn {
	return func() error {
		cr.Rules = defaultRules()
		return nil
	}
}

func initClusterRoleBinding(crb *rbacv1.ClusterRoleBinding, instance *operatorv1alpha1.HigressController) *rbacv1.ClusterRoleBinding {
	if crb == nil {
		return nil
	}
	*crb = rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance.Namespace + "-" + instance.Spec.Console.Name,
			Namespace: instance.Namespace,
		},
	}

	updateClusterRoleBinding(crb, instance)
	return crb
}

func updateClusterRoleBinding(crb *rbacv1.ClusterRoleBinding, instance *operatorv1alpha1.HigressController) {
	crb.RoleRef = rbacv1.RoleRef{
		Kind:     "ClusterRole",
		Name:     instance.Spec.Console.Name + "-" + instance.Namespace,
		APIGroup: "rbac.authorization.k8s.io",
	}

	subject := rbacv1.Subject{
		Kind:      "ServiceAccount",
		Name:      instance.Spec.Console.Name,
		Namespace: instance.Namespace,
	}

	for _, sub := range crb.Subjects {
		if reflect.DeepEqual(sub, subject) {
			return
		}
	}

	crb.Subjects = append(crb.Subjects, subject)
}

func muteClusterRoleBinding(crb *rbacv1.ClusterRoleBinding, instance *operatorv1alpha1.HigressController) controllerutil.MutateFn {
	return func() error {
		updateClusterRoleBinding(crb, instance)
		return nil
	}
}

func initRoleBinding(rb *rbacv1.RoleBinding, instance *operatorv1alpha1.HigressController) *rbacv1.RoleBinding {
	*rb = rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance.Spec.Console.Name,
			Namespace: instance.Namespace,
		},
	}

	updateRoleBinding(rb, instance)
	return rb
}

func updateRoleBinding(rb *rbacv1.RoleBinding, instance *operatorv1alpha1.HigressController) {
	rb.RoleRef = rbacv1.RoleRef{
		Kind:     "Role",
		Name:     instance.Spec.Console.Name,
		APIGroup: "rbac.authorization.k8s.io",
	}

	subject := rbacv1.Subject{
		Kind:      "ServiceAccount",
		Name:      instance.Spec.Console.Name + "-" + instance.Namespace,
		Namespace: instance.Namespace,
	}

	for _, sub := range rb.Subjects {
		if reflect.DeepEqual(sub, subject) {
			return
		}
	}

	rb.Subjects = append(rb.Subjects, subject)
}

func muteRoleBinding(rb *rbacv1.RoleBinding, instance *operatorv1alpha1.HigressController) controllerutil.MutateFn {
	return func() error {
		updateRoleBinding(rb, instance)
		return nil
	}
}

func initRole(r *rbacv1.Role, instance *operatorv1alpha1.HigressController) *rbacv1.Role {
	*r = rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance.Spec.Console.Name,
			Namespace: instance.Namespace,
		},
		Rules: defaultRules(),
	}

	return r
}

func muteRole(role *rbacv1.Role, instance *operatorv1alpha1.HigressController) controllerutil.MutateFn {
	return func() error {
		role.Rules = defaultRules()
		return nil
	}
}
