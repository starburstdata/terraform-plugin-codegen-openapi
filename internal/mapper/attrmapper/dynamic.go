// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package attrmapper

import (
	"github.com/starburstdata/terraform-plugin-codegen-openapi/internal/explorer"
	"github.com/starburstdata/terraform-plugin-codegen-openapi/internal/mapper/util"
	"github.com/hashicorp/terraform-plugin-codegen-spec/datasource"
	"github.com/hashicorp/terraform-plugin-codegen-spec/provider"
	"github.com/hashicorp/terraform-plugin-codegen-spec/resource"
)

type ResourceDynamicAttribute struct {
	resource.DynamicAttribute

	Name string
}

func (a *ResourceDynamicAttribute) GetName() string {
	return a.Name
}

func (a *ResourceDynamicAttribute) Merge(mergeAttribute ResourceAttribute) (ResourceAttribute, error) {
	dynamicAttribute, ok := mergeAttribute.(*ResourceDynamicAttribute)
	// TODO: return error if types don't match?
	if ok && (a.Description == nil || *a.Description == "") {
		a.Description = dynamicAttribute.Description
	}

	return a, nil
}

func (a *ResourceDynamicAttribute) ApplyOverride(override explorer.Override) (ResourceAttribute, error) {
	a.Description = &override.Description

	return a, nil
}

func (a *ResourceDynamicAttribute) ToSpec() resource.Attribute {
	return resource.Attribute{
		Name:    util.TerraformIdentifier(a.Name),
		Dynamic: &a.DynamicAttribute,
	}
}

type DataSourceDynamicAttribute struct {
	datasource.DynamicAttribute

	Name string
}

func (a *DataSourceDynamicAttribute) GetName() string {
	return a.Name
}

func (a *DataSourceDynamicAttribute) Merge(mergeAttribute DataSourceAttribute) (DataSourceAttribute, error) {
	dynamicAttribute, ok := mergeAttribute.(*DataSourceDynamicAttribute)
	// TODO: return error if types don't match?
	if ok && (a.Description == nil || *a.Description == "") {
		a.Description = dynamicAttribute.Description
	}

	return a, nil
}

func (a *DataSourceDynamicAttribute) ApplyOverride(override explorer.Override) (DataSourceAttribute, error) {
	a.Description = &override.Description

	return a, nil
}

func (a *DataSourceDynamicAttribute) ToSpec() datasource.Attribute {
	return datasource.Attribute{
		Name:    util.TerraformIdentifier(a.Name),
		Dynamic: &a.DynamicAttribute,
	}
}

type ProviderDynamicAttribute struct {
	provider.DynamicAttribute

	Name string
}

func (a *ProviderDynamicAttribute) ToSpec() provider.Attribute {
	return provider.Attribute{
		Name:    util.TerraformIdentifier(a.Name),
		Dynamic: &a.DynamicAttribute,
	}
}
