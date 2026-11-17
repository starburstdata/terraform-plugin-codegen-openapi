// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package oas

import (
	"context"
	"crypto/sha256"
	"strings"

	"github.com/hashicorp/terraform-plugin-codegen-spec/datasource"
	"github.com/hashicorp/terraform-plugin-codegen-spec/provider"
	"github.com/hashicorp/terraform-plugin-codegen-spec/resource"
	"github.com/hashicorp/terraform-plugin-codegen-spec/schema"
	"github.com/pb33f/libopenapi/datamodel/high/base"
	"github.com/pb33f/libopenapi/orderedmap"
	"github.com/starburstdata/terraform-plugin-codegen-openapi/internal/mapper/attrmapper"
	"github.com/starburstdata/terraform-plugin-codegen-openapi/internal/mapper/util"
)

const recursiveJSONNote = "Encoded as a JSON string because this type is recursive."

// BuildJSONStringResource builds a string attribute for a recursive schema edge. Terraform has no
// recursive nested type and does not allow dynamic values inside collections, so the recursive
// subtree is carried as JSON text.
func (s *OASSchema) BuildJSONStringResource(name string, computability schema.ComputedOptionalRequired) (attrmapper.ResourceAttribute, *SchemaError) {
	return &attrmapper.ResourceStringAttribute{
		Name: name,
		StringAttribute: resource.StringAttribute{
			ComputedOptionalRequired: computability,
			DeprecationMessage:       s.GetDeprecationMessage(),
			Description:              s.getRecursiveJSONDescription(),
			Sensitive:                s.jsonSensitive(),
		},
	}, nil
}

func (s *OASSchema) BuildJSONStringDataSource(name string, computability schema.ComputedOptionalRequired) (attrmapper.DataSourceAttribute, *SchemaError) {
	return &attrmapper.DataSourceStringAttribute{
		Name: name,
		StringAttribute: datasource.StringAttribute{
			ComputedOptionalRequired: computability,
			DeprecationMessage:       s.GetDeprecationMessage(),
			Description:              s.getRecursiveJSONDescription(),
			Sensitive:                s.jsonSensitive(),
		},
	}, nil
}

func (s *OASSchema) BuildJSONStringProvider(name string, optionalOrRequired schema.OptionalRequired) (attrmapper.ProviderAttribute, *SchemaError) {
	return &attrmapper.ProviderStringAttribute{
		Name: name,
		StringAttribute: provider.StringAttribute{
			OptionalRequired:   optionalOrRequired,
			DeprecationMessage: s.GetDeprecationMessage(),
			Description:        s.getRecursiveJSONDescription(),
			Sensitive:          s.jsonSensitive(),
		},
	}, nil
}

// jsonSensitive marks the JSON string sensitive when the schema or anything nested inside the
// collapsed subtree is a password field.
func (s *OASSchema) jsonSensitive() *bool {
	if s.IsSensitive() != nil || containsSensitive(base.CreateSchemaProxy(s.Schema)) {
		sensitive := true
		return &sensitive
	}

	return nil
}

func (s *OASSchema) getRecursiveJSONDescription() *string {
	description := recursiveJSONNote
	if d := s.GetDescription(); d != nil {
		description = *d + "\n" + recursiveJSONNote
	}

	return &description
}

// reentersVisitedRef reports whether a property's value reaches a schema already open on the descent
// path without passing through another nested object. It looks through array items, map values
// and allOf/oneOf/anyOf wrappers, so recursion through any of those is caught before descending.
func (o GlobalSchemaOpts) reentersVisitedRef(proxy *base.SchemaProxy) bool {
	return o.reentersVisitedRefSeen(proxy, map[*base.SchemaProxy]bool{})
}

func (o GlobalSchemaOpts) reentersVisitedRefSeen(proxy *base.SchemaProxy, seen map[*base.SchemaProxy]bool) bool {
	if proxy == nil || seen[proxy] {
		return false
	}
	seen[proxy] = true

	if o.IsVisitedRef(proxy) {
		return true
	}
	if proxy.IsReference() {
		return false
	}

	s := proxy.Schema()
	if s == nil {
		return false
	}
	for _, wrapped := range [][]*base.SchemaProxy{s.AllOf, s.OneOf, s.AnyOf} {
		for _, w := range wrapped {
			if o.reentersVisitedRefSeen(w, seen) {
				return true
			}
		}
	}
	if s.Items != nil && s.Items.IsA() && o.reentersVisitedRefSeen(s.Items.A, seen) {
		return true
	}
	if s.AdditionalProperties != nil && s.AdditionalProperties.IsA() && o.reentersVisitedRefSeen(s.AdditionalProperties.A, seen) {
		return true
	}

	return false
}

// isRecursiveEdge reports whether a property re-enters a schema already open on the descent path.
// Besides a direct $ref cycle, a recursive component can be inlined once before its $ref cycle
// starts; the inline copy then has the same attribute name as the component level below it, and
// the framework generator names nested custom types after the attribute, so that repeat would
// generate a type that contains itself. The name check only applies inside a visited $ref or when
// the property leads into a $ref, so finite inline schemas that repeat a name stay typed.
func (s *OASSchema) isRecursiveEdge(name string, pProxy *base.SchemaProxy) bool {
	if s.GlobalSchemaOpts.reentersVisitedRef(pProxy) {
		return true
	}

	ancestorHash, ok := s.GlobalSchemaOpts.AncestorNames[util.TerraformIdentifier(name)]
	if !ok || (len(s.GlobalSchemaOpts.VisitedRefs) == 0 && !reachesRef(pProxy, map[*base.SchemaProxy]bool{})) {
		return false
	}
	hash, nested := nestedObjectHash(pProxy)

	return nested && hash == ancestorHash
}

// reachesRef reports whether a property's value is a $ref, looking through array items, map
// values and allOf/oneOf/anyOf wrappers.
func reachesRef(proxy *base.SchemaProxy, seen map[*base.SchemaProxy]bool) bool {
	if proxy == nil || seen[proxy] {
		return false
	}
	seen[proxy] = true
	if proxy.IsReference() {
		return true
	}

	s := proxy.Schema()
	if s == nil {
		return false
	}
	for _, wrapped := range [][]*base.SchemaProxy{s.AllOf, s.OneOf, s.AnyOf} {
		for _, w := range wrapped {
			if reachesRef(w, seen) {
				return true
			}
		}
	}
	if s.Items != nil && s.Items.IsA() && reachesRef(s.Items.A, seen) {
		return true
	}

	return s.AdditionalProperties != nil && s.AdditionalProperties.IsA() && reachesRef(s.AdditionalProperties.A, seen)
}

// nestedObjectHash returns the shape hash of the object a property renders as a nested attribute:
// the object itself, the items of an array of objects, or the values of a map of objects.
func nestedObjectHash(proxy *base.SchemaProxy) ([32]byte, bool) {
	if proxy == nil {
		return [32]byte{}, false
	}
	s := proxy.Schema()
	if s == nil {
		return [32]byte{}, false
	}
	switch {
	case s.Items != nil && s.Items.IsA():
		return nestedObjectHash(s.Items.A)
	case s.AdditionalProperties != nil && s.AdditionalProperties.IsA():
		return nestedObjectHash(s.AdditionalProperties.A)
	case s.Properties != nil && s.Properties.Len() > 0:
		return objectShapeHash(s), true
	default:
		return [32]byte{}, false
	}
}

// containsSensitive reports whether any schema reachable from proxy is a password field, so a
// collapsed JSON subtree stays sensitive when it carries one.
func containsSensitive(proxy *base.SchemaProxy) bool {
	return containsSensitiveSeen(proxy, map[*base.Schema]bool{})
}

func containsSensitiveSeen(proxy *base.SchemaProxy, seen map[*base.Schema]bool) bool {
	if proxy == nil {
		return false
	}
	s := proxy.Schema()
	if s == nil || seen[s] {
		return false
	}
	seen[s] = true

	if s.Format == util.OAS_format_password {
		return true
	}
	for _, wrapped := range [][]*base.SchemaProxy{s.AllOf, s.OneOf, s.AnyOf} {
		for _, w := range wrapped {
			if containsSensitiveSeen(w, seen) {
				return true
			}
		}
	}
	if s.Items != nil && s.Items.IsA() && containsSensitiveSeen(s.Items.A, seen) {
		return true
	}
	if s.AdditionalProperties != nil && s.AdditionalProperties.IsA() && containsSensitiveSeen(s.AdditionalProperties.A, seen) {
		return true
	}
	if s.Properties != nil {
		for pair := range orderedmap.Iterate(context.TODO(), s.Properties) {
			if containsSensitiveSeen(pair.Value(), seen) {
				return true
			}
		}
	}

	return false
}

// withAncestorName returns a copy of the object schema whose descendants see name as an
// enclosing nested attribute.
func (s *OASSchema) withAncestorName(name string) *OASSchema {
	c := *s
	c.GlobalSchemaOpts = s.GlobalSchemaOpts.WithAncestorName(util.TerraformIdentifier(name), objectShapeHash(s.Schema))
	return &c
}

// objectShapeHash identifies an object schema by its property names and, for each property, its
// $ref or type. Two attributes with the same name and shape generate the same custom type.
func objectShapeHash(s *base.Schema) [32]byte {
	var b strings.Builder
	for pair := range orderedmap.Iterate(context.TODO(), orderedmap.SortAlpha(s.Properties)) {
		b.WriteString(pair.Key())
		b.WriteByte('=')
		if p := pair.Value(); p != nil {
			if p.IsReference() {
				b.WriteString(p.GetReference())
			} else if ps := p.Schema(); ps != nil {
				b.WriteString(strings.Join(ps.Type, "|"))
				if ps.Items != nil && ps.Items.IsA() && ps.Items.A.IsReference() {
					b.WriteString("[" + ps.Items.A.GetReference() + "]")
				}
			}
		}
		b.WriteByte(';')
	}

	return sha256.Sum256([]byte(b.String()))
}
