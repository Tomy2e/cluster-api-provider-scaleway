package client

import (
	"errors"

	"github.com/scaleway/scaleway-sdk-go/api/instance/v1"
	ipam "github.com/scaleway/scaleway-sdk-go/api/ipam/v1alpha1"
	"github.com/scaleway/scaleway-sdk-go/api/lb/v1"
	"github.com/scaleway/scaleway-sdk-go/api/marketplace/v1"
	"github.com/scaleway/scaleway-sdk-go/api/vpc/v2"
	"github.com/scaleway/scaleway-sdk-go/api/vpcgw/v1"
	"github.com/scaleway/scaleway-sdk-go/scw"
)

var (
	ErrNoItemFound       = errors.New("no item found")
	ErrTooManyItemsFound = errors.New("expected to find only one item")
)

type Client struct {
	ProjectID     string
	LoadBalancer  *lb.ZonedAPI
	Instance      *instance.API
	Marketplace   *marketplace.API
	VPC           *vpc.API
	VPCGW         *vpcgw.API
	IPAM          *ipam.API
	PublicGateway *vpcgw.API
}

// client MUST have a default project ID...
func New(client *scw.Client) *Client {
	projectID, ok := client.GetDefaultProjectID()
	if !ok {
		panic("missing projectID")
	}

	return &Client{
		ProjectID:     projectID,
		LoadBalancer:  lb.NewZonedAPI(client),
		Instance:      instance.NewAPI(client),
		Marketplace:   marketplace.NewAPI(client),
		VPC:           vpc.NewAPI(client),
		VPCGW:         vpcgw.NewAPI(client),
		IPAM:          ipam.NewAPI(client),
		PublicGateway: vpcgw.NewAPI(client),
	}
}
