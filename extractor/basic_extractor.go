// Package extractor - Entity and relationship extraction implementation
package extractor

import (
	"context"
	"fmt"
	"math"
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
			Confidence float64     `json:"confidence,omitempty"`
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
		nodeType, knownNodeType := normalizeNodeType(entity.Type)
		if be.config.StrictMode && !knownNodeType {
			return nil, fmt.Errorf("unknown node type in strict mode: %q", entity.Type)
		}

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

		// Normalize entity name. Some models may return empty name with only type/properties.
		entityName := strings.TrimSpace(entity.Name)
		if entityName == "" {
			entityName = strings.TrimSpace(extractName(properties))
		}
		if entityName == "" {
			entityName = strings.TrimSpace(entity.Type)
		}
		if entityName == "" {
			entityName = "Entity"
		}
		properties["name"] = entityName
		if c, ok := normalizeConfidence(entity.Confidence); ok {
			properties["confidence"] = c
		}

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

			nodeType, knownNodeType := normalizeNodeType(typeStr)
			if be.config.StrictMode && !knownNodeType {
				return nil, fmt.Errorf("unknown node type in strict mode: %q", typeStr)
			}
			node := schema.NewNode(nodeType, properties)
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
			Confidence float64     `json:"confidence,omitempty"`
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

		edgeType, knownEdgeType := normalizeEdgeType(rel.Type)
		if be.config.StrictMode && !knownEdgeType {
			return nil, fmt.Errorf("unknown edge type in strict mode: %q", rel.Type)
		}
		edge := schema.NewEdge(fromID, toID, edgeType, weight)

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
		if c, ok := normalizeConfidence(rel.Confidence); ok {
			if edge.Properties == nil {
				edge.Properties = make(map[string]interface{})
			}
			edge.Properties["confidence"] = c
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
					edgeType, knownEdgeType := normalizeEdgeType(relType)
					if be.config.StrictMode && !knownEdgeType {
						return nil, fmt.Errorf("unknown edge type in strict mode: %q", relType)
					}
					edge := schema.NewEdge(fromID, toID, edgeType, 1.0)
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
	return fmt.Sprintf(`Extract key entities from the following text block (which may include chat history).
Use this ontology for "type":
- Person, Org, Project, Task, Event, Document, Concept, Entity, Session, User

CRITICAL INSTRUCTIONS:
1. FOCUS primarily on new information in the "Current User Message" section if history is present.
2. DO NOT extract meta-concepts about the system itself (e.g., "mảnh ký ức", "lịch sử", "thông tin", "hệ thống", "quá trình").
3. DO NOT extract temporal words as entities (e.g., "bây giờ", "hôm qua").
4. If the text says "I am [Name]", extract type 'Person' with name '[Name]'.
5. Keep names canonical and concise (e.g., "OpenAI", not "the OpenAI company in this message").
6. Add optional properties when available: role, title, aliases, source_span.

Text for extraction:
%s

Return a JSON object with an "entities" array. Each entity should have:
- name: the entity name (canonical form)
- type: MUST be one of the ontology values listed above
- properties: a JSON object containing additional details discovered in the text.`, text)
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
- type: relationship type from ontology:
  MENTIONS, RELATED_TO, WORKS_ON, DEPENDS_ON, DISCUSSED_IN,
  CONTAINS, PART_OF, USED_IN, REFERENCED_BY, SIMILAR_TO, UPDATES, CONTRADICTS
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

// CompareEntities compares a new entity against an existing similar entity and determines consistency action deterministically without an LLM
func (be *BasicExtractor) CompareEntities(ctx context.Context, existing schema.Node, newEntity schema.Node) (*schema.ConsistencyResult, error) {
	// Rule 1: Type mismatch -> Different entities (KEEP_SEPARATE)
	if !strings.EqualFold(string(existing.Type), string(newEntity.Type)) {
		return &schema.ConsistencyResult{
			Action: schema.ResolutionKeepSeparate,
			Reason: fmt.Sprintf("Entity types differ: '%s' vs '%s'", existing.Type, newEntity.Type),
		}, nil
	}

	// Rule 2: Compare Name equivalence
	existName := extractName(existing.Properties)
	newName := extractName(newEntity.Properties)

	// If both have names but they differ significantly, they are different entities
	if existName != "" && newName != "" && !strings.EqualFold(strings.TrimSpace(existName), strings.TrimSpace(newName)) {
		return &schema.ConsistencyResult{
			Action: schema.ResolutionKeepSeparate,
			Reason: fmt.Sprintf("Entity names differ: '%s' vs '%s'", existName, newName),
		}, nil
	}

	// Rule 3: Types match, and names match (or are empty). Now deeply compare properties.
	hasNewInfo := false
	mergedData := make(map[string]interface{})

	// Copy existing properties
	for k, v := range existing.Properties {
		mergedData[k] = v
	}

	// Compare with new properties
	for k, newV := range newEntity.Properties {
		if existV, exists := existing.Properties[k]; exists {
			// Compare string representation to avoid complex type casting issues
			if fmt.Sprintf("%v", existV) != fmt.Sprintf("%v", newV) {
				// We consider this a CONTRADICTION if an existing property value differs.
				return &schema.ConsistencyResult{
					Action: schema.ResolutionContradict,
					Reason: fmt.Sprintf("Property '%s' conflicts. Existing: '%v', New: '%v'", k, existV, newV),
				}, nil
			}
		} else {
			hasNewInfo = true
			mergedData[k] = newV
		}
	}

	if hasNewInfo {
		return &schema.ConsistencyResult{
			Action:     schema.ResolutionUpdate,
			Reason:     "New entity contains additional properties",
			MergedData: mergedData,
		}, nil
	}

	// No conflicts, no new info -> IGNORE
	return &schema.ConsistencyResult{
		Action: schema.ResolutionIgnore,
		Reason: "New entity provides no new information",
	}, nil
}

// extractName is a helper to find a name-like property for comparison
func extractName(props map[string]interface{}) string {
	if props == nil {
		return ""
	}
	if name, ok := props["name"].(string); ok {
		return name
	}
	if title, ok := props["title"].(string); ok {
		return title
	}
	if id, ok := props["id"].(string); ok {
		return id
	}

	// Handle arrays natively created by JSON Unmarshal (e.g. from LLM extraction output)
	if names, ok := props["name"].([]interface{}); ok && len(names) > 0 {
		if str, ok := names[0].(string); ok {
			return str
		}
	}
	return ""
}

// ExtractRequestIntent detects if the user's message contains an intent to store information, answer a question, or delete data.
func (be *BasicExtractor) ExtractRequestIntent(ctx context.Context, text string) (*schema.RequestIntent, error) {
	prompt := fmt.Sprintf(`Analyze the following chat context and determine the user's intent for the LAST message.
Also, identify relationships between participants or mentioned entities:
- BOT <-> USER (the current user talking)
- USER <-> USER (relationships between people)
- USER <-> ENTITY (ownership, association with objects/animals/places)

Context (History + Current Message):
%s

RULES:
- 'is_delete' is true ONLY for explicit commands to delete or forget information (e.g., "Xóa thông tin...", "Quên đi..."). 
- Factual corrections (e.g., "không phải X mà là Y", "nó không phải 1 tuổi") are STATEMENT intents, NOT delete intents.
- Use the provided HISTORY as a primary source of context to resolve all references, pronouns, and implied subjects in the current message.
- For possession (e.g., "nhà tôi", "của tôi", "my"), resolve to the 'User' entity.
- Capture state-change facts (e.g., "đã mất" -> relationship: ("Vàng", "is", "dead") or ("Vàng", "status", "passed away")).
- Output ONLY the JSON object.
  - 'from'/'to' should be specific entity names derived from history (e.g., "Ben", "Bot", "Con chó Đen"). Do NOT use pronouns in 'from'/'to' if they can be resolved.
  - Connect concepts across sentences (e.g., if one turn mentions "X" and the next turn says "Y is part of it", identify the relationship using "X").
  - 'type' should be a standard relationship type (e.g., "FRIEND_OF", "OWNS", "LIVES_IN", "BORN_IN", "HAS_AGE", "HAS_STATUS").

Return a JSON object with:
- needs_vector_storage: boolean
- is_query: boolean
- is_delete: boolean
- delete_targets: array of strings (for DELETE)
- relationships: array of {from: string, to: string, type: string}
- reasoning: brief explanation`, text)

	type IntentResult struct {
		NeedsVectorStorage bool                      `json:"needs_vector_storage"`
		IsQuery            bool                      `json:"is_query"`
		IsDelete           bool                      `json:"is_delete"`
		DeleteTargets      []string                  `json:"delete_targets"`
		Relationships      []schema.RelationshipInfo `json:"relationships"`
		Reasoning          string                    `json:"reasoning"`
	}

	var result IntentResult
	_, err := be.provider.GenerateStructuredOutput(ctx, prompt, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to extract intent: %w", err)
	}

	return &schema.RequestIntent{
		NeedsVectorStorage: result.NeedsVectorStorage,
		IsQuery:            result.IsQuery,
		IsDelete:           result.IsDelete,
		DeleteTargets:      result.DeleteTargets,
		Relationships:      result.Relationships,
		Reasoning:          result.Reasoning,
	}, nil
}

// AnalyzeQuery analyzes a user query for pre-think search optimization
func (be *BasicExtractor) AnalyzeQuery(ctx context.Context, text string) (*schema.ThinkQueryAnalysis, error) {
	prompt := fmt.Sprintf(`Analyze the following user query to optimize context retrieval from a memory system (Vector DB + Knowledge Graph).
Extract the core intent, key subjects, and refined search keywords.

User Query:
%s

INSTRUCTIONS:
1. Identify the 'query_type':
   - 'factual': Simple question about a person, place, or thing.
   - 'relational': Question about how two or more things are connected.
   - 'summarization': Request to summarize a topic or character.
   - 'narrative': Question about plot points or story events.
2. Extract 'subjects':
   - These are specific named entities (Person, Place, Item) that should be used as anchors for Knowledge Graph traversal.
3. Generate 'search_keywords':
   - Optimized, standalone keywords for Vector similarity search. Expand abbreviations if possible.
4. Define 'expected_answer':
   - A brief description of what the user is looking for (e.g., "Identification of a character's rank and affiliation").

Return a JSON object with:
- query_type: string
- subjects: array of strings
- search_keywords: array of strings
- expected_answer: string
- reasoning: brief explanation of your analysis`, text)

	var result schema.ThinkQueryAnalysis
	_, err := be.provider.GenerateStructuredOutput(ctx, prompt, &result)
	if err != nil {
		return nil, fmt.Errorf("failed to analyze query: %w", err)
	}

	return &result, nil
}

func normalizeNodeType(raw string) (schema.NodeType, bool) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "person", "people", "human":
		return schema.NodeTypePerson, true
	case "org", "organization", "organisation", "company":
		return schema.NodeTypeOrg, true
	case "project", "initiative", "program":
		return schema.NodeTypeProject, true
	case "task", "todo", "ticket", "issue":
		return schema.NodeTypeTask, true
	case "event", "meeting", "incident":
		return schema.NodeTypeEvent, true
	case "document", "doc", "file":
		return schema.NodeTypeDocument, true
	case "session", "conversation", "chat":
		return schema.NodeTypeSession, true
	case "user":
		return schema.NodeTypeUser, true
	case "concept", "idea", "topic":
		return schema.NodeTypeConcept, true
	case "word", "term", "keyword":
		return schema.NodeTypeWord, true
	case "userpreference", "user_preference", "preference":
		return schema.NodeTypeUserPreference, true
	case "grammarrule", "grammar_rule":
		return schema.NodeTypeGrammarRule, true
	case "entity", "":
		return schema.NodeTypeEntity, true
	default:
		return schema.NodeTypeEntity, false
	}
}

func normalizeEdgeType(raw string) (schema.EdgeType, bool) {
	s := strings.ToUpper(strings.TrimSpace(raw))
	s = strings.ReplaceAll(s, "-", "_")
	s = strings.ReplaceAll(s, " ", "_")
	switch s {
	case "MENTION", "MENTIONS":
		return schema.EdgeTypeMentions, true
	case "WORKS_ON", "ASSIGNED_TO", "LEADS", "MANAGES", "OWNER_OF", "OWNS":
		return schema.EdgeTypeWorksOn, true
	case "DEPENDS_ON", "REQUIRES", "BLOCKED_BY", "NEEDS":
		return schema.EdgeTypeDependsOn, true
	case "DISCUSSED_IN", "DISCUSSED_AT", "MENTIONED_IN", "TALKED_IN":
		return schema.EdgeTypeDiscussedIn, true
	case "RELATED_TO", "RELATES_TO", "MEMBER_OF", "LOCATED_IN", "ASSOCIATED_WITH":
		return schema.EdgeTypeRelatedTo, true
	case "CONTAINS":
		return schema.EdgeTypeContains, true
	case "REFERENCED_BY":
		return schema.EdgeTypeReferencedBy, true
	case "SIMILAR_TO":
		return schema.EdgeTypeSimilarTo, true
	case "PART_OF":
		return schema.EdgeTypePartOf, true
	case "CREATED_BY":
		return schema.EdgeTypeCreatedBy, true
	case "USED_IN":
		return schema.EdgeTypeUsedIn, true
	case "CONTRADICTS":
		return schema.EdgeTypeContradicts, true
	case "UPDATES", "UPDATED_BY":
		return schema.EdgeTypeUpdates, true
	case "FAILED_AT":
		return schema.EdgeTypeFailedAt, true
	case "SYNONYM", "SYNONYM_OF":
		return schema.EdgeTypeSynonym, true
	case "STRUGGLES_WITH":
		return schema.EdgeTypeStrugglesWIth, true
	default:
		return schema.EdgeTypeRelatedTo, false
	}
}

func normalizeConfidence(raw float64) (float64, bool) {
	if raw <= 0 || math.IsNaN(raw) || math.IsInf(raw, 0) {
		return 0, false
	}
	if raw > 1 {
		return 1, true
	}
	return raw, true
}
