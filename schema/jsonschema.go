// Package schema - JSON Schema generation for LLM integration
package schema

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
)

// JSONSchema represents a JSON Schema definition
type JSONSchema struct {
	Schema      string                 `json:"$schema,omitempty"`
	Type        string                 `json:"type"`
	Title       string                 `json:"title,omitempty"`
	Description string                 `json:"description,omitempty"`
	Properties  map[string]*JSONSchema `json:"properties,omitempty"`
	Items       *JSONSchema            `json:"items,omitempty"`
	Required    []string               `json:"required,omitempty"`
	Enum        []interface{}          `json:"enum,omitempty"`
	Format      string                 `json:"format,omitempty"`
	Minimum     *float64               `json:"minimum,omitempty"`
	Maximum     *float64               `json:"maximum,omitempty"`
}

// GenerateJSONSchema generates a JSON Schema from a Go struct
func GenerateJSONSchema(v interface{}) (*JSONSchema, error) {
	t := reflect.TypeOf(v)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	if t.Kind() != reflect.Struct {
		return nil, fmt.Errorf("expected struct, got %s", t.Kind())
	}

	schema := &JSONSchema{
		Schema:     "http://json-schema.org/draft-07/schema#",
		Type:       "object",
		Title:      t.Name(),
		Properties: make(map[string]*JSONSchema),
		Required:   []string{},
	}

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		jsonTag := field.Tag.Get("json")
		if jsonTag == "" || jsonTag == "-" {
			continue
		}

		// Parse json tag
		parts := strings.Split(jsonTag, ",")
		fieldName := parts[0]
		isOptional := false
		for _, part := range parts[1:] {
			if part == "omitempty" {
				isOptional = true
			}
		}

		// Generate field schema
		fieldSchema := generateFieldSchema(field.Type)
		
		// Add description from struct tag if available
		if desc := field.Tag.Get("description"); desc != "" {
			fieldSchema.Description = desc
		}

		schema.Properties[fieldName] = fieldSchema

		// Add to required if not optional
		if !isOptional {
			schema.Required = append(schema.Required, fieldName)
		}
	}

	return schema, nil
}

// generateFieldSchema generates schema for a field type
func generateFieldSchema(t reflect.Type) *JSONSchema {
	schema := &JSONSchema{}

	// Handle pointers
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	switch t.Kind() {
	case reflect.String:
		schema.Type = "string"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		schema.Type = "integer"
	case reflect.Float32, reflect.Float64:
		schema.Type = "number"
	case reflect.Bool:
		schema.Type = "boolean"
	case reflect.Slice, reflect.Array:
		schema.Type = "array"
		schema.Items = generateFieldSchema(t.Elem())
	case reflect.Map:
		schema.Type = "object"
	case reflect.Struct:
		// Handle time.Time specially
		if t.String() == "time.Time" {
			schema.Type = "string"
			schema.Format = "date-time"
		} else {
			schema.Type = "object"
			// For nested structs, we could recursively generate schema
			// but for simplicity, we'll just mark it as object
		}
	default:
		schema.Type = "string"
	}

	return schema
}

// ToJSON converts the JSON Schema to a JSON string
func (js *JSONSchema) ToJSON() (string, error) {
	data, err := json.MarshalIndent(js, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// NodeExtractionSchema returns a JSON Schema for Node extraction
func NodeExtractionSchema() *JSONSchema {
	return &JSONSchema{
		Schema: "http://json-schema.org/draft-07/schema#",
		Type:   "object",
		Title:  "NodeExtraction",
		Properties: map[string]*JSONSchema{
			"nodes": {
				Type: "array",
				Items: &JSONSchema{
					Type: "object",
					Properties: map[string]*JSONSchema{
						"type": {
							Type: "string",
							Enum: []interface{}{
								"Concept", "Word", "UserPreference", "GrammarRule",
								"Entity", "Document", "Session", "User",
							},
						},
						"name": {
							Type:        "string",
							Description: "The name or label of the node",
						},
						"properties": {
							Type:        "object",
							Description: "Additional properties for the node",
						},
					},
					Required: []string{"type", "name"},
				},
			},
		},
		Required: []string{"nodes"},
	}
}

// EdgeExtractionSchema returns a JSON Schema for Edge extraction
func EdgeExtractionSchema() *JSONSchema {
	return &JSONSchema{
		Schema: "http://json-schema.org/draft-07/schema#",
		Type:   "object",
		Title:  "EdgeExtraction",
		Properties: map[string]*JSONSchema{
			"edges": {
				Type: "array",
				Items: &JSONSchema{
					Type: "object",
					Properties: map[string]*JSONSchema{
						"from": {
							Type:        "string",
							Description: "Source node name or ID",
						},
						"to": {
							Type:        "string",
							Description: "Target node name or ID",
						},
						"type": {
							Type: "string",
							Enum: []interface{}{
								"RELATED_TO", "FAILED_AT", "SYNONYM", "STRUGGLES_WITH",
								"CONTAINS", "REFERENCED_BY", "SIMILAR_TO", "PART_OF",
								"CREATED_BY", "USED_IN",
							},
						},
						"weight": {
							Type:    "number",
							Minimum: floatPtr(0.0),
							Maximum: floatPtr(1.0),
						},
					},
					Required: []string{"from", "to", "type"},
				},
			},
		},
		Required: []string{"edges"},
	}
}

// EntityRelationshipExtractionSchema returns a combined schema for entity and relationship extraction
func EntityRelationshipExtractionSchema() *JSONSchema {
	return &JSONSchema{
		Schema: "http://json-schema.org/draft-07/schema#",
		Type:   "object",
		Title:  "EntityRelationshipExtraction",
		Properties: map[string]*JSONSchema{
			"entities": {
				Type: "array",
				Items: &JSONSchema{
					Type: "object",
					Properties: map[string]*JSONSchema{
						"name": {
							Type:        "string",
							Description: "Entity name",
						},
						"type": {
							Type:        "string",
							Description: "Entity type (Concept, Word, etc.)",
						},
						"properties": {
							Type:        "object",
							Description: "Additional entity properties",
						},
					},
					Required: []string{"name", "type"},
				},
			},
			"relationships": {
				Type: "array",
				Items: &JSONSchema{
					Type: "object",
					Properties: map[string]*JSONSchema{
						"from": {
							Type:        "string",
							Description: "Source entity name",
						},
						"to": {
							Type:        "string",
							Description: "Target entity name",
						},
						"type": {
							Type:        "string",
							Description: "Relationship type",
						},
						"weight": {
							Type:    "number",
							Minimum: floatPtr(0.0),
							Maximum: floatPtr(1.0),
						},
					},
					Required: []string{"from", "to", "type"},
				},
			},
		},
		Required: []string{"entities", "relationships"},
	}
}

// Helper function to create float pointer
func floatPtr(f float64) *float64 {
	return &f
}

// GenerateSchemaForType generates a JSON Schema for common types
func GenerateSchemaForType(typeName string) (*JSONSchema, error) {
	switch typeName {
	case "Node":
		return GenerateJSONSchema(Node{})
	case "Edge":
		return GenerateJSONSchema(Edge{})
	case "DataPoint":
		return GenerateJSONSchema(DataPoint{})
	case "ProcessedQuery":
		return GenerateJSONSchema(ProcessedQuery{})
	case "SearchResult":
		return GenerateJSONSchema(SearchResult{})
	case "NodeExtraction":
		return NodeExtractionSchema(), nil
	case "EdgeExtraction":
		return EdgeExtractionSchema(), nil
	case "EntityRelationshipExtraction":
		return EntityRelationshipExtractionSchema(), nil
	default:
		return nil, fmt.Errorf("unknown type: %s", typeName)
	}
}
