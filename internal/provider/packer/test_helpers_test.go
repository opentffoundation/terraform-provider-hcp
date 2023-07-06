// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package packer_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/cenkalti/backoff"
	"github.com/go-openapi/strfmt"
	"github.com/google/uuid"
	"github.com/hashicorp/hcp-sdk-go/clients/cloud-operation/stable/2020-05-05/client/operation_service"
	"github.com/hashicorp/hcp-sdk-go/clients/cloud-packer-service/stable/2021-04-30/client/packer_service"
	"github.com/hashicorp/hcp-sdk-go/clients/cloud-packer-service/stable/2021-04-30/models"
	sharedmodels "github.com/hashicorp/hcp-sdk-go/clients/cloud-shared/v1/models"
	"github.com/hashicorp/terraform-provider-hcp/internal/clients"
	"github.com/hashicorp/terraform-provider-hcp/internal/provider/testhelpers"
	"google.golang.org/grpc/codes"
)

func upsertRegistry(t *testing.T) {
	t.Helper()

	client := testhelpers.DefaultProvider().Meta().(*clients.Client)
	loc := &sharedmodels.HashicorpCloudLocationLocation{
		OrganizationID: client.Config.OrganizationID,
		ProjectID:      client.Config.ProjectID,
	}

	params := packer_service.NewPackerServiceCreateRegistryParams()
	params.LocationOrganizationID = loc.OrganizationID
	params.LocationProjectID = loc.ProjectID
	featureTier := models.HashicorpCloudPackerRegistryConfigTierPLUS
	params.Body = packer_service.PackerServiceCreateRegistryBody{
		FeatureTier: &featureTier,
	}

	resp, err := client.Packer.PackerServiceCreateRegistry(params, nil)

	if err == nil {
		waitForOperation(t, loc, "Create Registry", resp.Payload.Operation.ID, client)
	}

	if err, ok := err.(*packer_service.PackerServiceCreateRegistryDefault); ok {
		switch err.Code() {
		case int(codes.AlreadyExists), http.StatusConflict:
			getParams := packer_service.NewPackerServiceGetRegistryParams()
			getParams.LocationOrganizationID = loc.OrganizationID
			getParams.LocationProjectID = loc.ProjectID
			getResp, err := client.Packer.PackerServiceGetRegistry(getParams, nil)
			if err != nil {
				t.Errorf("unexpected GetRegistry error: %v", err)
				return
			}
			if *getResp.Payload.Registry.Config.FeatureTier != models.HashicorpCloudPackerRegistryConfigTierPLUS {
				// Make sure is a plus registry
				params := packer_service.NewPackerServiceUpdateRegistryParams()
				params.LocationOrganizationID = loc.OrganizationID
				params.LocationProjectID = loc.ProjectID
				featureTier := models.HashicorpCloudPackerRegistryConfigTierPLUS
				params.Body = packer_service.PackerServiceUpdateRegistryBody{
					FeatureTier: &featureTier,
				}
				resp, err := client.Packer.PackerServiceUpdateRegistry(params, nil)
				if err != nil {
					t.Errorf("unexpected UpdateRegistry error: %v", err)
					return
				}
				waitForOperation(t, loc, "Reactivate Registry", resp.Payload.Operation.ID, client)
			}
			return
		default:
			t.Errorf("unexpected CreateRegistry error code, expected nil or 409. Got code: %d err: %v", err.Code(), err)
			return
		}
	}

	t.Errorf("unexpected CreateRegistry error, expected nil. Got: %v", err)
}

func waitForOperation(
	t *testing.T,
	loc *sharedmodels.HashicorpCloudLocationLocation,
	operationName string,
	operationID string,
	client *clients.Client,
) {
	timeout := "5s"
	params := operation_service.NewWaitParams()
	params.ID = operationID
	params.Timeout = &timeout
	params.LocationOrganizationID = loc.OrganizationID
	params.LocationProjectID = loc.ProjectID

	operation := func() error {
		resp, err := client.Operation.Wait(params, nil)
		if err != nil {
			t.Errorf("unexpected error %#v", err)
		}

		if resp.Payload.Operation.Error != nil {
			t.Errorf("Operation failed: %s", resp.Payload.Operation.Error.Message)
		}

		switch *resp.Payload.Operation.State {
		case sharedmodels.HashicorpCloudOperationOperationStatePENDING:
			msg := fmt.Sprintf("==> Operation \"%s\" pending...", operationName)
			return fmt.Errorf(msg)
		case sharedmodels.HashicorpCloudOperationOperationStateRUNNING:
			msg := fmt.Sprintf("==> Operation \"%s\" running...", operationName)
			return fmt.Errorf(msg)
		case sharedmodels.HashicorpCloudOperationOperationStateDONE:
		default:
			t.Errorf("Operation returned unknown state: %s", *resp.Payload.Operation.State)
		}
		return nil
	}

	bo := backoff.NewExponentialBackOff()
	bo.InitialInterval = 10 * time.Second
	bo.RandomizationFactor = 0.5
	bo.Multiplier = 1.5
	bo.MaxInterval = 30 * time.Second
	bo.MaxElapsedTime = 40 * time.Minute
	err := backoff.Retry(operation, bo)
	if err != nil {
		t.Errorf("unexpected error: %#v", err)
	}
}

func upsertBucket(t *testing.T, bucketSlug string) {
	t.Helper()

	client := testhelpers.DefaultProvider().Meta().(*clients.Client)
	loc := &sharedmodels.HashicorpCloudLocationLocation{
		OrganizationID: client.Config.OrganizationID,
		ProjectID:      client.Config.ProjectID,
	}

	createBktParams := packer_service.NewPackerServiceCreateBucketParams()
	createBktParams.LocationOrganizationID = loc.OrganizationID
	createBktParams.LocationProjectID = loc.ProjectID
	createBktParams.Body = packer_service.PackerServiceCreateBucketBody{
		BucketSlug: bucketSlug,
	}
	_, err := client.Packer.PackerServiceCreateBucket(createBktParams, nil)
	if err == nil {
		return
	}
	if err, ok := err.(*packer_service.PackerServiceCreateBucketDefault); ok {
		switch err.Code() {
		case int(codes.AlreadyExists), http.StatusConflict:
			// all good here !
			return
		}
	}

	t.Errorf("unexpected CreateBucket error, expected nil or 409. Got %v", err)
}

func upsertIteration(t *testing.T, bucketSlug, fingerprint string) *models.HashicorpCloudPackerIteration {
	t.Helper()

	client := testhelpers.DefaultProvider().Meta().(*clients.Client)
	loc := &sharedmodels.HashicorpCloudLocationLocation{
		OrganizationID: client.Config.OrganizationID,
		ProjectID:      client.Config.ProjectID,
	}

	createItParams := packer_service.NewPackerServiceCreateIterationParams()
	createItParams.LocationOrganizationID = loc.OrganizationID
	createItParams.LocationProjectID = loc.ProjectID
	createItParams.BucketSlug = bucketSlug
	createItParams.Body = packer_service.PackerServiceCreateIterationBody{
		Fingerprint: fingerprint,
	}

	iterationResp, err := client.Packer.PackerServiceCreateIteration(createItParams, nil)
	if err == nil {
		return iterationResp.Payload.Iteration
	} else if err, ok := err.(*packer_service.PackerServiceCreateIterationDefault); ok {
		switch err.Code() {
		case int(codes.AlreadyExists), http.StatusConflict:
			// all good here !
			getItParams := packer_service.NewPackerServiceGetIterationParams()
			getItParams.LocationOrganizationID = createItParams.LocationOrganizationID
			getItParams.LocationProjectID = createItParams.LocationProjectID
			getItParams.BucketSlug = createItParams.BucketSlug
			getItParams.Fingerprint = &createItParams.Body.Fingerprint
			iterationResp, err := client.Packer.PackerServiceGetIteration(getItParams, nil)
			if err != nil {
				t.Errorf("unexpected GetIteration error, expected nil. Got %v", err)
				return nil
			}
			return iterationResp.Payload.Iteration
		}
	}

	t.Errorf("unexpected CreateIteration error, expected nil or 409. Got %v", err)
	return nil
}

func upsertCompleteIteration(t *testing.T, bucketSlug, fingerprint string) *models.HashicorpCloudPackerIteration {
	iteration := upsertIteration(t, bucketSlug, fingerprint)
	if t.Failed() || iteration == nil {
		return nil
	}
	upsertBuild(t, bucketSlug, iteration.Fingerprint, iteration.ID)
	if t.Failed() {
		return nil
	}

	client := testhelpers.DefaultProvider().Meta().(*clients.Client)
	loc := &sharedmodels.HashicorpCloudLocationLocation{
		OrganizationID: client.Config.OrganizationID,
		ProjectID:      client.Config.ProjectID,
	}
	iteration, err := clients.GetIterationFromFingerprint(context.Background(), client, loc, bucketSlug, iteration.Fingerprint)
	if err != nil {
		t.Errorf("Complete iteration not found after upserting, received unexpected error. Got %v", err)
		return nil
	}

	return iteration
}

func revokeIteration(t *testing.T, iterationID, bucketSlug string, revokeAt strfmt.DateTime) {
	t.Helper()
	client := testhelpers.DefaultProvider().Meta().(*clients.Client)
	loc := &sharedmodels.HashicorpCloudLocationLocation{
		OrganizationID: client.Config.OrganizationID,
		ProjectID:      client.Config.ProjectID,
	}

	params := packer_service.NewPackerServiceUpdateIterationParams()
	params.LocationOrganizationID = loc.OrganizationID
	params.LocationProjectID = loc.ProjectID
	params.IterationID = iterationID
	params.Body = packer_service.PackerServiceUpdateIterationBody{
		BucketSlug: bucketSlug,
		RevokeAt:   revokeAt,
	}

	_, err := client.Packer.PackerServiceUpdateIteration(params, nil)
	if err != nil {
		t.Fatal(err)
	}
}

func getIterationIDFromFingerPrint(t *testing.T, bucketSlug string, fingerprint string) (string, error) {
	t.Helper()

	client := testhelpers.DefaultProvider().Meta().(*clients.Client)
	loc := &sharedmodels.HashicorpCloudLocationLocation{
		OrganizationID: client.Config.OrganizationID,
		ProjectID:      client.Config.ProjectID,
	}

	getItParams := packer_service.NewPackerServiceGetIterationParams()
	getItParams.LocationOrganizationID = loc.OrganizationID
	getItParams.LocationProjectID = loc.ProjectID
	getItParams.BucketSlug = bucketSlug
	getItParams.Fingerprint = &fingerprint

	ok, err := client.Packer.PackerServiceGetIteration(getItParams, nil)
	if err != nil {
		return "", err
	}
	return ok.Payload.Iteration.ID, nil
}

func upsertBuild(t *testing.T, bucketSlug, fingerprint, iterationID string) {
	client := testhelpers.DefaultProvider().Meta().(*clients.Client)

	createBuildParams := packer_service.NewPackerServiceCreateBuildParams()
	loc := &sharedmodels.HashicorpCloudLocationLocation{
		OrganizationID: client.Config.OrganizationID,
		ProjectID:      client.Config.ProjectID,
	}
	createBuildParams.LocationOrganizationID = loc.OrganizationID
	createBuildParams.LocationProjectID = loc.ProjectID
	createBuildParams.BucketSlug = bucketSlug
	createBuildParams.IterationID = iterationID

	status := models.HashicorpCloudPackerBuildStatusRUNNING
	createBuildParams.Body = packer_service.PackerServiceCreateBuildBody{
		Build: &models.HashicorpCloudPackerBuildCreateBody{
			CloudProvider: "aws",
			ComponentType: "amazon-ebs.example",
			PackerRunUUID: uuid.New().String(),
			Status:        &status,
		},
		Fingerprint: fingerprint,
	}

	build, err := client.Packer.PackerServiceCreateBuild(createBuildParams, nil)
	if err != nil {
		if err, ok := err.(*packer_service.PackerServiceCreateBuildDefault); ok {
			switch err.Code() {
			case int(codes.Aborted), http.StatusConflict:
				// all good here !
				return
			}
		}

		t.Errorf("unexpected CreateBuild error, expected nil. Got %v", err)
	}

	if build == nil {
		t.Errorf("unexpected CreateBuild response, expected non nil build. Got nil.")
		return
	}

	// Iterations are currently only assigned an incremental version when publishing image metadata on update.
	// Incremental versions are a requirement for assigning the channel.
	updateBuildParams := packer_service.NewPackerServiceUpdateBuildParams()
	updateBuildParams.LocationOrganizationID = loc.OrganizationID
	updateBuildParams.LocationProjectID = loc.ProjectID
	updateBuildParams.BuildID = build.Payload.Build.ID
	updatesStatus := models.HashicorpCloudPackerBuildStatusDONE
	updateBuildParams.Body = packer_service.PackerServiceUpdateBuildBody{
		Updates: &models.HashicorpCloudPackerBuildUpdates{
			Status: &updatesStatus,
			Images: []*models.HashicorpCloudPackerImageCreateBody{
				{
					ImageID: "ami-42",
					Region:  "us-east-1",
				},
				{
					ImageID: "ami-43",
					Region:  "us-east-2",
				},
			},
			Labels: map[string]string{"test-key": "test-value"},
		},
	}
	if _, err = client.Packer.PackerServiceUpdateBuild(updateBuildParams, nil); err != nil {
		t.Errorf("unexpected UpdateBuild error, expected nil. Got %v", err)
	}
}

func upsertChannel(t *testing.T, bucketSlug, channelSlug, iterationID string) {
	t.Helper()

	client := testhelpers.DefaultProvider().Meta().(*clients.Client)
	loc := &sharedmodels.HashicorpCloudLocationLocation{
		OrganizationID: client.Config.OrganizationID,
		ProjectID:      client.Config.ProjectID,
	}

	createChParams := packer_service.NewPackerServiceCreateChannelParams()
	createChParams.LocationOrganizationID = loc.OrganizationID
	createChParams.LocationProjectID = loc.ProjectID
	createChParams.BucketSlug = bucketSlug
	createChParams.Body = packer_service.PackerServiceCreateChannelBody{
		Slug:        channelSlug,
		IterationID: iterationID,
	}

	_, err := client.Packer.PackerServiceCreateChannel(createChParams, nil)
	if err == nil {
		return
	}
	if err, ok := err.(*packer_service.PackerServiceCreateChannelDefault); ok {
		switch err.Code() {
		case int(codes.AlreadyExists), http.StatusConflict:
			// all good here !
			updateChannelAssignment(t, bucketSlug, channelSlug, &models.HashicorpCloudPackerIteration{ID: iterationID})
			return
		}
	}
	t.Errorf("unexpected CreateChannel error, expected nil. Got %v", err)
}

func updateChannelAssignment(t *testing.T, bucketSlug string, channelSlug string, iteration *models.HashicorpCloudPackerIteration) {
	t.Helper()

	client := testhelpers.DefaultProvider().Meta().(*clients.Client)
	loc := &sharedmodels.HashicorpCloudLocationLocation{
		OrganizationID: client.Config.OrganizationID,
		ProjectID:      client.Config.ProjectID,
	}

	params := packer_service.NewPackerServiceUpdateChannelParams()
	params.LocationOrganizationID = loc.OrganizationID
	params.LocationProjectID = loc.ProjectID
	params.BucketSlug = bucketSlug
	params.Slug = channelSlug

	if iteration != nil {
		switch {
		case iteration.ID != "":
			params.Body.IterationID = iteration.ID
		case iteration.Fingerprint != "":
			params.Body.Fingerprint = iteration.Fingerprint
		case iteration.IncrementalVersion > 0:
			params.Body.IncrementalVersion = iteration.IncrementalVersion
		}
	}

	_, err := client.Packer.PackerServiceUpdateChannel(params, nil)
	if err == nil {
		return
	}
	t.Errorf("unexpected UpdateChannel error, expected nil. Got %v", err)
}

func deleteBucket(t *testing.T, bucketSlug string, logOnError bool) {
	t.Helper()

	client := testhelpers.DefaultProvider().Meta().(*clients.Client)
	loc := &sharedmodels.HashicorpCloudLocationLocation{
		OrganizationID: client.Config.OrganizationID,
		ProjectID:      client.Config.ProjectID,
	}

	deleteBktParams := packer_service.NewPackerServiceDeleteBucketParams()
	deleteBktParams.LocationOrganizationID = loc.OrganizationID
	deleteBktParams.LocationProjectID = loc.ProjectID
	deleteBktParams.BucketSlug = bucketSlug

	_, err := client.Packer.PackerServiceDeleteBucket(deleteBktParams, nil)
	if err == nil {
		return
	}
	if logOnError {
		t.Logf("unexpected DeleteBucket error, expected nil. Got %v", err)
	}
}

func deleteIteration(t *testing.T, bucketSlug string, iterationFingerprint string, logOnError bool) {
	t.Helper()

	client := testhelpers.DefaultProvider().Meta().(*clients.Client)
	loc := &sharedmodels.HashicorpCloudLocationLocation{
		OrganizationID: client.Config.OrganizationID,
		ProjectID:      client.Config.ProjectID,
	}

	iterationID, err := getIterationIDFromFingerPrint(t, bucketSlug, iterationFingerprint)
	if err != nil {
		if logOnError {
			t.Logf(err.Error())
		}
		return
	}

	deleteItParams := packer_service.NewPackerServiceDeleteIterationParams()
	deleteItParams.LocationOrganizationID = loc.OrganizationID
	deleteItParams.LocationProjectID = loc.ProjectID
	deleteItParams.BucketSlug = &bucketSlug
	deleteItParams.IterationID = iterationID

	_, err = client.Packer.PackerServiceDeleteIteration(deleteItParams, nil)
	if err == nil {
		return
	}
	if logOnError {
		t.Logf("unexpected DeleteIteration error, expected nil. Got %v", err)
	}
}

func deleteChannel(t *testing.T, bucketSlug string, channelSlug string, logOnError bool) {
	t.Helper()

	client := testhelpers.DefaultProvider().Meta().(*clients.Client)
	loc := &sharedmodels.HashicorpCloudLocationLocation{
		OrganizationID: client.Config.OrganizationID,
		ProjectID:      client.Config.ProjectID,
	}

	deleteChParams := packer_service.NewPackerServiceDeleteChannelParams()
	deleteChParams.LocationOrganizationID = loc.OrganizationID
	deleteChParams.LocationProjectID = loc.ProjectID
	deleteChParams.BucketSlug = bucketSlug
	deleteChParams.Slug = channelSlug

	_, err := client.Packer.PackerServiceDeleteChannel(deleteChParams, nil)
	if err == nil {
		return
	}
	if logOnError {
		t.Logf("unexpected DeleteChannel error, expected nil. Got %v", err)
	}
}
