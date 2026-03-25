# Tasks: Context-Aware Request Pipeline

## Phase 1: Schema & Extractor Preparation
- [ ] **Schema**: Update `schema.RequestIntent` to include `Relationships` field. `[engine/schema/schema.go]`
- [ ] **Extractor**: Update `ExtractRequestIntent` prompt in `extractor/basic_extractor.go`.
- [ ] **Extractor**: Update `RequestIntent` struct mapping in the extractor to handle the new field.

## Phase 2: Engine Refactoring
- [ ] **Engine**: Update `engine.Request` to fetch history *before* calling `ExtractRequestIntent`. `[engine/request.go]`
- [ ] **Engine**: Implement `processRelationships` helper method in `engine/request.go`.
- [ ] **Engine**: Update the routing logic to ensure queries wait for relationship analysis.

## Phase 3: Relationship Analysis Logic
- [ ] **Engine**: Enhance `AnalyzeHistory` or create a new `AnalyzeRelationships` method to specifically build the graph.
- [ ] **Integration**: Ensure the relationship analysis correctly identifies "Bot" and "Current User".

## Phase 4: Verification
- [ ] **Tests**: Create unit tests for the new request flow.
- [ ] **Manual**: Run `chat_history_agent` example to verify "cho nhà tôi tên gì" uses the constructed relationship.
- [ ] **Screenshot**: Capture CLI output showing the flow stages.

## Phase 5: Documentation
- [ ] Update `walkthrough.md` with the new architecture.
- [ ] Update `engine_flow_diagram.md` if necessary (though the design.md contains a new one).
