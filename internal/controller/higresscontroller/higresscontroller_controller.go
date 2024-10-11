package higresscontroller

import (
	"context"
	"fmt"
	operatorv1alpha1 "github.com/alibaba/higress/higress-operator/api/v1alpha1"
	. "github.com/alibaba/higress/higress-operator/internal/controller"
	"github.com/alibaba/higress/higress-operator/internal/controller/higressconsole"
	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apixv1client "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/typed/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	finalizer = "higresscontroller.higress.io/finalizer"
)

// HigressControllerReconciler reconciles a HigressController object
type HigressControllerReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Config *rest.Config
}

func (r *HigressControllerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&operatorv1alpha1.HigressController{}).
		Owns(&appsv1.Deployment{}).
		Owns(&apiv1.Service{}).
		Owns(&apiv1.ServiceAccount{}).
		Owns(&networkingv1.IngressClass{}).
		Complete(r)
}

func (r *HigressControllerReconciler) Reconcile(ctx context.Context, request ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	instance := &operatorv1alpha1.HigressController{}
	if err := r.Get(ctx, request.NamespacedName, instance); err != nil {
		if errors.IsNotFound(err) {
			logger.Info(fmt.Sprintf("HigressController(%v) resource not found. Ignoring since object must be deleted", request.NamespacedName))
			return ctrl.Result{}, nil
		}

		logger.Error(err, fmt.Sprintf("Failed to get resource HigressController(%v)", request.NamespacedName))
		return ctrl.Result{}, err
	}

	r.setDefaultValues(instance)

	// if DeletionTimestamp is not nil, it means is marked to be deleted
	if instance.GetDeletionTimestamp() != nil {
		if controllerutil.ContainsFinalizer(instance, finalizer) {
			if err := r.finalizeHigressController(instance, logger); err != nil {
				return ctrl.Result{}, err
			}

			controllerutil.RemoveFinalizer(instance, finalizer)

			if err := r.Update(ctx, instance); err != nil {
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	// check if the namespace still exists during the reconciling
	ns, nn := &apiv1.Namespace{}, types.NamespacedName{Name: instance.Namespace, Namespace: apiv1.NamespaceAll}
	err := r.Get(ctx, nn, ns)
	if (err != nil && errors.IsNotFound(err)) || (ns.Status.Phase == apiv1.NamespaceTerminating) {
		logger.Info(fmt.Sprintf("The namespace (%s) doesn't exist or is in Terminating status, canceling Reconciling", instance.Namespace))
		return ctrl.Result{}, nil
	} else if err != nil {
		logger.Error(err, "Failed to check if namespace exists")
		return ctrl.Result{}, nil
	}

	// add finalizer for this CR
	if controllerutil.AddFinalizer(instance, finalizer) {
		if err = r.Update(ctx, instance); err != nil {
			logger.Error(err, "Failed to add finalizer for higressController")
			return ctrl.Result{}, err
		}
	}

	if err = r.createCRDs(ctx, logger); err != nil {
		logger.Error(err, "Failed to create crds")
		return ctrl.Result{}, err
	}

	if err = r.createServiceAccount(ctx, instance, logger); err != nil {
		logger.Error(err, "Failed to create serviceAccount")
		return ctrl.Result{}, err
	}

	if err = r.createRBAC(ctx, instance, logger); err != nil {
		logger.Error(err, "Failed to create rbac")
		return ctrl.Result{}, err
	}
	if err = r.createConfigMap(ctx, instance, logger); err != nil {
		return ctrl.Result{}, err
	}
	if err = r.createDeployment(ctx, instance, logger); err != nil {
		logger.Error(err, "Failed to create deployment")
		return ctrl.Result{}, err
	}
	if err = r.createIngressClass(ctx, instance, logger); err != nil {
		logger.Error(err, "Failed to create ingressclass")
		return ctrl.Result{}, err
	}
	if err = r.createService(ctx, instance, logger); err != nil {
		logger.Error(err, "Failed to create service")
		return ctrl.Result{}, err
	}

	if !instance.Status.Deployed {
		instance.Status.Deployed = true
		if err = r.Status().Update(ctx, instance); err != nil {
			logger.Error(err, "Failed to update higressController/status")
			return ctrl.Result{}, err
		}
	}
	//todo 创建console
	if instance.Spec.Console.Name != "" {
		if err = r.addOrUpdateConsole(ctx, instance, logger); err != nil {

			logger.Error(err, "Failed to create console")
			return ctrl.Result{}, err
		}
	}
	return ctrl.Result{}, nil
}
func (r *HigressControllerReconciler) addOrUpdateConsole(ctx context.Context, instance *operatorv1alpha1.HigressController, logger logr.Logger) error {
	console := higressconsole.HigressConsole{
		Client: r.Client,
		Scheme: r.Scheme,
	}
	if err := console.Reconcile(ctx, instance); err != nil {
		return err
	}
	return nil
}

func (r *HigressControllerReconciler) createConfigMap(ctx context.Context, instance *operatorv1alpha1.HigressController, logger logr.Logger) error {
	gatewayConfigMap, err := initGatewayConfigMap(&apiv1.ConfigMap{}, instance)
	if err != nil {
		return err
	}
	if err = CreateOrUpdate(ctx, r.Client, "gatewayConfigMap", gatewayConfigMap,
		muteConfigMap(gatewayConfigMap, instance, updateGatewayConfigMapSpec), logger); err != nil {
		return err
	}
	return nil
}

func (r *HigressControllerReconciler) createServiceAccount(ctx context.Context, instance *operatorv1alpha1.HigressController, logger logr.Logger) error {
	if !instance.Spec.ServiceAccount.Enable {
		return nil
	}

	var (
		sa  = &apiv1.ServiceAccount{}
		err error
	)

	sa = initServiceAccount(sa, instance)
	if err = ctrl.SetControllerReference(instance, sa, r.Scheme); err != nil {
		return err
	}

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

func (r *HigressControllerReconciler) createRBAC(ctx context.Context, instance *operatorv1alpha1.HigressController, log logr.Logger) error {
	if instance.Spec.RBAC == nil {
		instance.Spec.RBAC = &operatorv1alpha1.RBAC{Enable: true}
	}
	if instance.Spec.ServiceAccount == nil {
		instance.Spec.ServiceAccount = &operatorv1alpha1.ServiceAccount{Enable: true}
	}
	if !instance.Spec.RBAC.Enable || !instance.Spec.ServiceAccount.Enable {
		return nil
	}

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

func (r *HigressControllerReconciler) createIngressClass(ctx context.Context, instance *operatorv1alpha1.HigressController, logger logr.Logger) error {
	ic := &networkingv1.IngressClass{}
	if err := r.Get(ctx, types.NamespacedName{Name: instance.Spec.IngressClass, Namespace: instance.Namespace}, ic); err != nil {
		if errors.IsNotFound(err) {
			ingressclass, err := initIngressclass(&networkingv1.IngressClass{}, instance)
			if err != nil {
				return err
			}
			if err = ctrl.SetControllerReference(instance, ingressclass, r.Scheme); err != nil {
				return err
			}

			return CreateOrUpdate(ctx, r.Client, "IngressClass", ingressclass, muteIngressclass(ingressclass, instance), logger)
		}
	}
	return nil
}

func (r *HigressControllerReconciler) createDeployment(ctx context.Context, instance *operatorv1alpha1.HigressController, logger logr.Logger) error {

	olddeploy := &appsv1.Deployment{}
	//deploy := initDeployment(&appsv1.Deployment{}, instance)
	err := r.Get(ctx, types.NamespacedName{Name: instance.Spec.Controller.Name, Namespace: instance.Namespace}, olddeploy)
	if errors.IsNotFound(err) {
		deploy := initDeployment(&appsv1.Deployment{}, instance)
		if err := ctrl.SetControllerReference(instance, deploy, r.Scheme); err != nil {
			return err
		}
		return CreateOrUpdate(ctx, r.Client, "Deployment", deploy, muteDeployment(deploy, instance), logger)
	}
	deploy := initDeployment(olddeploy, instance)
	if equality.Semantic.DeepEqual(deploy.Spec, olddeploy.Spec) {
		return nil
	}

	if err := ctrl.SetControllerReference(instance, deploy, r.Scheme); err != nil {
		return err
	}
	return CreateOrUpdate(ctx, r.Client, "Deployment", deploy, muteDeployment(deploy, instance), logger)
}

func (r *HigressControllerReconciler) createService(ctx context.Context, instance *operatorv1alpha1.HigressController, logger logr.Logger) error {
	svc := initService(&apiv1.Service{}, instance)
	if err := ctrl.SetControllerReference(instance, svc, r.Scheme); err != nil {
		return err
	}

	return CreateOrUpdate(ctx, r.Client, "Service", svc, muteService(svc, instance), logger)
}

func (r *HigressControllerReconciler) finalizeHigressController(instance *operatorv1alpha1.HigressController, logger logr.Logger) error {
	if !instance.Spec.RBAC.Enable || !instance.Spec.ServiceAccount.Enable {
		return nil
	}

	var (
		ctx = context.TODO()
		crb = &rbacv1.ClusterRoleBinding{}
	)

	name := getServiceAccount(instance)
	nn := types.NamespacedName{Name: instance.Namespace + "-" + name, Namespace: apiv1.NamespaceAll}
	if err := r.Get(ctx, nn, crb); err != nil {
		return err
	}

	var subjects []rbacv1.Subject
	for _, subject := range crb.Subjects {
		if subject.Name != name || subject.Namespace != instance.Namespace {
			subjects = append(subjects, subject)
		}
	}
	crb.Subjects = subjects
	return r.Update(ctx, crb)
}

func (r *HigressControllerReconciler) createCRDs(ctx context.Context, logger logr.Logger) error {
	apixClient, err := apixv1client.NewForConfig(r.Config)
	if err != nil {
		return err
	}

	crds, err := getCRDs()
	if err != nil {
		return err
	}

	cli := apixClient.CustomResourceDefinitions()
	for _, crd := range crds {
		if existing, err := cli.Get(ctx, crd.Name, metav1.GetOptions{TypeMeta: crd.TypeMeta}); err != nil {
			if !errors.IsNotFound(err) {
				logger.Error(err, fmt.Sprintf("failed to get CRD %v", crd.Name))
				return err
			}
			if _, err = cli.Create(ctx, crd, metav1.CreateOptions{TypeMeta: crd.TypeMeta}); err != nil {
				logger.Error(err, fmt.Sprintf("failed to create CRD %v", crd.Name))
				return err
			}
		} else if !equality.Semantic.DeepEqual(existing.Spec, crd.Spec) {
			// todo(lql): We should check if it has changed before updating it
			existing.Spec = crd.Spec
			if _, err = cli.Update(ctx, existing, metav1.UpdateOptions{TypeMeta: crd.TypeMeta}); err != nil {
				logger.Error(err, fmt.Sprintf("failed to update CRD %v", crd.Name))
				return err
			}
		}
	}
	return nil
}

func (r *HigressControllerReconciler) setDefaultValues(instance *operatorv1alpha1.HigressController) {
	if instance.Spec.RBAC == nil {
		instance.Spec.RBAC = &operatorv1alpha1.RBAC{Enable: true}
	}
	// serviceAccount
	if instance.Spec.ServiceAccount == nil {
		instance.Spec.ServiceAccount = &operatorv1alpha1.ServiceAccount{Enable: true, Name: instance.Name}
	}
	// SelectorLabels
	if len(instance.Spec.SelectorLabels) == 0 {
		instance.Spec.SelectorLabels = map[string]string{
			"app": instance.Spec.Controller.Name,
		}
	}
}
