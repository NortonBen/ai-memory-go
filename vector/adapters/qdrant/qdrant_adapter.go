package qdrant

import (
	"context"
	"fmt"
	"time"

	"github.com/NortonBen/ai-memory-go/schema"
	"github.com/NortonBen/ai-memory-go/vector"
	"github.com/google/uuid"
	qd "github.com/qdrant/go-client/qdrant"
	"google.golang.org/grpc"
)

func init() {
	vector.RegisterStore(vector.StoreTypeQdrant, func(config *vector.VectorConfig) (vector.VectorStore, error) {
		return NewQdrantStore(config)
	})
}

// QdrantStore implements VectorStore using Qdrant.
type QdrantStore struct {
	client     *qd.Client
	conn       *grpc.ClientConn
	config     *vector.VectorConfig
	collection string
}

// NewQdrantStore creates a new Qdrant vector store.
func NewQdrantStore(config *vector.VectorConfig) (*QdrantStore, error) {
	if config.Host == "" {
		config.Host = "localhost"
	}
	if config.Port == 0 {
		config.Port = 6334
	}

	client, err := qd.NewClient(&qd.Config{
		Host: config.Host,
		Port: config.Port,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create qdrant client: %w", err)
	}
	conn := client.GetConnection()

	store := &QdrantStore{
		client:     client,
		conn:       conn,
		config:     config,
		collection: config.Collection,
	}

	// Wait for connection to be ready (health check)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := store.Health(ctx); err != nil {
		conn.Close()
		return nil, fmt.Errorf("qdrant health check failed: %w", err)
	}

	return store, nil
}

// parseID generates a deterministic UUID from a string ID to satisfy Qdrant's PointId requirements.
func parseID(id string) *qd.PointId {
	// Qdrant supports UUIDs and integers. We use a deterministic UUID based on the string ID.
	// Use uuid.NewSHA1 with a nil namespace just to get a consistent hash.
	namespace := uuid.Nil
	u := uuid.NewSHA1(namespace, []byte(id))
	return &qd.PointId{
		PointIdOptions: &qd.PointId_Uuid{
			Uuid: u.String(),
		},
	}
}

// buildPayload converts generic map[string]interface{} to Qdrant's payload format.
func buildPayload(metadata map[string]interface{}) map[string]*qd.Value {
	payload := make(map[string]*qd.Value)
	if metadata == nil {
		return payload
	}

	for k, v := range metadata {
		val, err := qd.NewValue(v)
		if err == nil {
			payload[k] = val
		}
	}
	return payload
}

// parsePayload converts Qdrant's payload format back to map[string]interface{}.
func parsePayload(payload map[string]*qd.Value) map[string]interface{} {
	metadata := make(map[string]interface{})
	if payload == nil {
		return metadata
	}

	for k, v := range payload {
		// Attempt to extract value. A very simplified extraction covering common types.
		switch v.Kind.(type) {
		case *qd.Value_StringValue:
			metadata[k] = v.GetStringValue()
		case *qd.Value_BoolValue:
			metadata[k] = v.GetBoolValue()
		case *qd.Value_IntegerValue:
			metadata[k] = v.GetIntegerValue()
		case *qd.Value_DoubleValue:
			metadata[k] = v.GetDoubleValue()
		case *qd.Value_ListValue:
			metadata[k] = extractListValue(v.GetListValue())
		case *qd.Value_StructValue:
			// Structs are recursive maps, we might need a separate function if complex metadata is expected
			// For simplicity, we skip deeply nested structs, or we'd implement parsePayload recursively.
			metadata[k] = parsePayload(v.GetStructValue().Fields)
		}
	}
	return metadata
}

func extractListValue(list *qd.ListValue) []interface{} {
	var results []interface{}
	for _, v := range list.Values {
		switch v.Kind.(type) {
		case *qd.Value_StringValue:
			results = append(results, v.GetStringValue())
		case *qd.Value_BoolValue:
			results = append(results, v.GetBoolValue())
		case *qd.Value_IntegerValue:
			results = append(results, v.GetIntegerValue())
		case *qd.Value_DoubleValue:
			results = append(results, v.GetDoubleValue())
		}
	}
	return results
}

// StoreEmbedding implements VectorStore.
func (q *QdrantStore) StoreEmbedding(ctx context.Context, id string, embedding []float32, metadata map[string]interface{}) error {
	point := &qd.PointStruct{
		Id:      parseID(id),
		Vectors: qd.NewVectors(embedding...),
		Payload: buildPayload(metadata),
	}

	_, err := q.client.Upsert(ctx, &qd.UpsertPoints{
		CollectionName: q.collection,
		Wait:           func() *bool { b := true; return &b }(),
		Points:         []*qd.PointStruct{point},
	})
	return err
}

// GetEmbedding implements VectorStore.
func (q *QdrantStore) GetEmbedding(ctx context.Context, id string) ([]float32, map[string]interface{}, error) {
	resp, err := q.client.Get(ctx, &qd.GetPoints{
		CollectionName: q.collection,
		Ids:            []*qd.PointId{parseID(id)},
		WithVectors:    &qd.WithVectorsSelector{SelectorOptions: &qd.WithVectorsSelector_Enable{Enable: true}},
		WithPayload:    &qd.WithPayloadSelector{SelectorOptions: &qd.WithPayloadSelector_Enable{Enable: true}},
	})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get point: %w", err)
	}

	if len(resp) == 0 {
		return nil, nil, fmt.Errorf("point not found: %s", id)
	}

	point := resp[0]
	var embedding []float32
	if point.Vectors != nil {
		if vec := point.Vectors.GetVector(); vec != nil {
			if dense := vec.GetDense(); dense != nil {
				embedding = dense.Data
			}
		}
	}

	metadata := parsePayload(point.Payload)
	return embedding, metadata, nil
}

// UpdateEmbedding implements VectorStore.
func (q *QdrantStore) UpdateEmbedding(ctx context.Context, id string, embedding []float32) error {
	// Qdrant allows updating just vectors with UpdateVectors
	_, err := q.client.UpdateVectors(ctx, &qd.UpdatePointVectors{
		CollectionName: q.collection,
		Wait:           func() *bool { b := true; return &b }(),
		Points: []*qd.PointVectors{
			{
				Id:      parseID(id),
				Vectors: qd.NewVectors(embedding...),
			},
		},
	})
	return err
}

// DeleteEmbedding implements VectorStore.
func (q *QdrantStore) DeleteEmbedding(ctx context.Context, id string) error {
	_, err := q.client.Delete(ctx, &qd.DeletePoints{
		CollectionName: q.collection,
		Wait:           func() *bool { b := true; return &b }(),
		Points: &qd.PointsSelector{
			PointsSelectorOneOf: &qd.PointsSelector_Points{
				Points: &qd.PointsIdsList{
					Ids: []*qd.PointId{parseID(id)},
				},
			},
		},
	})
	return err
}

// StoreBatchEmbeddings implements VectorStore.
func (q *QdrantStore) StoreBatchEmbeddings(ctx context.Context, embeddings []*vector.EmbeddingData) error {
	if len(embeddings) == 0 {
		return nil
	}

	points := make([]*qd.PointStruct, len(embeddings))
	for i, emb := range embeddings {
		metadata := emb.Metadata
		if metadata == nil {
			metadata = make(map[string]interface{})
		}
		// ensure we keep original ID in metadata in case UUID is not reversible
		metadata["_original_id"] = emb.ID

		points[i] = &qd.PointStruct{
			Id:      parseID(emb.ID),
			Vectors: qd.NewVectors(emb.Embedding...),
			Payload: buildPayload(metadata),
		}
	}

	_, err := q.client.Upsert(ctx, &qd.UpsertPoints{
		CollectionName: q.collection,
		Wait:           func() *bool { b := true; return &b }(),
		Points:         points,
	})
	return err
}

// DeleteBatchEmbeddings implements VectorStore.
func (q *QdrantStore) DeleteBatchEmbeddings(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}

	qids := make([]*qd.PointId, len(ids))
	for i, id := range ids {
		qids[i] = parseID(id)
	}

	_, err := q.client.Delete(ctx, &qd.DeletePoints{
		CollectionName: q.collection,
		Wait:           func() *bool { b := true; return &b }(),
		Points: &qd.PointsSelector{
			PointsSelectorOneOf: &qd.PointsSelector_Points{
				Points: &qd.PointsIdsList{
					Ids: qids,
				},
			},
		},
	})
	return err
}

// buildFilter creates a Qdrant filter from a generic map
func buildFilter(filters map[string]interface{}) *qd.Filter {
	if len(filters) == 0 {
		return nil
	}

	var conditions []*qd.Condition
	for k, v := range filters {
		val, err := qd.NewValue(v)
		if err == nil {
			conditions = append(conditions, &qd.Condition{
				ConditionOneOf: &qd.Condition_Field{
					Field: &qd.FieldCondition{
						Key: k,
						Match: &qd.Match{
							MatchValue: &qd.Match_Keyword{Keyword: val.GetStringValue()},
						},
					},
				},
			})
		}
	}

	if len(conditions) == 0 {
		return nil
	}

	return &qd.Filter{
		Must: conditions,
	}
}

// SimilaritySearch implements VectorStore.
func (q *QdrantStore) SimilaritySearch(ctx context.Context, queryEmbedding []float32, limit int, threshold float64) ([]*vector.SimilarityResult, error) {
	return q.SimilaritySearchWithFilter(ctx, queryEmbedding, nil, limit, threshold)
}

// SimilaritySearchWithFilter implements VectorStore.
func (q *QdrantStore) SimilaritySearchWithFilter(ctx context.Context, queryEmbedding []float32, filters map[string]interface{}, limit int, threshold float64) ([]*vector.SimilarityResult, error) {
	if limit <= 0 {
		limit = 10
	}
	searchLimit := limit
	if schema.VectorSearchHasLabelFilter(filters) {
		searchLimit = min(limit*4, 200)
	}
	req := &qd.QueryPoints{
		CollectionName: q.collection,
		Query:          qd.NewQuery(queryEmbedding...),
		Limit:          func() *uint64 { l := uint64(searchLimit); return &l }(),
		WithPayload:    &qd.WithPayloadSelector{SelectorOptions: &qd.WithPayloadSelector_Enable{Enable: true}},
		ScoreThreshold: func() *float32 { t := float32(threshold); return &t }(),
	}

	if ef := schema.FiltersForVectorSearchEngine(filters); len(ef) > 0 {
		req.Filter = buildFilter(ef)
	}

	resp, err := q.client.Query(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	out := make([]*vector.SimilarityResult, 0, min(len(resp), limit))
	for _, scored := range resp {
		metadata := parsePayload(scored.Payload)
		if len(filters) > 0 && !schema.MetadataMatchesVectorSearchFilters(metadata, filters) {
			continue
		}
		id := ""
		if scored.Id.GetUuid() != "" {
			id = scored.Id.GetUuid()
		} else {
			id = fmt.Sprintf("%d", scored.Id.GetNum())
		}
		if origID, ok := metadata["_original_id"]; ok {
			id = fmt.Sprintf("%v", origID)
		}

		out = append(out, &vector.SimilarityResult{
			ID:        id,
			Score:     float64(scored.Score),
			Metadata:  metadata,
			Embedding: nil,
			Distance:  1.0 - float64(scored.Score),
		})
		if len(out) >= limit {
			break
		}
	}

	return out, nil
}

// CreateCollection implements VectorStore.
func (q *QdrantStore) CreateCollection(ctx context.Context, name string, dimension int, config *vector.CollectionConfig) error {
	distance := qd.Distance_Cosine
	if config != nil && config.DistanceMetric != "" {
		switch config.DistanceMetric {
		case "euclidean":
			distance = qd.Distance_Euclid
		case "dot_product":
			distance = qd.Distance_Dot
		case "manhattan":
			distance = qd.Distance_Manhattan
		}
	} else if q.config.DistanceMetric != "" {
		switch q.config.DistanceMetric {
		case "euclidean":
			distance = qd.Distance_Euclid
		case "dot_product":
			distance = qd.Distance_Dot
		case "manhattan":
			distance = qd.Distance_Manhattan
		}
	}

	err := q.client.CreateCollection(ctx, &qd.CreateCollection{
		CollectionName: name,
		VectorsConfig: &qd.VectorsConfig{
			Config: &qd.VectorsConfig_Params{
				Params: &qd.VectorParams{
					Size:     uint64(dimension),
					Distance: distance,
				},
			},
		},
	})
	return err
}

// DeleteCollection implements VectorStore.
func (q *QdrantStore) DeleteCollection(ctx context.Context, name string) error {
	return q.client.DeleteCollection(ctx, name)
}

// ListCollections implements VectorStore.
func (q *QdrantStore) ListCollections(ctx context.Context) ([]string, error) {
	return q.client.ListCollections(ctx)
}

// GetCollectionInfo implements VectorStore.
func (q *QdrantStore) GetCollectionInfo(ctx context.Context, name string) (*vector.CollectionInfo, error) {
	resp, err := q.client.GetCollectionInfo(ctx, name)
	if err != nil {
		return nil, err
	}

	info := &vector.CollectionInfo{
		Name:         name,
		VectorCount:  int64(resp.GetPointsCount()),
		IndexedCount: int64(resp.GetIndexedVectorsCount()),
		Status:       resp.GetStatus().String(),
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	return info, nil
}

// GetEmbeddingCount implements VectorStore.
func (q *QdrantStore) GetEmbeddingCount(ctx context.Context) (int64, error) {
	resp, err := q.client.GetCollectionInfo(ctx, q.collection)
	if err != nil {
		return 0, err
	}

	return int64(resp.GetPointsCount()), nil
}

// Health implements VectorStore.
func (q *QdrantStore) Health(ctx context.Context) error {
	resp, err := q.client.HealthCheck(ctx)
	if err != nil {
		return err
	}
	if resp.Title == "" {
		return fmt.Errorf("qdrant cluster health check returned empty title")
	}
	return nil
}

// Close implements VectorStore.
func (q *QdrantStore) Close() error {
	if q.conn != nil {
		return q.conn.Close()
	}
	return nil
}
