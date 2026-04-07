package engine

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/NortonBen/ai-memory-go/extractor"
	"github.com/NortonBen/ai-memory-go/graph"
	"github.com/NortonBen/ai-memory-go/schema"
	"github.com/NortonBen/ai-memory-go/storage"
	"github.com/NortonBen/ai-memory-go/vector"
)

// MemifyTask is responsible for pushing the JSON graph drafts into the global explicit GraphStore and VectorStore.
// It also executes Consistency Reasoning to avoid duplicated facts.
type MemifyTask struct {
	DataPoint            *schema.DataPoint
	ConsistencyThreshold float32
}

func (t *MemifyTask) Execute(ctx context.Context, ext extractor.LLMExtractor, emb vector.EmbeddingProvider, store storage.Storage, graphStore graph.GraphStore, vectorStore vector.VectorStore, wp *WorkerPool) error {
	if t.DataPoint.ProcessingStatus != schema.StatusCognified {
		return fmt.Errorf("cannot memify DataPoint in status %s (requires cognified)", t.DataPoint.ProcessingStatus)
	}

	var extraEdges []schema.Edge

	// 1. Process Extracted Nodes against GraphStore
	if graphStore != nil {
		for _, node := range t.DataPoint.Nodes {
			// Assign source_id so graph traversal knows which DataPoint created this node
			if node.Properties == nil {
				node.Properties = make(map[string]interface{})
			}
			node.Properties["source_id"] = t.DataPoint.ID
			if _, ok := node.Properties["timestamp"]; !ok {
				node.Properties["timestamp"] = time.Now().UTC().Format(time.RFC3339Nano)
			}
			if _, ok := node.Properties["confidence"]; !ok {
				node.Properties["confidence"] = 0.8
			}

			// --- Consistency Reasoning ---
			if t.ConsistencyThreshold > 0 && vectorStore != nil {
				// Embed node context for similarity comparison
				nodeContent := fmt.Sprintf("Entity: %s\nType: %s\nProperties: %v", node.ID, node.Type, node.Properties)
				nodeEmb, err := emb.GenerateEmbedding(ctx, nodeContent)
				if err == nil {
					// Search for similar entities
					filters := map[string]interface{}{"is_entity": true}
					vecResults, err := vectorStore.SimilaritySearchWithFilter(ctx, nodeEmb, filters, 1, float64(t.ConsistencyThreshold))
					if err == nil && len(vecResults) > 0 {
						matchedVec := vecResults[0]
						existingNodeID, ok := matchedVec.Metadata["entity_id"].(string)
						if ok && existingNodeID != node.ID {
							existingNode, err := graphStore.GetNode(ctx, existingNodeID)
							if err == nil && existingNode != nil {
								resolution, err := ext.CompareEntities(ctx, *existingNode, *node)
								if err == nil && resolution != nil && resolution.Action != schema.ResolutionKeepSeparate {
									log.Printf("Consistency Reasoning [%s vs %s] -> %s", node.ID, existingNode.ID, resolution.Action)
									if resolution.Action == schema.ResolutionUpdate {
										// Merge properties into existing node
										if existingNode.Properties == nil {
											existingNode.Properties = make(map[string]interface{})
										}
										for k, v := range node.Properties {
											existingNode.Properties[k] = v
										}
										_ = graphStore.UpdateNode(ctx, existingNode)
										node.ID = existingNode.ID
										node.Properties = existingNode.Properties
										node.Type = existingNode.Type
									} else if resolution.Action == schema.ResolutionIgnore {
										// Map ID and properties to existing so relationships attach correctly but node isn't overwritten
										node.ID = existingNode.ID
										node.Properties = existingNode.Properties
										node.Type = existingNode.Type
									} else if resolution.Action == schema.ResolutionContradict {
										// Create CONTRADICTS edge from new node to old node
										extraEdges = append(extraEdges, schema.Edge{
											ID:         fmt.Sprintf("%s-contradicts-%s", node.ID, existingNode.ID),
											From:       node.ID,
											To:         existingNode.ID,
											Type:       schema.EdgeTypeContradicts,
											Properties: map[string]interface{}{"reason": resolution.Reason},
											Weight:     1.0,
										})
									}
								}
							}
						}
					}
					// Store current entity's embedding for future comparisons
					_ = vectorStore.StoreEmbedding(ctx, "entity-"+node.ID, nodeEmb, map[string]interface{}{
						"is_entity":   true,
						"entity_id":   node.ID,
						"entity_type": string(node.Type),
						"source_id":   t.DataPoint.ID,
					})
				}
			}

			err := graphStore.StoreNode(ctx, node)
			if err != nil {
				log.Printf("failed to store node %s in graphStore: %v", node.ID, err)
			}
		}
	}

	// 2. Process Relationships
	if graphStore != nil && (len(t.DataPoint.Edges) > 0 || len(extraEdges) > 0) {
		allEdges := append(t.DataPoint.Edges, extraEdges...)
		for i := range allEdges {
			if allEdges[i].Properties == nil {
				allEdges[i].Properties = make(map[string]interface{})
			}
			allEdges[i].Properties["source_id"] = t.DataPoint.ID
			if _, ok := allEdges[i].Properties["timestamp"]; !ok {
				allEdges[i].Properties["timestamp"] = time.Now().UTC().Format(time.RFC3339Nano)
			}
			if _, ok := allEdges[i].Properties["confidence"]; !ok {
				allEdges[i].Properties["confidence"] = 0.8
			}
			if allEdges[i].SessionID == "" {
				allEdges[i].SessionID = t.DataPoint.SessionID
			}
			err := graphStore.CreateRelationship(ctx, &allEdges[i])
			if err != nil {
				log.Printf("failed to store edge in graphStore: %v", err)
			}
		}
	}

	// 3. Map edges to Relationships in DataPoint
	var relationships []schema.Relationship
	for _, edge := range t.DataPoint.Edges {
		relationships = append(relationships, edge.ToRelationship())
	}
	t.DataPoint.Relationships = relationships

	// 4. Update Status
	t.DataPoint.ProcessingStatus = schema.StatusCompleted
	t.DataPoint.UpdatedAt = time.Now()

	if err := store.UpdateDataPoint(ctx, t.DataPoint); err != nil {
		return t.fail(ctx, store, fmt.Errorf("failed to update DataPoint: %w", err))
	}

	// If this is a chunk DataPoint, synchronize parent input DataPoint status.
	if t.DataPoint.Metadata != nil {
		if parentID, ok := t.DataPoint.Metadata["parent_id"].(string); ok && parentID != "" {
			parent, err := store.GetDataPoint(ctx, parentID)
			if err == nil && parent != nil {
				if err := syncParentStatusFromChildren(ctx, store, parent); err != nil {
					log.Printf("failed to sync parent completion for %s: %v", parentID, err)
				}
			}
		}
	}

	return nil
}

func (t *MemifyTask) fail(ctx context.Context, store storage.Storage, err error) error {
	t.DataPoint.ProcessingStatus = schema.StatusFailed
	t.DataPoint.ErrorMessage = err.Error()
	t.DataPoint.UpdatedAt = time.Now()
	_ = store.UpdateDataPoint(ctx, t.DataPoint)
	return err
}
