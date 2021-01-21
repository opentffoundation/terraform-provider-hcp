package clients

import (
	"context"

	"github.com/hashicorp/cloud-sdk-go/clients/cloud-network/preview/2020-09-07/client/network_service"
	networkmodels "github.com/hashicorp/cloud-sdk-go/clients/cloud-network/preview/2020-09-07/models"
	sharedmodels "github.com/hashicorp/cloud-sdk-go/clients/cloud-shared/v1/models"
)

// GetHvnByID gets an HVN by its ID
func GetHvnByID(ctx context.Context, client *Client, loc *sharedmodels.HashicorpCloudLocationLocation, hvnID string) (*networkmodels.HashicorpCloudNetwork20200907Network, error) {
	getParams := network_service.NewGetParams()
	getParams.Context = ctx
	getParams.ID = hvnID
	getParams.LocationOrganizationID = loc.OrganizationID
	getParams.LocationProjectID = loc.ProjectID
	getParams.LocationRegionProvider = &loc.Region.Provider
	getParams.LocationRegionRegion = &loc.Region.Region
	getResponse, err := client.Network.Get(getParams, nil)
	if err != nil {
		return nil, err
	}

	return getResponse.Payload.Network, nil
}