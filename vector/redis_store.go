package vector

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisVectorStore struct {
	client    *redis.Client
	dim       int
	indexName string
}

func NewRedisVectorStore(endpoint, password string, dim int) (*RedisVectorStore, error) {
	if endpoint == "" {
		endpoint = "localhost:6379"
	}
	dbInt := 1
	dbRedis, ok := os.LookupEnv("REDIS_DB_VECTOR")
	if ok {
		dbInt, _ = strconv.Atoi(dbRedis)
	}
	rdb := redis.NewClient(&redis.Options{
		Addr:     endpoint,
		Password: password,
		DB:       dbInt,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis ping failed: %w", err)
	}

	return &RedisVectorStore{client: rdb, dim: dim, indexName: "idx:vectors"}, nil
}

func float32ToBytes(floats []float32) []byte {
	bytes := make([]byte, len(floats)*4)
	for i, f := range floats {
		bits := math.Float32bits(f)
		binary.LittleEndian.PutUint32(bytes[i*4:], bits)
	}
	return bytes
}

func bytesToFloat32(bytes []byte) []float32 {
	if len(bytes)%4 != 0 {
		return nil
	}
	floats := make([]float32, len(bytes)/4)
	for i := range floats {
		bits := binary.LittleEndian.Uint32(bytes[i*4:])
		floats[i] = math.Float32frombits(bits)
	}
	return floats
}

func (s *RedisVectorStore) StoreEmbedding(ctx context.Context, id string, embedding []float32, metadata map[string]interface{}) error {
	metaJSON, _ := json.Marshal(metadata)
	vecBytes := float32ToBytes(embedding)
	return s.client.HSet(ctx, "vec:"+id, map[string]interface{}{
		"embedding": vecBytes,
		"metadata":  metaJSON,
	}).Err()
}

func (s *RedisVectorStore) GetEmbedding(ctx context.Context, id string) ([]float32, map[string]interface{}, error) {
	res, err := s.client.HGetAll(ctx, "vec:"+id).Result()
	if err != nil {
		return nil, nil, err
	}
	if len(res) == 0 {
		return nil, nil, fmt.Errorf("embedding not found: %s", id)
	}

	vecStr := res["embedding"]
	vecBytes := []byte(vecStr)
	embedding := bytesToFloat32(vecBytes)

	var metadata map[string]interface{}
	metaStr := res["metadata"]
	if metaStr != "" {
		_ = json.Unmarshal([]byte(metaStr), &metadata)
	}
	return embedding, metadata, nil
}

func (s *RedisVectorStore) UpdateEmbedding(ctx context.Context, id string, embedding []float32) error {
	exists, err := s.client.Exists(ctx, "vec:"+id).Result()
	if err != nil {
		return err
	}
	if exists == 0 {
		return fmt.Errorf("embedding not found: %s", id)
	}
	vecBytes := float32ToBytes(embedding)
	return s.client.HSet(ctx, "vec:"+id, "embedding", vecBytes).Err()
}

func (s *RedisVectorStore) DeleteEmbedding(ctx context.Context, id string) error {
	return s.client.Del(ctx, "vec:"+id).Err()
}

func (s *RedisVectorStore) SimilaritySearch(ctx context.Context, queryEmbedding []float32, limit int, threshold float64) ([]*SimilarityResult, error) {
	return s.SimilaritySearchWithFilter(ctx, queryEmbedding, nil, limit, threshold)
}

func (s *RedisVectorStore) SimilaritySearchWithFilter(ctx context.Context, queryEmbedding []float32, filters map[string]interface{}, limit int, threshold float64) ([]*SimilarityResult, error) {
	vecBytes := float32ToBytes(queryEmbedding)

	filterStr := "*"

	query := fmt.Sprintf("%s=>[KNN %d @embedding $query_vec AS distance]", filterStr, limit)

	cmd := s.client.Do(ctx,
		"FT.SEARCH", s.indexName, query,
		"PARAMS", "2", "query_vec", vecBytes,
		"RETURN", "3", "distance", "embedding", "metadata",
		"SORTBY", "distance", "ASC",
		"DIALECT", "2",
	)

	res, err := cmd.Result()
	if err != nil {
		if strings.Contains(err.Error(), "no such index") {
			return []*SimilarityResult{}, nil
		}
		return nil, fmt.Errorf("redis ft.search failed: %w", err)
	}

	slice, ok := res.([]interface{})
	if !ok || len(slice) == 0 {
		return []*SimilarityResult{}, nil
	}

	var results []*SimilarityResult
	for i := 1; i < len(slice); i += 2 {
		key, ok := slice[i].(string)
		if !ok {
			continue
		}

		fields, ok := slice[i+1].([]interface{})
		if !ok {
			continue
		}

		var distance float64
		var metaJSON []byte
		var embBytes []byte

		for j := 0; j < len(fields); j += 2 {
			k := fields[j].(string)
			v := fields[j+1]

			switch k {
			case "distance":
				distance, _ = strconv.ParseFloat(v.(string), 64)
			case "metadata":
				metaJSON = []byte(v.(string))
			case "embedding":
				embBytes = []byte(v.(string))
			}
		}

		score := 1.0 - distance
		if score < threshold {
			continue
		}

		var metadata map[string]interface{}
		if len(metaJSON) > 0 {
			_ = json.Unmarshal(metaJSON, &metadata)
		}

		vec := bytesToFloat32(embBytes)

		id := strings.TrimPrefix(key, "vec:")
		results = append(results, &SimilarityResult{
			ID:        id,
			Score:     score,
			Metadata:  metadata,
			Embedding: vec,
			Distance:  distance,
		})
	}

	return results, nil
}

func (s *RedisVectorStore) StoreBatchEmbeddings(ctx context.Context, embeddings []*EmbeddingData) error {
	pipe := s.client.Pipeline()
	for _, emb := range embeddings {
		metaJSON, _ := json.Marshal(emb.Metadata)
		vecBytes := float32ToBytes(emb.Embedding)
		pipe.HSet(ctx, "vec:"+emb.ID, map[string]interface{}{
			"embedding": vecBytes,
			"metadata":  metaJSON,
		})
	}
	_, err := pipe.Exec(ctx)
	return err
}

func (s *RedisVectorStore) DeleteBatchEmbeddings(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	keys := make([]string, len(ids))
	for i, id := range ids {
		keys[i] = "vec:" + id
	}
	return s.client.Del(ctx, keys...).Err()
}

func (s *RedisVectorStore) CreateCollection(ctx context.Context, name string, dimension int, config *CollectionConfig) error {
	dist := "COSINE"
	if config != nil && config.DistanceMetric == "l2" {
		dist = "L2"
	}

	s.indexName = fmt.Sprintf("idx:%s", name)

	cmd := s.client.Do(ctx,
		"FT.CREATE", s.indexName, "ON", "HASH", "PREFIX", "1", "vec:",
		"SCHEMA", "embedding", "VECTOR", "FLAT", "6",
		"TYPE", "FLOAT32", "DIM", strconv.Itoa(dimension), "DISTANCE_METRIC", dist,
	)
	err := cmd.Err()
	if err != nil && strings.Contains(err.Error(), "Index already exists") {
		return nil
	}
	return err
}

func (s *RedisVectorStore) DeleteCollection(ctx context.Context, name string) error {
	idxName := fmt.Sprintf("idx:%s", name)
	return s.client.Do(ctx, "FT.DROPINDEX", idxName).Err()
}

func (s *RedisVectorStore) ListCollections(ctx context.Context) ([]string, error) {
	res, err := s.client.Do(ctx, "FT._LIST").Result()
	if err != nil {
		return nil, err
	}
	slice, ok := res.([]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected FT._LIST format")
	}

	var out []string
	for _, item := range slice {
		if s, ok := item.(string); ok {
			out = append(out, strings.TrimPrefix(s, "idx:"))
		}
	}
	return out, nil
}

func (s *RedisVectorStore) GetCollectionInfo(ctx context.Context, name string) (*CollectionInfo, error) {
	idxName := fmt.Sprintf("idx:%s", name)
	res, err := s.client.Do(ctx, "FT.INFO", idxName).Result()
	if err != nil {
		return nil, err
	}

	slice, ok := res.([]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected FT.INFO format")
	}

	var count int64
	for i := 0; i < len(slice); i += 2 {
		k, ok := slice[i].(string)
		if ok && k == "num_docs" {
			switch v := slice[i+1].(type) {
			case string:
				count, _ = strconv.ParseInt(v, 10, 64)
			case int64:
				count = v
			}
		}
	}

	return &CollectionInfo{
		Name:        name,
		Dimension:   s.dim,
		VectorCount: count,
		Status:      "ready",
	}, nil
}

func (s *RedisVectorStore) GetEmbeddingCount(ctx context.Context) (int64, error) {
	res, _ := s.GetCollectionInfo(ctx, strings.TrimPrefix(s.indexName, "idx:"))
	if res != nil {
		return res.VectorCount, nil
	}
	return 0, nil
}

func (s *RedisVectorStore) Health(ctx context.Context) error {
	return s.client.Ping(ctx).Err()
}

func (s *RedisVectorStore) Close() error {
	return s.client.Close()
}
