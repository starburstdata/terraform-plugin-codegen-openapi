// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package oas

import (
	"github.com/starburstdata/terraform-plugin-codegen-openapi/internal/mapper/attrmapper"
	"github.com/hashicorp/terraform-plugin-codegen-spec/datasource"
	"github.com/hashicorp/terraform-plugin-codegen-spec/provider"
	"github.com/hashicorp/terraform-plugin-codegen-spec/resource"
	"github.com/hashicorp/terraform-plugin-codegen-spec/schema"
)

// BuildDynamicResource builds a dynamic attribute. It is used to represent a schema
// edge that cannot be expressed as a typed Terraform attribute - in particular a
// self-referential (recursive) schema, which Terraform's plugin framework has no
// recursive nested type for. Degrading such an edge to `dynamic` keeps the surrounding
// resource generatable and the recursive structure authorable.
func (s *OASSchema) BuildDynamicResource(name string, computability schema.ComputedOptionalRequired) (attrmapper.ResourceAttribute, *SchemaError) {
	result := &attrmapper.ResourceDynamicAttribute{
		Name: name,
		DynamicAttribute: resource.DynamicAttribute{
			ComputedOptionalRequired: computability,
			DeprecationMessage:       s.GetDeprecationMessage(),
			Description:              s.GetDescription(),
			Sensitive:                s.IsSensitive(),
		},
	}

	return result, nil
}

func (s *OASSchema) BuildDynamicDataSource(name string, computability schema.ComputedOptionalRequired) (attrmapper.DataSourceAttribute, *SchemaError) {
	result := &attrmapper.DataSourceDynamicAttribute{
		Name: name,
		DynamicAttribute: datasource.DynamicAttribute{
			ComputedOptionalRequired: computability,
			DeprecationMessage:       s.GetDeprecationMessage(),
			Description:              s.GetDescription(),
			Sensitive:                s.IsSensitive(),
		},
	}

	return result, nil
}

func (s *OASSchema) BuildDynamicProvider(name string, optionalOrRequired schema.OptionalRequired) (attrmapper.ProviderAttribute, *SchemaError) {
	result := &attrmapper.ProviderDynamicAttribute{
		Name: name,
		DynamicAttribute: provider.DynamicAttribute{
			OptionalRequired:   optionalOrRequired,
			DeprecationMessage: s.GetDeprecationMessage(),
			Description:        s.GetDescription(),
			Sensitive:          s.IsSensitive(),
		},
	}

	return result, nil
}
