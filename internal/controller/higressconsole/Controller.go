package higressconsole

import (
	"context"
	"fmt"
	operatorv1alpha1 "github.com/alibaba/higress/higress-operator/api/v1alpha1"
	. "github.com/alibaba/higress/higress-operator/internal/controller"
	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type HigressConsole struct {
	client.Client
	Scheme *runtime.Scheme
}

func (r *HigressConsole) Reconcile(ctx context.Context, instance *operatorv1alpha1.HigressController) error {
	logger := log.FromContext(ctx)
	//todo 创建role
	if err := r.CreateRBAC(ctx, instance, logger); err != nil {
		logger.Error(err, "console Failed to create crds")
		return err
	}
	//创建servicecount
	if err := r.createServiceAccount(ctx, instance, logger); err != nil {
		logger.Error(err, "console Failed to create serviceAccount")
		return err
	}
	//创建或更新configmap
	if err := r.createConfigMap(ctx, instance, logger); err != nil {
		return err
	}
	//创建deployment
	if err := r.createDeployment(ctx, instance, logger); err != nil {
		logger.Error(err, "Failed to create deployment")
		return err
	}
	//创建service
	if err := r.createService(ctx, instance, logger); err != nil {
		logger.Error(err, "Failed to create service")
		return err
	}
	return nil
}

func (r *HigressConsole) createConfigMap(ctx context.Context, instance *operatorv1alpha1.HigressController, logger logr.Logger) error {
	gatewayConfigMap, err := initConsoleConfigMap(&apiv1.ConfigMap{}, instance)
	if err != nil {
		return err
	}
	if err = CreateOrUpdate(ctx, r.Client, "consoleConfigMap", gatewayConfigMap,
		muteConfigMap(gatewayConfigMap, instance, updateConsoleConfigMapSpec), logger); err != nil {
		return err
	}
	return nil
}

func (r *HigressConsole) createService(ctx context.Context, instance *operatorv1alpha1.HigressController, logger logr.Logger) error {
	svc := initService(&apiv1.Service{}, instance)
	if err := ctrl.SetControllerReference(instance, svc, r.Scheme); err != nil {
		return err
	}

	return CreateOrUpdate(ctx, r.Client, "Service", svc, muteService(svc, instance), logger)
}

func (r *HigressConsole) createDeployment(ctx context.Context, instance *operatorv1alpha1.HigressController, logger logr.Logger) error {
	deploy := initDeployment(&appsv1.Deployment{}, instance)
	if err := ctrl.SetControllerReference(instance, deploy, r.Scheme); err != nil {
		return err
	}

	return CreateOrUpdate(ctx, r.Client, "Deployment", deploy, muteDeployment(deploy, instance), logger)
}
func (r *HigressConsole) createServiceAccount(ctx context.Context, instance *operatorv1alpha1.HigressController, logger logr.Logger) error {
	if !instance.Spec.ServiceAccount.Enable {
		return nil
	}

	var (
		sa  = &apiv1.ServiceAccount{}
		err error
	)

	sa = initServiceAccount(sa, instance)
	//if err = ctrl.SetControllerReference(instance, sa, r.Scheme); err != nil {
	//	return err
	//}
	exist, err := CreateIfNotExits(ctx, r.Client, sa)
	if err != nil {
		logger.Error(err, fmt.Sprintf("Failed to create serviceAccount for HigressController(%v)", instance.Name))
		return err
	}

	if !exist {
		logger.Info(fmt.Sprintf("Create servieAccount for HigressController(%v)", instance.Name))
	}

	return nil
}

func (r HigressConsole) CreateRBAC(ctx context.Context, instance *operatorv1alpha1.HigressController, log logr.Logger) error {
	var (
		role = &rbacv1.Role{}
		rb   = &rbacv1.RoleBinding{}
		cr   = &rbacv1.ClusterRole{}
		crb  = &rbacv1.ClusterRoleBinding{}
	)

	initClusterRole(cr, instance)
	if err := CreateOrUpdate(ctx, r.Client, "ClusterRole", cr, muteClusterRole(cr, instance), log); err != nil {
		return err
	}

	initClusterRoleBinding(crb, instance)
	if err := CreateOrUpdate(ctx, r.Client, "ClusterRoleBinding", crb, muteClusterRoleBinding(crb, instance), log); err != nil {
		return err
	}

	initRole(role, instance)
	if err := CreateOrUpdate(ctx, r.Client, "role", role, muteRole(role, instance), log); err != nil {
		return err
	}

	initRoleBinding(rb, instance)
	if err := CreateOrUpdate(ctx, r.Client, "roleBinding", rb, muteRoleBinding(rb, instance), log); err != nil {
		return err
	}

	return nil
}
