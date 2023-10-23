package v1beta1

import (
	"reflect"

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
var scalewaymachinelog = logf.Log.WithName("scalewaymachine-resource")

func (r *ScalewayMachine) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

//+kubebuilder:webhook:path=/validate-infrastructure-cluster-x-k8s-io-v1beta1-scalewaymachine,mutating=false,failurePolicy=fail,sideEffects=None,groups=infrastructure.cluster.x-k8s.io,resources=scalewaymachines,verbs=create;update,versions=v1beta1,name=vscalewaymachine.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &ScalewayMachine{}

func (r *ScalewayMachine) validate() error {
	var allErrs field.ErrorList

	if r.Spec.RootVolumeSize != nil && *r.Spec.RootVolumeSize < 5 {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec", "rootVolumeSize"), r.Spec.RootVolumeSize, "must be at least 5 GB"))
	}

	if allErrs == nil {
		return nil
	}

	return apierrors.NewInvalid(schema.GroupKind{Group: GroupVersion.Group, Kind: "ScalewayMachine"}, r.Name, allErrs)
}

func (r *ScalewayMachine) enforceImmutability(old *ScalewayMachine) error {
	var allErrs field.ErrorList

	// ProviderID can only be set once.
	if old.Spec.ProviderID != nil && !reflect.DeepEqual(r.Spec.ProviderID, old.Spec.ProviderID) {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec", "providerID"), r.Spec.ProviderID, "field is immutable"))
	}

	if r.Spec.Image != old.Spec.Image {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec", "image"), r.Spec.Image, "field is immutable"))
	}

	if r.Spec.Type != old.Spec.Type {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec", "type"), r.Spec.Type, "field is immutable"))
	}

	if !reflect.DeepEqual(old.Spec.RootVolumeSize, r.Spec.RootVolumeSize) {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec", "rootVolumeSize"), r.Spec.RootVolumeSize, "field is immutable"))
	}

	if !reflect.DeepEqual(old.Spec.RootVolumeType, r.Spec.RootVolumeType) {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec", "rootVolumeType"), r.Spec.RootVolumeType, "field is immutable"))
	}

	// Once PublicIP is set, it is immutable.
	if old.Spec.PublicIP != nil && !reflect.DeepEqual(old.Spec.PublicIP, r.Spec.PublicIP) {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec", "publicIP"), r.Spec.PublicIP, "field is immutable"))
	}

	// Only allow nil PublicIP field to be set to the default value (which is false).
	if old.Spec.PublicIP == nil && r.Spec.PublicIP != nil && *r.Spec.PublicIP {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec", "publicIP"), r.Spec.PublicIP, "field can only be set to false"))
	}

	// Cannot change SecurityGroupName.
	if !reflect.DeepEqual(old.Spec.SecurityGroupName, r.Spec.SecurityGroupName) {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec", "securityGroupName"), r.Spec.SecurityGroupName, "field is immutable"))
	}

	if allErrs == nil {
		return nil
	}

	return apierrors.NewInvalid(schema.GroupKind{Group: GroupVersion.Group, Kind: "ScalewayCluster"}, r.Name, allErrs)
}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *ScalewayMachine) ValidateCreate() (admission.Warnings, error) {
	scalewaymachinelog.Info("validate create", "name", r.Name)
	return nil, r.validate()
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *ScalewayMachine) ValidateUpdate(old runtime.Object) (admission.Warnings, error) {
	scalewaymachinelog.Info("validate update", "name", r.Name)

	if err := r.enforceImmutability(old.(*ScalewayMachine)); err != nil {
		return nil, err
	}

	return nil, r.validate()
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *ScalewayMachine) ValidateDelete() (admission.Warnings, error) {
	scalewaymachinelog.Info("validate delete", "name", r.Name)
	return nil, nil
}
