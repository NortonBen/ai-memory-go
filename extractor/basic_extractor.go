// Package extractor - Entity and relationship extraction implementation
package extractor

import (
	"context"
	"fmt"
	"strings"

	"github.com/NortonBen/ai-memory-go/schema"
)

// BasicExtractor implements LLMExtractor interface
type BasicExtractor struct {
	provider LLMProvider
	config   *ExtractionConfig
}

// NewBasicExtractor creates a new basic extractor
func NewBasicExtractor(provider LLMProvider, config *ExtractionConfig) *BasicExtractor {
	if config == nil {
		config = DefaultExtractionConfig()
	}
	return &BasicExtractor{
		provider: provider,
		config:   config,
	}
}

// ExtractEntities extracts entities from text content
func (be *BasicExtractor) ExtractEntities(ctx context.Context, text string) ([]schema.Node, error) {
	// Generate prompt based on domain
	prompt := be.generateEntityPrompt(text)

	// Use JSON schema mode if enabled
	if be.config.UseJSONSchema {
		return be.extractEntitiesWithSchema(ctx, prompt)
	}

	// Fallback to text-based extraction
	return be.extractEntitiesFromText(ctx, prompt)
}

// extractEntitiesWithSchema extracts entities using JSON schema
func (be *BasicExtractor) extractEntitiesWithSchema(ctx context.Context, prompt string) ([]schema.Node, error) {
	// Define extraction result structure
	type ExtractionResult struct {
		Entities []struct {
			Name       string      `json:"name"`
			Type       string      `json:"type"`
			Properties interface{} `json:"properties,omitempty"`
		} `json:"entities"`
	}

	var result ExtractionResult
	_, err := be.provider.GenerateStructuredOutput(ctx, prompt, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to extract entities: %w", err)
	}

	// Convert to schema.Node
	nodes := make([]schema.Node, 0, len(result.Entities))
	for _, entity := range result.Entities {
		nodeType := schema.NodeType(entity.Type)

		properties := make(map[string]interface{})

		// Robust property parsing to handle different LLM outputs gracefully
		if entity.Properties != nil {
			switch v := entity.Properties.(type) {
			case map[string]interface{}:
				properties = v
			case []interface{}:
				var strVals []string
				for _, item := range v {
					if str, ok := item.(string); ok {
						strVals = append(strVals, str)
					}
				}
				if len(strVals) > 0 {
					properties["description"] = strings.Join(strVals, ", ")
				} else {
					for i, item := range v {
						properties[fmt.Sprintf("item_%d", i)] = item
					}
				}
			case string:
				properties["description"] = v
			}
		}

		properties["name"] = entity.Name

		node := schema.NewNode(nodeType, properties)
		nodes = append(nodes, *node)
	}

	return nodes, nil
}

// extractEntitiesFromText extracts entities from text response
func (be *BasicExtractor) extractEntitiesFromText(ctx context.Context, prompt string) ([]schema.Node, error) {
	response, err := be.provider.GenerateCompletion(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("failed to get completion: %w", err)
	}

	// Parse response (simplified - in production use more robust parsing)
	nodes := make([]schema.Node, 0)
	lines := strings.Split(response, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Simple parsing: "EntityName (Type)"
		parts := strings.Split(line, "(")
		if len(parts) >= 2 {
			name := strings.TrimSpace(parts[0])
			typeStr := strings.TrimSuffix(strings.TrimSpace(parts[1]), ")")

			properties := map[string]interface{}{
				"name": name,
			}

			node := schema.NewNode(schema.NodeType(typeStr), properties)
			nodes = append(nodes, *node)
		}
	}

	return nodes, nil
}

// ExtractRelationships detects relationships between entities
func (be *BasicExtractor) ExtractRelationships(ctx context.Context, text string, entities []schema.Node) ([]schema.Edge, error) {
	// Generate prompt with entities context
	prompt := be.generateRelationshipPrompt(text, entities)

	// Use JSON schema mode if enabled
	if be.config.UseJSONSchema {
		return be.extractRelationshipsWithSchema(ctx, prompt, entities)
	}

	// Fallback to text-based extraction
	return be.extractRelationshipsFromText(ctx, prompt, entities)
}

// extractRelationshipsWithSchema extracts relationships using JSON schema
func (be *BasicExtractor) extractRelationshipsWithSchema(ctx context.Context, prompt string, entities []schema.Node) ([]schema.Edge, error) {
	// Define extraction result structure
	type RelationshipResult struct {
		Relationships []struct {
			From       string      `json:"from"`
			To         string      `json:"to"`
			Type       string      `json:"type"`
			Weight     float64     `json:"weight,omitempty"`
			Properties interface{} `json:"properties,omitempty"`
		} `json:"relationships"`
	}

	var result RelationshipResult
	_, err := be.provider.GenerateStructuredOutput(ctx, prompt, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to extract relationships: %w", err)
	}

	//fmt.Println("Relationships extracted:", result.Relationships)

	// Create entity name to ID mapping
	entityMap := make(map[string]string)
	for _, entity := range entities {
		if name, ok := entity.Properties["name"].(string); ok {
			entityMap[name] = entity.ID
		}
	}

	// Convert to schema.Edge
	edges := make([]schema.Edge, 0, len(result.Relationships))
	for _, rel := range result.Relationships {
		fromID, fromExists := entityMap[rel.From]
		toID, toExists := entityMap[rel.To]

		if !fromExists || !toExists {
			continue // Skip if entities not found
		}

		weight := rel.Weight
		if weight == 0 {
			weight = 1.0
		}

		edge := schema.NewEdge(fromID, toID, schema.EdgeType(rel.Type), weight)

		// Parse properties robustly if present
		if rel.Properties != nil {
			switch v := rel.Properties.(type) {
			case map[string]interface{}:
				edge.Properties = v
			case []interface{}:
				edge.Properties = make(map[string]interface{})
				var strVals []string
				for _, item := range v {
					if str, ok := item.(string); ok {
						strVals = append(strVals, str)
					}
				}
				if len(strVals) > 0 {
					edge.Properties["description"] = strings.Join(strVals, ", ")
				}
			case string:
				edge.Properties = map[string]interface{}{"description": v}
			}
		}

		edges = append(edges, *edge)
	}

	return edges, nil
}

// extractRelationshipsFromText extracts relationships from text response
func (be *BasicExtractor) extractRelationshipsFromText(ctx context.Context, prompt string, entities []schema.Node) ([]schema.Edge, error) {
	response, err := be.provider.GenerateCompletion(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("failed to get completion: %w", err)
	}

	// Create entity name to ID mapping
	entityMap := make(map[string]string)
	for _, entity := range entities {
		if name, ok := entity.Properties["name"].(string); ok {
			entityMap[name] = entity.ID
		}
	}

	// Parse response (simplified)
	edges := make([]schema.Edge, 0)
	lines := strings.Split(response, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Simple parsing: "Entity1 -> Entity2 (RelationType)"
		parts := strings.Split(line, "->")
		if len(parts) >= 2 {
			from := strings.TrimSpace(parts[0])
			rest := strings.TrimSpace(parts[1])

			toParts := strings.Split(rest, "(")
			if len(toParts) >= 2 {
				to := strings.TrimSpace(toParts[0])
				relType := strings.TrimSuffix(strings.TrimSpace(toParts[1]), ")")

				fromID, fromExists := entityMap[from]
				toID, toExists := entityMap[to]

				if fromExists && toExists {
					edge := schema.NewEdge(fromID, toID, schema.EdgeType(relType), 1.0)
					edges = append(edges, *edge)
				}
			}
		}
	}

	return edges, nil
}

// ExtractWithSchema extracts structured data using a Go struct schema
func (be *BasicExtractor) ExtractWithSchema(ctx context.Context, text string, schemaStruct interface{}) (interface{}, error) {
	prompt := fmt.Sprintf("Extract structured information from the following text:\n\n%s", text)
	return be.provider.GenerateStructuredOutput(ctx, prompt, schemaStruct)
}

// SetProvider sets the LLM provider
func (be *BasicExtractor) SetProvider(provider LLMProvider) {
	be.provider = provider
}

// GetProvider returns the current LLM provider
func (be *BasicExtractor) GetProvider() LLMProvider {
	return be.provider
}

// generateEntityPrompt generates a prompt for entity extraction
func (be *BasicExtractor) generateEntityPrompt(text string) string {
	if be.config.EntityPrompt != "" {
		return strings.ReplaceAll(be.config.EntityPrompt, "{text}", text)
	}

	// Default generic prompt
	return fmt.Sprintf(`Extract key entities from the following text.
Identify important concepts, people, places, organizations, and other significant entities.

Text: %s

Return a JSON object with an "entities" array. Each entity should have:
- name: the entity name
- type: the entity type (Concept, Entity, etc.)
- properties: a JSON object containing key-value pairs of additional details (optional)`, text)
}

// generateRelationshipPrompt generates a prompt for relationship extraction
func (be *BasicExtractor) generateRelationshipPrompt(text string, entities []schema.Node) string {
	if be.config.RelationshipPrompt != "" {
		entityNames := make([]string, len(entities))
		for i, entity := range entities {
			if name, ok := entity.Properties["name"].(string); ok {
				entityNames[i] = name
			}
		}
		prompt := strings.ReplaceAll(be.config.RelationshipPrompt, "{text}", text)
		prompt = strings.ReplaceAll(prompt, "{entities}", strings.Join(entityNames, ", "))
		return prompt
	}

	// Build entity list
	entityNames := make([]string, len(entities))
	for i, entity := range entities {
		if name, ok := entity.Properties["name"].(string); ok {
			entityNames[i] = name
		}
	}

	// Default generic prompt
	return fmt.Sprintf(`Given these entities: %s

Analyze the following text and identify relationships between the entities:
%s

Return a JSON object with a "relationships" array. Each relationship should have:
- from: source entity name
- to: target entity name
- type: relationship type (RELATED_TO, SIMILAR_TO, PART_OF, etc.)
- weight: relationship strength (0.0 to 1.0, optional)`, strings.Join(entityNames, ", "), text)
}

// ExtractBridgingRelationship creates a direct relationship summarizing an LLM's multi-hop reasoning sequence
func (be *BasicExtractor) ExtractBridgingRelationship(ctx context.Context, question string, answer string) (*schema.Edge, error) {
	prompt := fmt.Sprintf(`Given the following question and answer, extract the core underlying relationship that directly answers the question.
If the answer relies on multi-hop reasoning, summarize it into a single direct relationship between the most important entities.

Question: %s
Answer: %s

Return a JSON object with a "relationship" object. It should have:
- from: source entity name (e.g., a person, company, etc.)
- to: target entity name
- type: relationship type (e.g., HAS_CEO_BEST_FRIEND, RELATED_TO)
- weight: 1.0 (or a float between 0.0 and 1.0)`, question, answer)

	type ExtractionResult struct {
		Relationship struct {
			From   string  `json:"from"`
			To     string  `json:"to"`
			Type   string  `json:"type"`
			Weight float64 `json:"weight,omitempty"`
		} `json:"relationship"`
	}

	var result ExtractionResult
	_, err := be.provider.GenerateStructuredOutput(ctx, prompt, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to extract bridging relationship: %w", err)
	}

	rel := result.Relationship
	if rel.From == "" || rel.To == "" || rel.Type == "" {
		return nil, fmt.Errorf("invalid bridging relationship extracted: missing fields")
	}

	weight := rel.Weight
	if weight == 0 {
		weight = 1.0
	}

	edge := schema.NewEdge(rel.From, rel.To, schema.EdgeType(rel.Type), weight)
	edge.Properties = map[string]interface{}{
		"is_bridging": true,
		"question":    question,
		"answer":      answer,
	}

	return edge, nil
}
