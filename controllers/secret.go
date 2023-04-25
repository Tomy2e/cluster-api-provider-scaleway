package controllers

import (
	"context"

	infrastructurev1beta1 "github.com/Tomy2e/cluster-api-provider-scaleway/api/v1beta1"
	"github.com/scaleway/scaleway-sdk-go/scw"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func clientFromSecret(ctx context.Context, client client.Client, scalewayCluster *infrastructurev1beta1.ScalewayCluster) (*scw.Client, error) {
	// TODO: read secret: API URL, Access key, secret Key, ProjectID, default zone
	// TODO: take ownership of secret?

	secret := &corev1.Secret{}
	if err := client.Get(ctx, types.NamespacedName{
		Namespace: scalewayCluster.Namespace,
		Name:      scalewayCluster.Spec.ScalewaySecretName,
	}, secret); err != nil {
		return nil, err
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
