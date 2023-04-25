package controllers

import (
	"context"
	"errors"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/annotations"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	k8slog "sigs.k8s.io/controller-runtime/pkg/log"

	infrastructurev1beta1 "github.com/Tomy2e/cluster-api-provider-scaleway/api/v1beta1"
	"github.com/Tomy2e/cluster-api-provider-scaleway/pkg/scope"
	scwClient "github.com/Tomy2e/cluster-api-provider-scaleway/pkg/service/scaleway/client"
	"github.com/Tomy2e/cluster-api-provider-scaleway/pkg/service/scaleway/instance"
)

// ScalewayMachineReconciler reconciles a ScalewayMachine object
type ScalewayMachineReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;update
//+kubebuilder:rbac:groups=cluster.x-k8s.io,resources=machines;machines/status,verbs=get;list;watch
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=scalewaymachines,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=scalewaymachines/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=scalewaymachines/finalizers,verbs=update

func (r *ScalewayMachineReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	log := k8slog.FromContext(ctx)

	scalewayMachine := &infrastructurev1beta1.ScalewayMachine{}
	if err := r.Get(ctx, req.NamespacedName, scalewayMachine); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	log = log.WithValues("ScalewayMachine", klog.KObj(scalewayMachine))

	machine, err := util.GetOwnerMachine(ctx, r.Client, scalewayMachine.ObjectMeta)
	if err != nil {
		return ctrl.Result{}, err
	}
	if machine == nil {
		log.Info("Machine Controller has not yet set OwnerRef")
		return ctrl.Result{}, nil
	}

	log = log.WithValues("Machine", klog.KObj(machine))

	// Fetch the Cluster.
	cluster, err := util.GetClusterFromMetadata(ctx, r.Client, machine.ObjectMeta)
	if err != nil {
		log.Info("Machine is missing cluster label or cluster does not exist")
		return ctrl.Result{}, nil
	}

	if annotations.IsPaused(cluster, scalewayMachine) {
		log.Info("ScalewayMachine or linked Cluster is marked as paused. Won't reconcile")
		return ctrl.Result{}, nil
	}

	log = log.WithValues("Cluster", klog.KObj(cluster))

	scalewayCluster := &infrastructurev1beta1.ScalewayCluster{}
	scalewayClusterName := client.ObjectKey{
		Namespace: cluster.Namespace,
		Name:      cluster.Spec.InfrastructureRef.Name,
	}
	if err := r.Client.Get(ctx, scalewayClusterName, scalewayCluster); err != nil {
		log.Info("ScalewayCluster is not available yet")
		return ctrl.Result{}, err
	}

	log = log.WithValues("ScalewayCluster", klog.KObj(scalewayCluster))
	ctx = ctrl.LoggerInto(ctx, log)

	c, err := clientFromSecret(ctx, r.Client, scalewayCluster)
	if err != nil {
		return ctrl.Result{}, err
	}

	machineScope, err := scope.NewMachine(&scope.MachineParams{
		ClusterParams: &scope.ClusterParams{
			Client:          r.Client,
			ScalewayClient:  scwClient.New(c),
			ScalewayCluster: scalewayCluster,
			Cluster:         cluster,
		},
		ScalewayMachine: scalewayMachine,
		Machine:         machine,
	})
	if err != nil {
		return ctrl.Result{}, err
	}

	defer func() {
		if err := machineScope.Close(ctx); err != nil && reterr == nil {
			reterr = err
		}
	}()

	if !scalewayMachine.ObjectMeta.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, machineScope)
	}

	return r.reconcileNormal(ctx, machineScope)

}

func (r *ScalewayMachineReconciler) reconcileNormal(ctx context.Context, machineScope *scope.Machine) (ctrl.Result, error) {
	log := k8slog.FromContext(ctx)

	if controllerutil.AddFinalizer(machineScope.ScalewayMachine, infrastructurev1beta1.MachineFinalizer) {
		if err := machineScope.PatchObject(ctx); err != nil {
			return ctrl.Result{}, err
		}
	}

	if !machineScope.Cluster.Cluster.Status.InfrastructureReady {
		log.Info("Infrastructure not ready yet")
		return ctrl.Result{RequeueAfter: 2 * time.Second}, nil
	}

	if err := instance.NewService(machineScope).Reconcile(ctx); err != nil {
		if errors.Is(err, instance.ErrPrivateIPNotFound) {
			log.Info("Private IP not available yet")
			return ctrl.Result{RequeueAfter: 2 * time.Second}, nil
		}

		if errors.Is(err, scope.ErrBootstrapDataNotReady) {
			log.Info("Bootstrap data not available yet")
			return ctrl.Result{RequeueAfter: 2 * time.Second}, nil
		}

		return ctrl.Result{}, err
	}

	machineScope.ScalewayMachine.Status.Ready = true

	return ctrl.Result{}, nil
}

func (r *ScalewayMachineReconciler) reconcileDelete(ctx context.Context, machineScope *scope.Machine) (ctrl.Result, error) {
	if err := instance.NewService(machineScope).Delete(ctx); err != nil {
		return ctrl.Result{}, err
	}

	controllerutil.RemoveFinalizer(machineScope.ScalewayMachine, infrastructurev1beta1.MachineFinalizer)

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ScalewayMachineReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&infrastructurev1beta1.ScalewayMachine{}).
		Complete(r)
}
