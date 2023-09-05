package v1beta1

import (
	"reflect"

	"github.com/scaleway/scaleway-sdk-go/scw"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// log is for logging in this package.
var scalewayclusterlog = logf.Log.WithName("scalewaycluster-resource")

func (r *ScalewayCluster) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

//+kubebuilder:webhook:path=/validate-infrastructure-cluster-x-k8s-io-v1beta1-scalewaycluster,mutating=false,failurePolicy=fail,sideEffects=None,groups=infrastructure.cluster.x-k8s.io,resources=scalewayclusters,verbs=create;update,versions=v1beta1,name=vscalewaycluster.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &ScalewayCluster{}

func (r *ScalewayCluster) validate() error {
	region, err := r.validateRegion()
	if err != nil {
		return apierrors.NewInvalid(schema.GroupKind{Group: GroupVersion.Group, Kind: "ScalewayCluster"}, r.Name, field.ErrorList{err})

	}

	var allErrs field.ErrorList

	if err := r.validateFailureDomains(region); err != nil {
		allErrs = append(allErrs, err)
	}

	if err := r.validateLoadBalancerSpec(region); err != nil {
		allErrs = append(allErrs, err)
	}

	if err := r.validateNetworkSpec(region); err != nil {
		allErrs = append(allErrs, err)
	}

	if allErrs == nil {
		return nil
	}

	return apierrors.NewInvalid(schema.GroupKind{Group: GroupVersion.Group, Kind: "ScalewayCluster"}, r.Name, allErrs)
}

func (r *ScalewayCluster) validateRegion() (scw.Region, *field.Error) {
	region, err := scw.ParseRegion(r.Spec.Region)
	if err != nil {
		return scw.Region(""), field.Invalid(field.NewPath("spec", "region"), r.Spec.Region, err.Error())
	}

	return region, nil
}

func (r *ScalewayCluster) validateFailureDomains(region scw.Region) *field.Error {
	if len(r.Spec.FailureDomains) == 0 {
		return nil
	}

	// If set, FailureDomains must:
	// - have no duplicates
	// - be in the same region as the cluster region
	dupeMap := make(map[scw.Zone]struct{})

	for i, fd := range r.Spec.FailureDomains {
		f := field.NewPath("spec", "failureDomains").Index(i)
		zone, err := scw.ParseZone(fd)
		if err != nil {
			return field.Invalid(f, fd, err.Error())
		}

		zoneRegion, err := zone.Region()
		if err != nil {
			return field.Invalid(f, fd, err.Error())
		}

		if region != zoneRegion {
			return field.Invalid(f, fd, "failureDomain must be in the cluster region")
		}

		if _, ok := dupeMap[zone]; ok {
			return field.Duplicate(f, fd)
		}

		dupeMap[zone] = struct{}{}
	}

	return nil
}

func (r *ScalewayCluster) validateLoadBalancerSpec(region scw.Region) *field.Error {
	if r.Spec.ControlPlaneLoadBalancer == nil || r.Spec.ControlPlaneLoadBalancer.Zone == nil {
		return nil
	}

	// Zone:
	// - must be valid
	// - in the same region as the cluster region
	f := field.NewPath("spec", "controlPlaneLoadBalancer", "zone")
	zone, err := scw.ParseZone(*r.Spec.ControlPlaneLoadBalancer.Zone)
	if err != nil {
		return field.Invalid(f, *r.Spec.ControlPlaneLoadBalancer.Zone, err.Error())
	}

	zoneRegion, err := zone.Region()
	if err != nil {
		return field.Invalid(f, *r.Spec.ControlPlaneLoadBalancer.Zone, err.Error())
	}

	if zoneRegion != region {
		return field.Invalid(f, *r.Spec.ControlPlaneLoadBalancer.Zone, "loadbalancer zone must be in the cluster region")
	}

	return nil
}

func (r *ScalewayCluster) validateNetworkSpec(region scw.Region) *field.Error {
	// Currently, this field is mandatory.
	if r.Spec.Network == nil {
		return field.Invalid(
			field.NewPath("spec", "network"),
			false,
			"network must be set",
		)
	}

	// Currently, using a PrivateNetwork is mandatory.
	if r.Spec.Network.PrivateNetwork == nil || !r.Spec.Network.PrivateNetwork.Enabled {
		return field.Invalid(
			field.NewPath("spec", "network", "privateNetwork", "enabled"),
			false,
			"private network must be enabled",
		)
	}

	// Currently, using a PublicGateway is mandatory.
	if r.Spec.Network.PublicGateway == nil || !r.Spec.Network.PublicGateway.Enabled {
		return field.Invalid(
			field.NewPath("spec", "network", "publicGateway", "enabled"),
			false,
			"public gateway must be enabled",
		)
	}

	if r.Spec.Network.PublicGateway.Zone != nil {
		zone, err := scw.ParseZone(*r.Spec.Network.PublicGateway.Zone)
		if err != nil {
			return field.Invalid(
				field.NewPath("spec", "network", "publicGateway", "zone"),
				*r.Spec.Network.PublicGateway.Zone,
				err.Error(),
			)
		}

		zoneRegion, err := zone.Region()
		if err != nil {
			return field.Invalid(
				field.NewPath("spec", "network", "publicGateway", "zone"),
				*r.Spec.Network.PublicGateway.Zone,
				err.Error(),
			)
		}

		if region != zoneRegion {
			return field.Invalid(
				field.NewPath("spec", "network", "publicGateway", "zone"),
				*r.Spec.Network.PublicGateway.Zone,
				"public gateway must be in the cluster region",
			)
		}
	}

	if r.Spec.Network.PublicGateway.ID != nil {
		if r.Spec.Network.PublicGateway.Type != nil {
			return field.Invalid(
				field.NewPath("spec", "network", "publicGateway", "type"),
				*r.Spec.Network.PublicGateway.Type,
				"type should not be specified because id is set",
			)
		}

		if r.Spec.Network.PublicGateway.IP != nil {
			return field.Invalid(
				field.NewPath("spec", "network", "publicGateway", "ip"),
				*r.Spec.Network.PublicGateway.IP,
				"ip should not be specified because id is set",
			)
		}

		if r.Spec.Network.PublicGateway.Zone == nil {
			return field.Invalid(
				field.NewPath("spec", "network", "publicGateway", "zone"),
				*r.Spec.Network.PublicGateway.Zone,
				"zone is needed",
			)
		}
	}

	return nil
}

func (r *ScalewayCluster) enforceImmutability(old *ScalewayCluster) error {
	var allErrs field.ErrorList

	if r.Spec.Region != old.Spec.Region {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec", "region"), r.Spec.Region, "field is immutable"))
	}

	if r.Spec.ScalewaySecretName != old.Spec.ScalewaySecretName {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec", "scalewaySecretName"), r.Spec.ScalewaySecretName, "field is immutable"))
	}

	if !reflect.DeepEqual(r.Spec.Network, old.Spec.Network) {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec", "network"), r.Spec.Network, "field is immutable"))
	}

	if old.Spec.ControlPlaneLoadBalancer == nil {
		old.Spec.ControlPlaneLoadBalancer = &LoadBalancerSpec{}
	}

	if r.Spec.ControlPlaneLoadBalancer == nil {
		r.Spec.ControlPlaneLoadBalancer = &LoadBalancerSpec{}
	}

	if !reflect.DeepEqual(old.Spec.ControlPlaneLoadBalancer.Zone, r.Spec.ControlPlaneLoadBalancer.Zone) {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec", "controlPlaneLoadBalancer", "zone"), r.Spec.ControlPlaneLoadBalancer.Zone, "field is immutable"))
	}

	if !reflect.DeepEqual(old.Spec.ControlPlaneLoadBalancer.IP, r.Spec.ControlPlaneLoadBalancer.IP) {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec", "controlPlaneLoadBalancer", "ip"), r.Spec.ControlPlaneLoadBalancer.IP, "field is immutable"))
	}

	if allErrs == nil {
		return nil
	}

	return apierrors.NewInvalid(schema.GroupKind{Group: GroupVersion.Group, Kind: "ScalewayCluster"}, r.Name, allErrs)
}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *ScalewayCluster) ValidateCreate() (admission.Warnings, error) {
	scalewayclusterlog.Info("validate create", "name", r.Name)

	return nil, r.validate()
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *ScalewayCluster) ValidateUpdate(old runtime.Object) (admission.Warnings, error) {
	scalewayclusterlog.Info("validate update", "name", r.Name)

	if err := r.enforceImmutability(old.(*ScalewayCluster)); err != nil {
		return nil, err
	}

	return nil, r.validate()
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *ScalewayCluster) ValidateDelete() (admission.Warnings, error) {
	scalewayclusterlog.Info("validate delete", "name", r.Name)
	return nil, nil
}
