// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package oas_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/hashicorp/terraform-plugin-codegen-spec/resource"
	"github.com/hashicorp/terraform-plugin-codegen-spec/schema"
	"github.com/pb33f/libopenapi"

	"github.com/starburstdata/terraform-plugin-codegen-openapi/internal/mapper/attrmapper"
	"github.com/starburstdata/terraform-plugin-codegen-openapi/internal/mapper/oas"
)

// Mirrors the Galaxy live table shapes: a component that $refs itself (IngestColumn), and an
// attribute whose items are an inlined copy of that component, so the copy's nestedColumns
// $refs the component (FilterAndProjectTransformProperties.columns).
const recursiveOAS = `
openapi: 3.0.3
info:
  title: recursive
  version: "1"
paths: {}
components:
  schemas:
    Column:
      type: object
      required: [name]
      properties:
        name:
          type: string
          description: Column name
        nestedColumns:
          type: array
          description: Nested columns
          items:
            $ref: '#/components/schemas/Column'
    Table:
      type: object
      properties:
        columns:
          type: array
          items:
            $ref: '#/components/schemas/Column'
    InlineTable:
      type: object
      properties:
        nestedColumns:
          type: array
          items:
            type: object
            required: [name]
            properties:
              name:
                type: string
                description: Column name
              nestedColumns:
                type: array
                description: Nested columns
                items:
                  $ref: '#/components/schemas/Column'
`

func buildRecursiveTable(t *testing.T, name string) attrmapper.ResourceAttributes {
	t.Helper()

	doc, err := libopenapi.NewDocument([]byte(recursiveOAS))
	if err != nil {
		t.Fatalf("parse: %s", err)
	}
	model, errs := doc.BuildV3Model()
	if len(errs) > 0 {
		t.Fatalf("model: %v", errs)
	}
	table, ok := model.Model.Components.Schemas.Get(name)
	if !ok {
		t.Fatalf("%s schema missing", name)
	}

	s, schemaErr := oas.BuildSchema(table, oas.SchemaOpts{}, oas.GlobalSchemaOpts{})
	if schemaErr != nil {
		t.Fatalf("build schema: %s", schemaErr)
	}
	attributes, schemaErr := s.BuildResourceAttributes()
	if schemaErr != nil {
		t.Fatalf("build attributes: %s", schemaErr)
	}

	return attributes
}

func columnLevel(nested attrmapper.ResourceAttribute) attrmapper.ResourceAttributes {
	return attrmapper.ResourceAttributes{
		&attrmapper.ResourceStringAttribute{
			Name: "name",
			StringAttribute: resource.StringAttribute{
				ComputedOptionalRequired: schema.Required,
				Description:              pointer("Column name"),
			},
		},
		nested,
	}
}

func listNested(name string, attributes attrmapper.ResourceAttributes) attrmapper.ResourceAttribute {
	return &attrmapper.ResourceListNestedAttribute{
		Name:                name,
		NestedObject:        attrmapper.ResourceNestedAttributeObject{Attributes: attributes},
		ListNestedAttribute: resource.ListNestedAttribute{ComputedOptionalRequired: schema.ComputedOptional},
	}
}

var nestedColumnsJSON = &attrmapper.ResourceStringAttribute{
	Name: "nestedColumns",
	StringAttribute: resource.StringAttribute{
		ComputedOptionalRequired: schema.ComputedOptional,
		Description:              pointer("Nested columns\nEncoded as a JSON string because this type is recursive."),
	},
}

// A component that $refs itself keeps one typed level; the self-reference is a JSON string.
func TestBuildResourceAttributes_RefCycleBecomesJSONString(t *testing.T) {
	t.Parallel()

	want := attrmapper.ResourceAttributes{
		listNested("columns", columnLevel(nestedColumnsJSON)),
	}

	if diff := cmp.Diff(buildRecursiveTable(t, "Table"), want); diff != "" {
		t.Errorf("unexpected difference: %s", diff)
	}
}

// When the component is first reached through an inlined copy under the same attribute name,
// the component level repeats the enclosing name and shape. The framework generator would name
// both levels NestedColumnsValue, a type that contains itself, so the repeat is a JSON string.
func TestBuildResourceAttributes_RepeatedNestedNameBecomesJSONString(t *testing.T) {
	t.Parallel()

	want := attrmapper.ResourceAttributes{
		listNested("nestedColumns", columnLevel(nestedColumnsJSON)),
	}

	if diff := cmp.Diff(buildRecursiveTable(t, "InlineTable"), want); diff != "" {
		t.Errorf("unexpected difference: %s", diff)
	}
}

const reviewOAS = `
openapi: 3.0.3
info:
  title: review
  version: "1"
paths: {}
components:
  schemas:
    MapColumn:
      type: object
      properties:
        children:
          type: object
          description: Children by name
          additionalProperties:
            $ref: '#/components/schemas/MapColumn'
    MatrixColumn:
      type: object
      properties:
        matrix:
          type: array
          description: Grid of columns
          items:
            type: array
            items:
              $ref: '#/components/schemas/MatrixColumn'
    SecretColumn:
      type: object
      properties:
        password:
          type: string
          format: password
        nestedColumns:
          type: array
          description: Nested columns
          items:
            $ref: '#/components/schemas/SecretColumn'
    Finite:
      type: object
      properties:
        node:
          type: object
          properties:
            node:
              type: object
              properties:
                node:
                  type: object
                  properties:
                    leaf:
                      type: string
`

func buildReviewSchema(t *testing.T, name string) attrmapper.ResourceAttributes {
	t.Helper()

	doc, err := libopenapi.NewDocument([]byte(reviewOAS))
	if err != nil {
		t.Fatalf("parse: %s", err)
	}
	model, errs := doc.BuildV3Model()
	if len(errs) > 0 {
		t.Fatalf("model: %v", errs)
	}
	proxy, ok := model.Model.Components.Schemas.Get(name)
	if !ok {
		t.Fatalf("%s schema missing", name)
	}

	s, schemaErr := oas.BuildSchema(proxy, oas.SchemaOpts{}, oas.GlobalSchemaOpts{})
	if schemaErr != nil {
		t.Fatalf("build schema: %s", schemaErr)
	}
	attributes, schemaErr := s.BuildResourceAttributes()
	if schemaErr != nil {
		t.Fatalf("build attributes: %s", schemaErr)
	}

	return attributes
}

func jsonStringAttr(name, description string, sensitive *bool) attrmapper.ResourceAttribute {
	return &attrmapper.ResourceStringAttribute{
		Name: name,
		StringAttribute: resource.StringAttribute{
			ComputedOptionalRequired: schema.ComputedOptional,
			Description:              pointer(description + "\nEncoded as a JSON string because this type is recursive."),
			Sensitive:                sensitive,
		},
	}
}

// Recursion through map values used to overflow the stack. The component keeps one typed map
// level; the recursive map inside it is a JSON string.
func TestBuildResourceAttributes_RecursiveMapBecomesJSONString(t *testing.T) {
	t.Parallel()

	want := attrmapper.ResourceAttributes{
		&attrmapper.ResourceMapNestedAttribute{
			Name: "children",
			NestedObject: attrmapper.ResourceNestedAttributeObject{
				Attributes: attrmapper.ResourceAttributes{jsonStringAttr("children", "Children by name", nil)},
			},
			MapNestedAttribute: resource.MapNestedAttribute{
				ComputedOptionalRequired: schema.ComputedOptional,
				Description:              pointer("Children by name"),
			},
		},
	}

	if diff := cmp.Diff(buildReviewSchema(t, "MapColumn"), want); diff != "" {
		t.Errorf("unexpected difference: %s", diff)
	}
}

// Recursion through an array of arrays goes through the element-type builders and used to
// overflow the stack. The outer list of lists stays typed; the recursive property inside each
// element is JSON text.
func TestBuildResourceAttributes_RecursiveNestedArrayBecomesJSONString(t *testing.T) {
	t.Parallel()

	jsonElem := schema.ElementType{String: &schema.StringType{}}
	want := attrmapper.ResourceAttributes{
		&attrmapper.ResourceListAttribute{
			Name: "matrix",
			ListAttribute: resource.ListAttribute{
				ComputedOptionalRequired: schema.ComputedOptional,
				Description:              pointer("Grid of columns"),
				ElementType: schema.ElementType{
					List: &schema.ListType{ElementType: schema.ElementType{
						Object: &schema.ObjectType{AttributeTypes: []schema.ObjectAttributeType{
							{Name: "matrix", String: jsonElem.String},
						}},
					}},
				},
			},
		},
	}

	if diff := cmp.Diff(buildReviewSchema(t, "MatrixColumn"), want); diff != "" {
		t.Errorf("unexpected difference: %s", diff)
	}
}

// The JSON string carries deeper password fields, so it must be sensitive too.
func TestBuildResourceAttributes_RecursiveJSONStringKeepsSensitivity(t *testing.T) {
	t.Parallel()

	want := attrmapper.ResourceAttributes{
		&attrmapper.ResourceListNestedAttribute{
			Name: "nestedColumns",
			NestedObject: attrmapper.ResourceNestedAttributeObject{
				Attributes: attrmapper.ResourceAttributes{
					jsonStringAttr("nestedColumns", "Nested columns", pointer(true)),
					&attrmapper.ResourceStringAttribute{
						Name: "password",
						StringAttribute: resource.StringAttribute{
							ComputedOptionalRequired: schema.ComputedOptional,
							Sensitive:                pointer(true),
						},
					},
				},
			},
			ListNestedAttribute: resource.ListNestedAttribute{
				ComputedOptionalRequired: schema.ComputedOptional,
				Description:              pointer("Nested columns"),
			},
		},
		&attrmapper.ResourceStringAttribute{
			Name: "password",
			StringAttribute: resource.StringAttribute{
				ComputedOptionalRequired: schema.ComputedOptional,
				Sensitive:                pointer(true),
			},
		},
	}

	if diff := cmp.Diff(buildReviewSchema(t, "SecretColumn"), want); diff != "" {
		t.Errorf("unexpected difference: %s", diff)
	}
}

// A finite schema that repeats an attribute name must stay fully typed.
func TestBuildResourceAttributes_FiniteRepeatedNameStaysTyped(t *testing.T) {
	t.Parallel()

	single := func(name string, attributes attrmapper.ResourceAttributes) attrmapper.ResourceAttribute {
		return &attrmapper.ResourceSingleNestedAttribute{
			Name:                  name,
			Attributes:            attributes,
			SingleNestedAttribute: resource.SingleNestedAttribute{ComputedOptionalRequired: schema.ComputedOptional},
		}
	}
	want := attrmapper.ResourceAttributes{
		single("node", attrmapper.ResourceAttributes{
			single("node", attrmapper.ResourceAttributes{
				single("node", attrmapper.ResourceAttributes{
					&attrmapper.ResourceStringAttribute{
						Name:            "leaf",
						StringAttribute: resource.StringAttribute{ComputedOptionalRequired: schema.ComputedOptional},
					},
				}),
			}),
		}),
	}

	if diff := cmp.Diff(buildReviewSchema(t, "Finite"), want); diff != "" {
		t.Errorf("unexpected difference: %s", diff)
	}
}
