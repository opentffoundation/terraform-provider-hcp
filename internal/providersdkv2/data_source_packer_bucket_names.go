// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package providersdkv2

import (
	"context"
	"log"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
	"github.com/hashicorp/terraform-provider-hcp/internal/clients"
	"github.com/hashicorp/terraform-provider-hcp/internal/clients/packerv2"
)

func dataSourcePackerBucketNames() *schema.Resource {
	return &schema.Resource{
		Description: "The Packer Bucket Names data source gets the names of all of the buckets in a single HCP Packer registry.",
		ReadContext: dataSourcePackerBucketsRead,
		Timeouts: &schema.ResourceTimeout{
			Default: &defaultPackerTimeout,
		},
		Schema: map[string]*schema.Schema{
			// Optional inputs
			"project_id": {
				Description:  "The ID of the HCP project where the HCP Packer registry is located.",
				Type:         schema.TypeString,
				Computed:     true,
				Optional:     true,
				ValidateFunc: validation.IsUUID,
			},
			// Computed outputs
			"organization_id": {
				Description: "The ID of the organization where the HCP Packer registry is located.",
				Type:        schema.TypeString,
				Computed:    true,
			},
			"names": {
				Description: "A set of names for all buckets in the HCP Packer registry.",
				Type:        schema.TypeSet,
				Computed:    true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
			},
		},
	}
}

func dataSourcePackerBucketsRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*clients.Client)
	loc, err := getAndUpdateLocationResourceData(d, client)
	if err != nil {
		return diag.FromErr(err)
	}

	log.Printf("[INFO] Reading HCP Packer registry buckets [project_id=%s, organization_id=%s]", loc.ProjectID, loc.OrganizationID)

	bucketData, err := packerv2.ListBuckets(ctx, client, loc)
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId(loc.ProjectID)

	if err := d.Set("names", flattenPackerBucketsList(bucketData)); err != nil {
		return diag.FromErr(err)
	}

	return nil
}

func flattenPackerBucketsList(buckets []*packerv2.Bucket) []string {
	var names []string

	for _, bucket := range buckets {

		names = append(names, bucket.Name)
	}
	return names
}
