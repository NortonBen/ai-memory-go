package schema

import (
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type jsSample struct {
	Name      string            `json:"name" description:"sample name"`
	Age       int               `json:"age"`
	Score     float64           `json:"score,omitempty"`
	Active    bool              `json:"active"`
	CreatedAt time.Time         `json:"created_at"`
	Tags      []string          `json:"tags,omitempty"`
	Props     map[string]string `json:"props,omitempty"`
	Ignored   string            `json:"-"`
}

func TestGenerateJSONSchema_BasicAndErrors(t *testing.T) {
	s, err := GenerateJSONSchema(jsSample{})
	require.NoError(t, err)
	require.Equal(t, "object", s.Type)
	require.Equal(t, "jsSample", s.Title)
	require.Contains(t, s.Properties, "name")
	require.Equal(t, "string", s.Properties["name"].Type)
	require.Equal(t, "sample name", s.Properties["name"].Description)
	require.Equal(t, "date-time", s.Properties["created_at"].Format)
	require.NotContains(t, s.Properties, "Ignored")
	require.Contains(t, s.Required, "name")
	require.Contains(t, s.Required, "age")
	require.NotContains(t, s.Required, "score")

	_, err = GenerateJSONSchema(123)
	require.Error(t, err)
}

func TestGenerateFieldSchema_Kinds(t *testing.T) {
	require.Equal(t, "string", generateFieldSchema(typeOf[string]()).Type)
	require.Equal(t, "integer", generateFieldSchema(typeOf[int64]()).Type)
	require.Equal(t, "number", generateFieldSchema(typeOf[float32]()).Type)
	require.Equal(t, "boolean", generateFieldSchema(typeOf[bool]()).Type)

	arr := generateFieldSchema(typeOf[[]int]())
	require.Equal(t, "array", arr.Type)
	require.NotNil(t, arr.Items)
	require.Equal(t, "integer", arr.Items.Type)

	require.Equal(t, "object", generateFieldSchema(typeOf[map[string]int]()).Type)

	ptr := generateFieldSchema(typeOf[*time.Time]())
	require.Equal(t, "string", ptr.Type)
	require.Equal(t, "date-time", ptr.Format)
}

func TestJSONSchemaHelpers_AndGenerateSchemaForType(t *testing.T) {
	node := NodeExtractionSchema()
	require.Equal(t, "NodeExtraction", node.Title)
	require.Contains(t, node.Required, "nodes")

	edge := EdgeExtractionSchema()
	require.Equal(t, "EdgeExtraction", edge.Title)
	require.Contains(t, edge.Required, "edges")
	require.NotNil(t, edge.Properties["edges"].Items.Properties["weight"].Minimum)
	require.NotNil(t, edge.Properties["edges"].Items.Properties["weight"].Maximum)

	combined := EntityRelationshipExtractionSchema()
	require.Equal(t, "EntityRelationshipExtraction", combined.Title)
	require.Contains(t, combined.Required, "entities")
	require.Contains(t, combined.Required, "relationships")

	js, err := combined.ToJSON()
	require.NoError(t, err)
	require.Contains(t, js, "EntityRelationshipExtraction")

	_, err = GenerateSchemaForType("Node")
	require.NoError(t, err)
	_, err = GenerateSchemaForType("Edge")
	require.NoError(t, err)
	_, err = GenerateSchemaForType("DataPoint")
	require.NoError(t, err)
	_, err = GenerateSchemaForType("ProcessedQuery")
	require.NoError(t, err)
	_, err = GenerateSchemaForType("SearchResult")
	require.NoError(t, err)
	_, err = GenerateSchemaForType("NodeExtraction")
	require.NoError(t, err)
	_, err = GenerateSchemaForType("EdgeExtraction")
	require.NoError(t, err)
	_, err = GenerateSchemaForType("EntityRelationshipExtraction")
	require.NoError(t, err)

	_, err = GenerateSchemaForType("UnknownX")
	require.Error(t, err)
}

func typeOf[T any]() reflect.Type {
	var z T
	return reflect.TypeOf(z)
}

