package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/NortonBen/ai-memory-go/schema"
	"github.com/NortonBen/ai-memory-go/storage"
	"github.com/NortonBen/ai-memory-go/vector"
)

func (e *defaultMemoryEngine) useFourTierSearch(q *schema.SearchQuery) bool {
	if q.FourTier != nil && q.FourTier.Enabled != nil {
		return *q.FourTier.Enabled
	}
	return e.fourTier.Enabled
}

func fourTierOptBool(ptr *bool, defaultOn bool) bool {
	if ptr != nil {
		return *ptr
	}
	return defaultOn
}

func (e *defaultMemoryEngine) retrieveContextFourTier(ctx context.Context, query *schema.SearchQuery) (*schema.SearchResults, error) {
	opts := query.FourTier
	if opts == nil {
		opts = &schema.FourTierSearchOptions{}
	}

	stats := &schema.FourTierSearchStats{}

	historyContext := e.getHistoryBuffer(ctx, query.SessionID, 10)
	extractionInput := query.Text
	if historyContext != "" {
		extractionInput = fmt.Sprintf("HISTORY:\n%s\n\nCURRENT USER MESSAGE:\n%s", historyContext, query.Text)
	}

	var emb []float32
	var extractedEntities []*schema.Node
	var embeddingErr error

	var step1WG sync.WaitGroup
	step1WG.Add(2)
	go func() {
		defer step1WG.Done()
		emb, embeddingErr = e.embedder.GenerateEmbedding(ctx, query.Text)
	}()
	go func() {
		defer step1WG.Done()
		if query.Analysis != nil && len(query.Analysis.Subjects) > 0 {
			for _, subject := range query.Analysis.Subjects {
				extractedEntities = append(extractedEntities, &schema.Node{
					ID:   subject,
					Type: schema.NodeTypeEntity,
					Properties: map[string]interface{}{
						"name": subject,
					},
				})
			}
			return
		}
		if e.extractor != nil {
			extractedNodes, err := e.extractor.ExtractEntities(ctx, extractionInput)
			if err == nil {
				for i := range extractedNodes {
					extractedEntities = append(extractedEntities, &extractedNodes[i])
				}
			}
		}
	}()
	step1WG.Wait()
	if embeddingErr != nil {
		return nil, fmt.Errorf("failed to generate embedding for query: %w", embeddingErr)
	}

	branchEmbedding := emb
	if query.Analysis != nil && len(query.Analysis.SearchKeywords) > 0 {
		searchText := strings.Join(query.Analysis.SearchKeywords, " ")
		refinedEmb, err := e.embedder.GenerateEmbedding(ctx, searchText)
		if err == nil {
			branchEmbedding = refinedEmb
		}
	}

	threshold := query.SimilarityThreshold
	if threshold == 0 {
		threshold = 0.45
	}

	tierLimit := query.Limit
	if tierLimit <= 0 {
		tierLimit = 20
	}
	legacyLimit := tierLimit * 4
	if legacyLimit < 40 {
		legacyLimit = 40
	}

	searchCore := fourTierOptBool(opts.SearchCore, true)
	searchGeneral := fourTierOptBool(opts.SearchGeneral, true)
	searchData := fourTierOptBool(opts.SearchData, true)
	includeStorage := opts.IncludeStorageTier
	weakTh := opts.WeakScoreThreshold
	if weakTh <= 0 {
		weakTh = 0.35
	}

	type tierVecResult struct {
		tier   string
		hits   []*vector.SimilarityResult
		err    error
		legacy bool
	}

	var coreDPs []*schema.DataPoint
	var tierOut [5]tierVecResult
	var wg sync.WaitGroup

	if searchCore {
		wg.Add(1)
		go func() {
			defer wg.Done()
			coreDPs = e.loadCoreTierDataPoints(ctx, query)
			stats.CoreHitCount = len(coreDPs)
		}()
	}

	if searchGeneral {
		wg.Add(1)
		go func() {
			defer wg.Done()
			hits, err := e.vectorStore.SimilaritySearchWithFilter(ctx, branchEmbedding, schema.VectorSearchTierFilter(schema.MemoryTierGeneral), tierLimit, threshold)
			tierOut[0] = tierVecResult{tier: schema.MemoryTierGeneral, hits: hits, err: err}
			stats.GeneralHitCount = len(hits)
		}()
	}

	if searchData {
		wg.Add(1)
		go func() {
			defer wg.Done()
			hits, err := e.vectorStore.SimilaritySearchWithFilter(ctx, branchEmbedding, schema.VectorSearchTierFilter(schema.MemoryTierData), tierLimit, threshold)
			tierOut[1] = tierVecResult{tier: schema.MemoryTierData, hits: hits, err: err}
			stats.DataHitCount = len(hits)
		}()
	}

	if includeStorage {
		wg.Add(1)
		go func() {
			defer wg.Done()
			hits, err := e.vectorStore.SimilaritySearchWithFilter(ctx, branchEmbedding, schema.VectorSearchTierFilter(schema.MemoryTierStorage), tierLimit, threshold)
			tierOut[2] = tierVecResult{tier: schema.MemoryTierStorage, hits: hits, err: err}
			stats.StorageHitCount = len(hits)
			stats.StorageSearched = true
		}()
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		hits, err := e.vectorStore.SimilaritySearch(ctx, branchEmbedding, legacyLimit, threshold)
		tierOut[3] = tierVecResult{tier: "", hits: hits, err: err, legacy: true}
		stats.LegacyHitCount = len(hits)
	}()

	wg.Wait()

	for _, tr := range tierOut {
		if tr.err != nil && !tr.legacy {
			// Bỏ qua lỗi từng nhánh có filter; legacy vẫn có thể cứu dữ liệu cũ.
		}
	}

	merged := make(map[string]struct {
		score float64
		tier  string
		vr    *vector.SimilarityResult
	})

	absorb := func(tr tierVecResult) {
		if tr.err != nil {
			return
		}
		for _, vr := range tr.hits {
			if vr == nil {
				continue
			}
			tier := tr.tier
			if tr.legacy {
				tier = schema.EffectiveMemoryTierFromVectorMetadata(vr.Metadata)
				if tier == schema.MemoryTierStorage && !includeStorage {
					continue
				}
			}
			sid := vectorResultSourceID(vr)
			w := vr.Score * tierVectorMultiplier(tier)
			cur, ok := merged[sid]
			if !ok || w > cur.score {
				merged[sid] = struct {
					score float64
					tier  string
					vr    *vector.SimilarityResult
				}{score: w, tier: tier, vr: vr}
			}
		}
	}

	absorb(tierOut[0])
	absorb(tierOut[1])
	absorb(tierOut[2])
	absorb(tierOut[3])

	vectorScores := make(map[string]float64)
	vectorAnchorNodeIDs := make(map[string]bool)
	for sid, m := range merged {
		vectorScores[sid] = m.score
		if m.vr != nil {
			linkedNodes, err := e.graphStore.FindNodesByProperty(ctx, "source_id", sid)
			if err == nil {
				for _, ln := range linkedNodes {
					vectorAnchorNodeIDs[ln.ID] = true
				}
			}
		}
	}

	for _, dp := range coreDPs {
		if dp == nil {
			continue
		}
		boost := coreTextRelevanceScore(query.Text, dp.Content)
		prev := vectorScores[dp.ID]
		mergedScore := boost * tierVectorMultiplier(schema.MemoryTierCore)
		if mergedScore > prev {
			vectorScores[dp.ID] = mergedScore
		}
		if linkedNodes, err := e.graphStore.FindNodesByProperty(ctx, "source_id", dp.ID); err == nil {
			for _, ln := range linkedNodes {
				vectorAnchorNodeIDs[ln.ID] = true
			}
		}
	}

	maxVec := 0.0
	for _, s := range vectorScores {
		if s > maxVec {
			maxVec = s
		}
	}

	if opts.AutoStorageIfWeak && !includeStorage && maxVec < weakTh {
		stats.StorageLazyRun = true
		stats.StorageSearched = true
		storHits, err := e.vectorStore.SimilaritySearchWithFilter(ctx, branchEmbedding, schema.VectorSearchTierFilter(schema.MemoryTierStorage), tierLimit*2, threshold)
		if err == nil {
			for _, vr := range storHits {
				if vr == nil {
					continue
				}
				sid := vectorResultSourceID(vr)
				w := vr.Score * tierVectorMultiplier(schema.MemoryTierStorage)
				cur, ok := merged[sid]
				if !ok || w > cur.score {
					merged[sid] = struct {
						score float64
						tier  string
						vr    *vector.SimilarityResult
					}{score: w, tier: schema.MemoryTierStorage, vr: vr}
				}
				vectorScores[sid] = merged[sid].score
				if linkedNodes, err2 := e.graphStore.FindNodesByProperty(ctx, "source_id", sid); err2 == nil {
					for _, ln := range linkedNodes {
						vectorAnchorNodeIDs[ln.ID] = true
					}
				}
			}
			stats.StorageHitCount = len(storHits)
		}
	}

	return e.hybridRankFromVectorScores(ctx, query, extractedEntities, vectorScores, vectorAnchorNodeIDs, nil, stats)
}

func (e *defaultMemoryEngine) loadCoreTierDataPoints(ctx context.Context, query *schema.SearchQuery) []*schema.DataPoint {
	if e.store == nil {
		return nil
	}
	q := &storage.DataPointQuery{
		SessionID: query.SessionID,
		Limit:     300,
	}
	if query.SessionID == "" {
		q.SessionID = "default"
	}
	dps, err := e.store.QueryDataPoints(ctx, q)
	if err != nil {
		return nil
	}
	var out []*schema.DataPoint
	for _, dp := range dps {
		if dp == nil {
			continue
		}
		if schema.MemoryTierFromDataPoint(dp) != schema.MemoryTierCore {
			continue
		}
		out = append(out, dp)
	}
	return out
}

func vectorResultSourceID(vr *vector.SimilarityResult) string {
	if vr == nil {
		return ""
	}
	// Embedding con có ID kiểu "{datapoint-chunk-NNN}-chunk-M" nhưng DataPoint trong DB chỉ là "{id}-chunk-NNN".
	// Metadata source_id luôn trỏ đúng bản ghi relational để truy hồi nhãn / nội dung.
	if vr.Metadata != nil {
		if sid, ok := vr.Metadata["source_id"].(string); ok {
			if t := strings.TrimSpace(sid); t != "" {
				return t
			}
		}
	}
	return vr.ID
}

func tierVectorMultiplier(tier string) float64 {
	switch tier {
	case schema.MemoryTierCore:
		return 1.12
	case schema.MemoryTierGeneral:
		return 1.0
	case schema.MemoryTierData:
		return 0.96
	case schema.MemoryTierStorage:
		return 0.90
	default:
		return 1.0
	}
}

func coreTextRelevanceScore(queryText, content string) float64 {
	q := strings.ToLower(strings.TrimSpace(queryText))
	c := strings.ToLower(content)
	if q == "" {
		return 0.55
	}
	if strings.Contains(c, q) {
		return 0.95
	}
	parts := strings.Fields(q)
	hits := 0
	for _, p := range parts {
		if len(p) < 2 {
			continue
		}
		if strings.Contains(c, p) {
			hits++
		}
	}
	if len(parts) == 0 {
		return 0.55
	}
	return 0.5 + 0.4*float64(hits)/float64(len(parts))
}

// hybridRankFromVectorScores chạy graph anchors, fusion và ParsedContext (dùng chung logic legacy).
// entityAnchorNodeIDs có thể nil (sẽ resolve từ extractedEntities).
func (e *defaultMemoryEngine) hybridRankFromVectorScores(ctx context.Context, query *schema.SearchQuery, extractedEntities []*schema.Node, vectorScores map[string]float64, vectorAnchorNodeIDs map[string]bool, entityAnchorNodeIDs map[string]bool, stats *schema.FourTierSearchStats) (*schema.SearchResults, error) {
	type itemScore struct {
		dp          *schema.DataPoint
		vectorScore float64
		graphScore  float64
	}
	scores := make(map[string]*itemScore)

	trackDataPoint := func(id string) *itemScore {
		if _, exists := scores[id]; !exists {
			dp, err := e.store.GetDataPoint(ctx, id)
			if err == nil && dp != nil {
				scores[id] = &itemScore{dp: dp}
			}
		}
		return scores[id]
	}

	if entityAnchorNodeIDs == nil {
		entityAnchorNodeIDs = make(map[string]bool)
		for _, entity := range extractedEntities {
			name, _ := entity.Properties["name"].(string)
			if name == "" {
				name = entity.ID
			}
			nodes, err := e.graphStore.FindNodesByEntity(ctx, name, entity.Type)
			if err == nil {
				for _, n := range nodes {
					entityAnchorNodeIDs[n.ID] = true
				}
			}
		}
	}

	anchorNodeIDs := make(map[string]bool, len(vectorAnchorNodeIDs)+len(entityAnchorNodeIDs))
	for id := range vectorAnchorNodeIDs {
		anchorNodeIDs[id] = true
	}
	for id := range entityAnchorNodeIDs {
		anchorNodeIDs[id] = true
	}

	for sourceID, score := range vectorScores {
		if item := trackDataPoint(sourceID); item != nil {
			item.vectorScore = score
		}
	}

	hopDepth := query.HopDepth
	if hopDepth <= 0 {
		hopDepth = 2
	}

	graphNodesContext := make(map[string]*schema.Node)
	for nodeID := range anchorNodeIDs {
		neighbors, err := e.graphStore.TraverseGraph(ctx, nodeID, hopDepth, nil)
		if err == nil {
			for _, neighbor := range neighbors {
				graphNodesContext[neighbor.ID] = neighbor
				if sourceID, ok := neighbor.Properties["source_id"].(string); ok {
					if item := trackDataPoint(sourceID); item != nil {
						item.graphScore += 0.5
					}
				}
			}
		}
	}

	var rankedItems []*itemScore
	for _, item := range scores {
		rankedItems = append(rankedItems, item)
	}

	for _, item := range rankedItems {
		temporalScore := 0.0
		if time.Since(item.dp.CreatedAt).Hours() < 24*7 {
			temporalScore = 1.0
		}
		finalScore := (item.vectorScore * 0.40) + (item.graphScore * 0.30) + (temporalScore * 0.20)
		item.vectorScore = finalScore
	}

	sort.Slice(rankedItems, func(i, j int) bool {
		return rankedItems[i].vectorScore > rankedItems[j].vectorScore
	})

	limit := query.Limit
	if limit > 0 && len(rankedItems) > limit {
		rankedItems = rankedItems[:limit]
	}

	results := &schema.SearchResults{
		Results:       make([]*schema.SearchResult, 0, len(rankedItems)),
		Total:         len(scores),
		FourTierStats: stats,
	}

	for _, item := range rankedItems {
		results.Results = append(results.Results, schema.NewSearchResult(item.dp, item.vectorScore, query.Mode))
	}

	var contextBuilder strings.Builder
	if stats != nil {
		contextBuilder.WriteString("--- [4-TIER] MERGED MEMORIES ---\n")
		for i, item := range rankedItems {
			contextBuilder.WriteString(fmt.Sprintf("[%d] (Score: %.2f, tier=%s):\n%s\n", i+1, item.vectorScore, schema.MemoryTierFromDataPoint(item.dp), item.dp.Content))
		}
	} else {
		contextBuilder.WriteString("--- MEMORIES FROM VECTOR SEARCH ---\n")
		for i, item := range rankedItems {
			contextBuilder.WriteString(fmt.Sprintf("[%d] (Score: %.2f):\n%s\n", i+1, item.vectorScore, item.dp.Content))
		}
	}

	if len(graphNodesContext) > 0 {
		contextBuilder.WriteString("\n--- KNOWLEDGE GRAPH (ENTITIES & RELATIONSHIPS) ---\n")
		i := 1
		for _, n := range graphNodesContext {
			props, _ := json.Marshal(n.Properties)
			contextBuilder.WriteString(fmt.Sprintf("- [NodeType: %s] %s: %s\n", n.Type, n.ID, string(props)))
			i++
			if i > 20 {
				break
			}
		}
	}
	results.ParsedContext = contextBuilder.String()

	return results, nil
}
