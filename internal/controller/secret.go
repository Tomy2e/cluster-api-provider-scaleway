package controller

import (
	"context"
	"fmt"

	infrastructurev1beta1 "github.com/Tomy2e/cluster-api-provider-scaleway/api/v1beta1"
	"github.com/scaleway/scaleway-sdk-go/scw"
	"golang.org/x/exp/slices"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func clientFromSecret(ctx context.Context, client client.Client, scalewayCluster *infrastructurev1beta1.ScalewayCluster) (*scw.Client, error) {
	// TODO: read secret: API URL, Access key, secret Key, ProjectID, default zone

	secret := &corev1.Secret{}
	if err := client.Get(ctx, types.NamespacedName{
		Namespace: scalewayCluster.Namespace,
		Name:      scalewayCluster.Spec.ScalewaySecretName,
	}, secret); err != nil {
		return nil, err
	}

	// Take ownership of secret.
	if !metav1.IsControlledBy(secret, scalewayCluster) {
		if !slices.ContainsFunc(secret.GetOwnerReferences(), func(o metav1.OwnerReference) bool {
			return o.UID == scalewayCluster.UID
		}) {
			if err := controllerutil.SetOwnerReference(scalewayCluster, secret, client.Scheme()); err != nil {
				return nil, fmt.Errorf("failed to set owner reference for secret %s: %w", secret.Name, err)
			}

			if err := client.Update(ctx, secret); err != nil {
				return nil, fmt.Errorf("failed to update secret %s: %w", secret.Name, err)
			}
		}
	}

	opts := []scw.ClientOption{
		scw.WithAuth(string(secret.Data["accessKey"]), string(secret.Data["secretKey"])),
		scw.WithDefaultProjectID(string(secret.Data["projectID"])),
	}

	if string(secret.Data["apiURL"]) != "" {
		opts = append(opts, scw.WithAPIURL(string(secret.Data["apiURL"])))
	}

	c, err := scw.NewClient(opts...)
	if err != nil {
		return nil, err
	}

	return c, nil
}
