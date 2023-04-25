package scope

import (
	"context"
	"fmt"

	infrastructurev1beta1 "github.com/Tomy2e/cluster-api-provider-scaleway/api/v1beta1"
	"github.com/pkg/errors"
	"github.com/scaleway/scaleway-sdk-go/scw"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/patch"
)

var ErrBootstrapDataNotReady = errors.New("error retrieving bootstrap data: linked Machine's bootstrap.dataSecretName is nil")

type Machine struct {
	Cluster
	ScalewayMachine *infrastructurev1beta1.ScalewayMachine
	Machine         *v1beta1.Machine
}

type MachineParams struct {
	*ClusterParams
	ScalewayMachine *infrastructurev1beta1.ScalewayMachine
	Machine         *v1beta1.Machine
}

func NewMachine(params *MachineParams) (*Machine, error) {
	clusterScope, err := NewCluster(params.ClusterParams)
	if err != nil {
		return nil, err
	}

	clusterScope.patchHelper, err = patch.NewHelper(params.ScalewayMachine, params.Client)
	if err != nil {
		return nil, fmt.Errorf("failed to init patch helper: %w", err)
	}

	return &Machine{
		Cluster:         *clusterScope,
		ScalewayMachine: params.ScalewayMachine,
		Machine:         params.Machine,
	}, nil
}

func (m *Machine) PatchObject(ctx context.Context) error {
	return m.patchHelper.Patch(ctx, m.ScalewayMachine)
}

func (m *Machine) Close(ctx context.Context) error {
	return m.PatchObject(ctx)
}

func (m *Machine) GetRawBootstrapData(ctx context.Context) ([]byte, error) {
	if m.Machine.Spec.Bootstrap.DataSecretName == nil {
		return nil, ErrBootstrapDataNotReady
	}

	key := types.NamespacedName{Namespace: m.Machine.GetNamespace(), Name: *m.Machine.Spec.Bootstrap.DataSecretName}
	secret := &corev1.Secret{}
	if err := m.Cluster.Client.Get(ctx, key, secret); err != nil {
		return nil, err
	}

	value, ok := secret.Data["value"]
	if !ok {
		return nil, errors.New("error retrieving bootstrap data: secret value key is missing")
	}

	return value, nil
}

func (m *Machine) NeedsPublicIP() bool {
	if m.ScalewayMachine.Spec.PublicIP != nil {
		return *m.ScalewayMachine.Spec.PublicIP
	}

	return false
}

func (m *Machine) Tags() []string {
	return []string{
		fmt.Sprintf("caps-cluster=%s", m.ScalewayCluster.Name),
		fmt.Sprintf("caps-node=%s", m.ScalewayMachine.Name),
	}
}

func (m *Machine) Zone() scw.Zone {
	if m.Machine.Spec.FailureDomain == nil {
		return scw.Zone(fmt.Sprintf("%s-1", m.Cluster.Region()))
	}

	return scw.Zone(*m.Machine.Spec.FailureDomain)
}

// Name returns the name that resources created for the machine should have.
func (m *Machine) Name() string {
	return fmt.Sprintf("caps-%s", m.ScalewayMachine.Name)
}

func (m *Machine) ProviderID(serverID string) string {
	return fmt.Sprintf("scaleway://instance/%s/%s", m.Zone(), serverID)
}
