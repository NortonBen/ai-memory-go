package qdrant

import (
	"context"
	"reflect"
	"testing"
	"time"

	qd "github.com/qdrant/go-client/qdrant"
)

func TestQdrantParseID(t *testing.T) {
	id1 := "test-id-1"
	id2 := "test-id-2"

	qId1 := parseID(id1)
	qId2 := parseID(id2)

	if qId1.GetUuid() == "" {
		t.Errorf("Expected parseID to return UUID wrapped ID, got: %v", qId1)
	}

	if qId1.GetUuid() == qId2.GetUuid() {
		t.Errorf("Expected different string IDs to map to different UUIDs")
	}

	// Should be deterministic
	qId1_again := parseID(id1)
	if qId1.GetUuid() != qId1_again.GetUuid() {
		t.Errorf("Expected deterministic UUID generation, but got different: %v != %v", qId1.GetUuid(), qId1_again.GetUuid())
	}
}

func TestQdrantBuildFilter(t *testing.T) {
	filters := map[string]interface{}{
		"category": "docs",
	}

	filter := buildFilter(filters)

	if filter == nil {
		t.Fatalf("Expected filter to be built, got nil")
	}

	if len(filter.Must) != 1 {
		t.Fatalf("Expected 1 Must condition, got %d", len(filter.Must))
	}

	cond := filter.Must[0]
	fieldCond := cond.GetField()
	if fieldCond == nil {
		t.Fatalf("Expected field condition")
	}

	if fieldCond.Key != "category" {
		t.Errorf("Expected key 'category', got '%s'", fieldCond.Key)
	}

	matchCond := fieldCond.Match.GetMatchValue()
	switch m := matchCond.(type) {
	case *qd.Match_Keyword:
		if m.Keyword != "docs" {
			t.Errorf("Expected keyword 'docs', got '%s'", m.Keyword)
		}
	default:
		t.Errorf("Expected Match_Keyword condition type, got %T", m)
	}
}

func TestBuildPayloadAndParsePayload_Roundtrip(t *testing.T) {
	meta := map[string]interface{}{
		"s": "x",
		"b": true,
		"i": int64(3),
		"f": float64(1.25),
		"l": []interface{}{"a", int64(2), float64(3.5), true},
		"m": map[string]interface{}{"k": "v"},
	}

	p := buildPayload(meta)
	out := parsePayload(p)

	// parsePayload returns numeric types as int64/float64 depending on qd.Value storage.
	if out["s"] != "x" || out["b"] != true {
		t.Fatalf("unexpected primitives: %#v", out)
	}
	if _, ok := out["i"].(int64); !ok {
		t.Fatalf("expected int64 for i, got %T", out["i"])
	}
	if _, ok := out["f"].(float64); !ok {
		t.Fatalf("expected float64 for f, got %T", out["f"])
	}
	if _, ok := out["l"].([]interface{}); !ok {
		t.Fatalf("expected list for l, got %T", out["l"])
	}
	if m, ok := out["m"].(map[string]interface{}); !ok || m["k"] != "v" {
		t.Fatalf("expected nested map, got %#v", out["m"])
	}
}

func TestExtractListValue_HandlesTypes(t *testing.T) {
	v1, err := qd.NewValue("a")
	if err != nil {
		t.Fatalf("NewValue err: %v", err)
	}
	v2, err := qd.NewValue(int64(2))
	if err != nil {
		t.Fatalf("NewValue err: %v", err)
	}
	v3, err := qd.NewValue(float64(3.5))
	if err != nil {
		t.Fatalf("NewValue err: %v", err)
	}
	v4, err := qd.NewValue(true)
	if err != nil {
		t.Fatalf("NewValue err: %v", err)
	}
	lv := &qd.ListValue{
		Values: []*qd.Value{
			v1,
			v2,
			v3,
			v4,
		},
	}
	got := extractListValue(lv)
	want := []interface{}{"a", int64(2), float64(3.5), true}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}

func TestBuildFilter_EmptyOrUnsupported(t *testing.T) {
	if buildFilter(nil) != nil {
		t.Fatalf("expected nil for nil map")
	}
	// qd.NewValue on function should error; expect nil filter due to no conditions.
	if buildFilter(map[string]interface{}{"bad": func() {}}) != nil {
		t.Fatalf("expected nil when no conditions are built")
	}
}

func TestQdrantStore_CloseNilConn(t *testing.T) {
	s := &QdrantStore{conn: nil}
	if err := s.Close(); err != nil {
		t.Fatalf("Close with nil conn should return nil, got %v", err)
	}
}

func TestQdrantStore_HealthErrorAndCloseConn(t *testing.T) {
	// Use an unreachable local port to exercise Health() error path without external service.
	client, err := qd.NewClient(&qd.Config{Host: "127.0.0.1", Port: 65535})
	if err != nil {
		t.Fatalf("NewClient failed: %v", err)
	}
	conn := client.GetConnection()
	s := &QdrantStore{client: client, conn: conn, collection: "x"}

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	if err := s.Health(ctx); err == nil {
		t.Fatalf("expected Health error for unreachable endpoint")
	}
	if err := s.Close(); err != nil {
		t.Fatalf("expected Close to close grpc conn, got %v", err)
	}
}
