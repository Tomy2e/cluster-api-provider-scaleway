package controller

import (
	"context"
	"errors"
	"fmt"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/annotations"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	infrastructurev1beta1 "github.com/Tomy2e/cluster-api-provider-scaleway/api/v1beta1"
	"github.com/Tomy2e/cluster-api-provider-scaleway/internal/scope"
	scwClient "github.com/Tomy2e/cluster-api-provider-scaleway/internal/service/scaleway/client"
	"github.com/Tomy2e/cluster-api-provider-scaleway/internal/service/scaleway/loadbalancer"
	"github.com/Tomy2e/cluster-api-provider-scaleway/internal/service/scaleway/securitygroup"
	"github.com/Tomy2e/cluster-api-provider-scaleway/internal/service/scaleway/vpc"
	"github.com/Tomy2e/cluster-api-provider-scaleway/internal/service/scaleway/vpcgw"
	"github.com/scaleway/scaleway-sdk-go/scw"
)

// ScalewayClusterReconciler reconciles a ScalewayCluster object
type ScalewayClusterReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters;clusters/status,verbs=get;list;watch
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=scalewayclusters,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=scalewayclusters/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=scalewayclusters/finalizers,verbs=update

func (r *ScalewayClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, retErr error) {
	l := log.FromContext(ctx)

	scalewayCluster := &infrastructurev1beta1.ScalewayCluster{}
	if err := r.Get(ctx, req.NamespacedName, scalewayCluster); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
	}

	l.WithValues("ScalewayCluster", klog.KObj(scalewayCluster))
	l.Info("Starting reconciling cluster")

	cluster, err := util.GetOwnerCluster(ctx, r.Client, scalewayCluster.ObjectMeta)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to get owner cluster: %w", err)
	}

	if cluster == nil {
		l.Info("Cluster Controller has not yet set OwnerRef")
		return ctrl.Result{
			RequeueAfter: 2 * time.Second,
		}, nil
	}

	if annotations.IsPaused(cluster, scalewayCluster) {
		l.Info("ScalewayCluster or linked Cluster is marked as paused. Won't reconcile")
		return ctrl.Result{}, nil
	}

	l = l.WithValues("Cluster", klog.KObj(cluster))
	ctx = log.IntoContext(ctx, l)

	c, err := clientFromSecret(ctx, r.Client, scalewayCluster)
	if err != nil {
		return ctrl.Result{}, err
	}

	clusterScope, err := scope.NewCluster(&scope.ClusterParams{
		Client:          r.Client,
		ScalewayCluster: scalewayCluster,
		Cluster:         cluster,
		ScalewayClient:  scwClient.New(c),
	})
	if err != nil {
		return ctrl.Result{}, err
	}

	defer func() {
		if err := clusterScope.Close(ctx); err != nil && retErr == nil {
			retErr = err
		}
	}()

	if !scalewayCluster.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, clusterScope)
	}

	return r.reconcileNormal(ctx, clusterScope)
}

func (r *ScalewayClusterReconciler) reconcileNormal(ctx context.Context, clusterScope *scope.Cluster) (ctrl.Result, error) {
	l := log.FromContext(ctx)

	if controllerutil.AddFinalizer(clusterScope.ScalewayCluster, infrastructurev1beta1.ClusterFinalizer) {
		if err := clusterScope.PatchObject(ctx); err != nil {
			return ctrl.Result{}, err
		}
	}

	clusterScope.ScalewayCluster.Status.FailureDomains = clusterScope.FailureDomains()

	if err := securitygroup.NewService(clusterScope).Reconcile(ctx); err != nil {
		return ctrl.Result{}, err
	}

	if err := vpc.NewService(clusterScope).Reconcile(ctx); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to reconcile vpc: %w", err)
	}

	// TODO: maybe wait for the gateway to be ready?
	if err := vpcgw.NewService(clusterScope).Reconcile(ctx); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to reconcile vpcgw: %w", err)
	}

	if err := loadbalancer.NewService(clusterScope).Reconcile(ctx); err != nil {
		if errors.Is(err, loadbalancer.ErrLoadBalancerNotReady) {
			l.Info("loadbalancer is not ready yet, retrying")
			return ctrl.Result{RequeueAfter: 2 * time.Second}, nil
		}
		return ctrl.Result{}, fmt.Errorf("failed to reconcile loadbalancer: %w", err)
	}

	clusterScope.ScalewayCluster.Status.Ready = true

	l.Info("Reconciled cluster successfully")

	return ctrl.Result{}, nil
}

func (r *ScalewayClusterReconciler) reconcileDelete(ctx context.Context, clusterScope *scope.Cluster) (ctrl.Result, error) {
	l := log.FromContext(ctx)

	l.Info("Deleting cluster")

	if err := loadbalancer.NewService(clusterScope).Delete(ctx); err != nil {
		return ctrl.Result{}, err
	}

	if err := vpcgw.NewService(clusterScope).Delete(ctx); err != nil {
		return ctrl.Result{}, err
	}

	if err := vpc.NewService(clusterScope).Delete(ctx); err != nil {
		var pfe *scw.PreconditionFailedError
		if errors.As(err, &pfe) {
			l.Info("cannot delete Private Network due to precondition failure, retrying", "err", err)
			return ctrl.Result{RequeueAfter: 2 * time.Second}, nil
		}
		return ctrl.Result{}, err
	}

	if err := securitygroup.NewService(clusterScope).Delete(ctx); err != nil {
		return ctrl.Result{}, err
	}

	controllerutil.RemoveFinalizer(clusterScope.ScalewayCluster, infrastructurev1beta1.ClusterFinalizer)

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ScalewayClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&infrastructurev1beta1.ScalewayCluster{}).
		Complete(r)
}
