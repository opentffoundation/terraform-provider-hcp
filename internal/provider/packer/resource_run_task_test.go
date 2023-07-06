// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package packer_test

import (
	"context"
	"testing"

	"github.com/hashicorp/hcp-sdk-go/clients/cloud-packer-service/stable/2021-04-30/models"
	sharedmodels "github.com/hashicorp/hcp-sdk-go/clients/cloud-shared/v1/models"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/hashicorp/terraform-provider-hcp/internal/clients"
	"github.com/hashicorp/terraform-provider-hcp/internal/provider/testhelpers"
)

func TestAccPackerRunTask(t *testing.T) {
	runTask := runTaskConfigBuilder("runTask", `false`)
	config := testhelpers.BuildTestConfig(runTask)
	runTaskRegen := runTaskConfigBuilderFromRunTask(runTask, `true`)
	configRegen := testhelpers.BuildTestConfig(runTaskRegen)

	getHmacBeforeStep := func(hmacPtr *string) func() {
		return func() {
			client := testhelpers.DefaultProvider().Meta().(*clients.Client)
			loc := &sharedmodels.HashicorpCloudLocationLocation{
				OrganizationID: client.Config.OrganizationID,
				ProjectID:      client.Config.ProjectID,
			}
			resp, err := clients.GetRunTask(context.Background(), client, loc)
			if err != nil {
				t.Errorf("failed to get run task before test step, received error: %v", err)
				return
			}
			*hmacPtr = resp.HmacKey
		}
	}

	var preStep2HmacKey string
	var preStep3HmacKey string
	var preStep4HmacKey string

	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testhelpers.PreCheck(t, map[string]bool{"aws": false, "azure": false})
			upsertRegistry(t)
		},
		ProviderFactories: testhelpers.ProviderFactories(),
		Steps: []resource.TestStep{
			{
				Config: config,
				Check:  checkRunTaskStateMatchesAPI(runTask.ResourceName()),
			},
			{ // Ensure HMAC key is different after apply
				PreConfig: getHmacBeforeStep(&preStep2HmacKey),
				Config:    configRegen,
				Check: resource.ComposeAggregateTestCheckFunc(
					checkRunTaskStateMatchesAPI(runTaskRegen.ResourceName()),
					testhelpers.CheckResourceAttrPtrDifferent(runTaskRegen.ResourceName(), "hmac_key", &preStep2HmacKey),
				),
				ExpectNonEmptyPlan: true, // `regenerate_hmac = true` creates a perpetual diff
			},
			{ // Ensure that repetitive applies without changes still regenerate the HMAC key
				PreConfig: getHmacBeforeStep(&preStep3HmacKey),
				Config:    configRegen,
				Check: resource.ComposeAggregateTestCheckFunc(
					checkRunTaskStateMatchesAPI(runTaskRegen.ResourceName()),
					testhelpers.CheckResourceAttrPtrDifferent(runTaskRegen.ResourceName(), "hmac_key", &preStep3HmacKey),
				),
				ExpectNonEmptyPlan: true, // `regenerate_hmac = true` creates a perpetual diff
			},
			{ // Ensure that applies with regeneration off don't regenerate
				PreConfig: getHmacBeforeStep(&preStep4HmacKey),
				Config:    config,
				Check: resource.ComposeAggregateTestCheckFunc(
					checkRunTaskStateMatchesAPI(runTaskRegen.ResourceName()),
					resource.TestCheckResourceAttrPtr(runTask.ResourceName(), "hmac_key", &preStep4HmacKey),
				),
			},
		},
	})
}

func runTaskConfigBuilder(uniqueName string, regenerateHmac string) testhelpers.ConfigBuilder {
	return testhelpers.NewResourceConfigBuilder(
		"hcp_packer_run_task",
		uniqueName,
		map[string]string{
			"regenerate_hmac": regenerateHmac,
		},
	)
}

func runTaskConfigBuilderFromRunTask(oldRT testhelpers.ConfigBuilder, regenerateHmac string) testhelpers.ConfigBuilder {
	return runTaskConfigBuilder(
		oldRT.UniqueName(),
		regenerateHmac,
	)
}

func pullRunTaskFromAPIWithRunTaskState(resourceName string, state *terraform.State) (*models.HashicorpCloudPackerGetRegistryTFCRunTaskAPIResponse, error) {
	client := testhelpers.DefaultProvider().Meta().(*clients.Client)

	loc, _ := testhelpers.GetLocationFromState(resourceName, state)

	resp, err := clients.GetRunTask(context.Background(), client, loc)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func checkRunTaskStateMatchesRunTask(resourceName string, runTaskPtr **models.HashicorpCloudPackerGetRegistryTFCRunTaskAPIResponse) resource.TestCheckFunc {
	return func(state *terraform.State) error {
		var runTask *models.HashicorpCloudPackerGetRegistryTFCRunTaskAPIResponse
		if runTaskPtr != nil {
			runTask = *runTaskPtr
		}
		if runTask == nil {
			runTask = &models.HashicorpCloudPackerGetRegistryTFCRunTaskAPIResponse{}
		}

		return resource.ComposeAggregateTestCheckFunc(
			resource.TestCheckResourceAttr(resourceName, "endpoint_url", runTask.APIURL),
			resource.TestCheckResourceAttr(resourceName, "hmac_key", runTask.HmacKey),
		)(state)
	}
}

func checkRunTaskStateMatchesAPI(resourceName string) resource.TestCheckFunc {
	return func(state *terraform.State) error {
		runTask, err := pullRunTaskFromAPIWithRunTaskState(resourceName, state)
		if err != nil {
			return err
		}

		return checkRunTaskStateMatchesRunTask(resourceName, &runTask)(state)
	}
}
