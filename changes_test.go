package main

import (
	"fmt"
	openrpc "github.com/vmkteam/meta-schema/v2"
	"testing"
)

func TestNewDiff(t *testing.T) {
	diff, err := NewDiff("testdata/openrpc_old.json", "testdata/openrpc_new.json", Options{ShowMeta: true})

	if err != nil {
		t.Fatalf("new diff error: %s", err)
	}

	changesMap := map[CriticalityLevel][]Change{
		Breaking:    {},
		Dangerous:   {},
		NonBreaking: {},
	}

	for _, change := range diff.Changes {
		changesMap[change.Criticality] = append(changesMap[change.Criticality], change)
	}

	if diff.Criticality != Breaking {
		t.Fatalf("diff.Criticality = %v, wanted %v", diff.Criticality, Breaking)
	}

	if len(diff.Changes) != 18 {
		t.Fatalf("len(diff.Changes) = %v, wanted %v", len(diff.Changes), 17)
	}

	if len(changesMap[Breaking]) != 7 {
		t.Fatalf("len %s changes = %v, wanted %v", Breaking, len(changesMap[Breaking]), 7)
	}

	if len(changesMap[Dangerous]) != 1 {
		t.Fatalf("len %s changes = %v, wanted %v", Dangerous, len(changesMap[Dangerous]), 7)
	}

	if len(changesMap[NonBreaking]) != 10 {
		t.Fatalf("len %s changes = %v, wanted %v", NonBreaking, len(changesMap[NonBreaking]), 7)
	}

	fmt.Println(diff.String())
}

func Test_detectRequiredInput(t *testing.T) {
	doc := &openrpc.OpenrpcDocument{
		Methods: []openrpc.MethodOrReference{{
			MethodObject: &openrpc.MethodObject{
				Name: "method",
				Params: []openrpc.ContentDescriptorOrReference{
					{
						ContentDescriptorObject: &openrpc.ContentDescriptorObject{
							Name: "node",
							Schema: &openrpc.JSONSchema{
								JSONSchemaObject: &openrpc.JSONSchemaObject{
									Ref: "#/components/schemas/Node",
								},
							},
							Required: false,
						},
					},
					{
						ContentDescriptorObject: &openrpc.ContentDescriptorObject{
							Name: "parent",
							Schema: &openrpc.JSONSchema{
								JSONSchemaObject: &openrpc.JSONSchemaObject{
									Ref: "#/components/schemas/Parent",
								},
							},
							Required: false,
						},
					},
				},
			}},
		},
		Components: &openrpc.Components{
			Schemas: &openrpc.SchemaMap{
				{
					JSONSchemaObject: &openrpc.JSONSchemaObject{
						Id:       "Node",
						Title:    "Node",
						Required: []string{},
						Properties: &openrpc.SchemaMap{
							{
								JSONSchemaObject: &openrpc.JSONSchemaObject{
									Id:    "node",
									Title: "node",
									Ref:   "#/components/schemas/Node",
								},
							},
							{
								JSONSchemaObject: &openrpc.JSONSchemaObject{
									Id:    "parent",
									Title: "parent",
									Ref:   "#/components/schemas/Parent",
								},
							},
						},
					},
				},
				{
					JSONSchemaObject: &openrpc.JSONSchemaObject{
						Id:    "Parent",
						Title: "Parent",
						Required: []string{
							"node",
						},
						Properties: &openrpc.SchemaMap{
							{
								JSONSchemaObject: &openrpc.JSONSchemaObject{
									Id:    "node",
									Title: "node",
									Ref:   "#/components/schemas/Node",
								},
							},
						},
					},
				},
			},
		},
	}

	isInput := detectRequiredInput("Node", doc, []string{}, 0)
	if isInput == false {
		t.Fatalf("isInput must be true")
	}
}

func Test_matchPath(t *testing.T) {
	tests := []struct {
		name    string
		path    []string
		pattern string
		want    bool
	}{
		{
			name:    "should match pattern 1",
			path:    []string{"methods", "check.AddRequiredParam", "params", "param2"},
			pattern: "methods.*.params",
			want:    true,
		},
		{
			name:    "should match pattern 2",
			path:    []string{"methods", "check.AddRequiredParam", "params", "param2"},
			pattern: "methods",
			want:    true,
		},
		{
			name:    "should not match pattern 1",
			path:    []string{"methods", "check.AddRequiredParam", "params", "param2"},
			pattern: "methods.methods",
			want:    false,
		},
		{
			name:    "should not match pattern 2",
			path:    []string{"methods", "check.AddRequiredParam", "params", "param2"},
			pattern: "methods.*.*.*.*.*",
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := matchPath(tt.path, tt.pattern); got != tt.want {
				t.Errorf("matchPath() = %v, want %v", got, tt.want)
			}
		})
	}
}
