package controllers

import (
	"context"
	"errors"
	"fmt"
	"time"

	scwClient "github.com/Tomy2e/cluster-api-provider-scaleway/pkg/service/scaleway/client"
	"github.com/Tomy2e/cluster-api-provider-scaleway/pkg/service/scaleway/vpc"
	"github.com/Tomy2e/cluster-api-provider-scaleway/pkg/service/scaleway/vpcgw"
	"github.com/scaleway/scaleway-sdk-go/scw"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	"sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/annotations"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	k8slog "sigs.k8s.io/controller-runtime/pkg/log"

	infrastructurev1beta1 "github.com/Tomy2e/cluster-api-provider-scaleway/api/v1beta1"
	"github.com/Tomy2e/cluster-api-provider-scaleway/pkg/scope"
	"github.com/Tomy2e/cluster-api-provider-scaleway/pkg/service/scaleway/loadbalancer"
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

func (r *ScalewayClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	log := k8slog.FromContext(ctx)

	scalewayCluster := &infrastructurev1beta1.ScalewayCluster{}
	if err := r.Get(ctx, req.NamespacedName, scalewayCluster); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
	}

	log.WithValues("ScalewayCluster", klog.KObj(scalewayCluster))
	log.Info("Starting reconciling cluster")

	cluster, err := util.GetOwnerCluster(ctx, r.Client, scalewayCluster.ObjectMeta)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to get owner cluster: %w", err)
	}

	if cluster == nil {
		log.Info("Cluster Controller has not yet set OwnerRef")
		return ctrl.Result{
			RequeueAfter: 2 * time.Second,
		}, nil
	}

	if annotations.IsPaused(cluster, scalewayCluster) {
		log.Info("ScalewayCluster or linked Cluster is marked as paused. Won't reconcile")
		return ctrl.Result{}, nil
	}

	log = log.WithValues("Cluster", klog.KObj(cluster))
	ctx = k8slog.IntoContext(ctx, log)

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
		if err := clusterScope.Close(ctx); err != nil && reterr == nil {
			reterr = err
		}
	}()

	if !scalewayCluster.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, clusterScope)
	}

	return r.reconcileNormal(ctx, clusterScope)
}

func (r *ScalewayClusterReconciler) reconcileNormal(ctx context.Context, clusterScope *scope.Cluster) (ctrl.Result, error) {
	log := k8slog.FromContext(ctx)

	if controllerutil.AddFinalizer(clusterScope.ScalewayCluster, infrastructurev1beta1.ClusterFinalizer) {
		if err := clusterScope.PatchObject(ctx); err != nil {
			return ctrl.Result{}, err
		}
	}

	zones := clusterScope.Region().GetZones()

	if len(zones) == 0 {
		zones = append(zones, scw.Zone(fmt.Sprintf("%s-1", clusterScope.Region())))
	}

	failureDomains := make(v1beta1.FailureDomains, len(zones))
	for _, zone := range zones {
		if len(clusterScope.ScalewayCluster.Spec.FailureDomains) > 0 {
			for _, fd := range clusterScope.ScalewayCluster.Spec.FailureDomains {
				if fd == zone.String() {
					failureDomains[zone.String()] = v1beta1.FailureDomainSpec{
						ControlPlane: true,
					}
				}
			}
		} else {
			failureDomains[zone.String()] = v1beta1.FailureDomainSpec{
				ControlPlane: true,
			}
		}
	}

	clusterScope.ScalewayCluster.Status.FailureDomains = failureDomains

	if err := vpc.NewService(clusterScope).Reconcile(ctx); err != nil {
		return ctrl.Result{}, err
	}

	if err := vpcgw.NewService(clusterScope).Reconcile(ctx); err != nil {
		return ctrl.Result{}, err
	}

	if err := loadbalancer.NewService(clusterScope).Reconcile(ctx); err != nil {
		if errors.Is(err, loadbalancer.ErrLoadBalancerNotReady) {
			log.Info("loadbalancer is not ready yet, retrying")
			return ctrl.Result{RequeueAfter: 2 * time.Second}, nil
		}
		return ctrl.Result{}, err
	}

	clusterScope.ScalewayCluster.Status.Ready = true

	return ctrl.Result{}, nil
}

func (r *ScalewayClusterReconciler) reconcileDelete(ctx context.Context, clusterScope *scope.Cluster) (ctrl.Result, error) {
	log := k8slog.FromContext(ctx)

	log.Info("deleting ScalewayCluster")

	if err := loadbalancer.NewService(clusterScope).Delete(ctx); err != nil {
		return ctrl.Result{}, err
	}

	if err := vpcgw.NewService(clusterScope).Delete(ctx); err != nil {
		return ctrl.Result{}, err
	}

	if err := vpc.NewService(clusterScope).Delete(ctx); err != nil {
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
