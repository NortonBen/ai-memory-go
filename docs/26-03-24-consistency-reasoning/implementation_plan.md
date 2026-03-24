# Implementation Plan: Consistency Reasoning

## User Review Required

No breaking changes. New feature involves an addition to the extraction pipeline. A new configuration option `WithConsistencyThreshold` will be introduced.

## Proposed Changes

### Configuration

#### [MODIFY] schema/options.go

- Add `ConsistencyThreshold` to `AddOptions`.
- Add `WithConsistencyThreshold(t float32)` so users can enable/disable/tune the feature.

### Core Schema

#### [MODIFY] schema/schema.go

- Add `type ResolutionAction string` and the constants `ResolutionUpdate, ResolutionContradict, ResolutionIgnore`.
- Add `type ConsistencyResult`.

### Extractor Interface

#### [MODIFY] extractor/extractor.go

- Expand the `LLMExtractor` interface with:
  `CompareEntities(ctx context.Context, existing Entity, newEntity Entity) (*ConsistencyResult, error)`

#### [MODIFY] extractor/lmstudio.go

- Implement `CompareEntities` for `LMStudioExtractor` (and subsequently others).
- Add strict JSON schema prompt to decide whether the new entity `UPDATES` or `CONTRADICTS` the old one.

### Memory Engine

#### [MODIFY] engine/engine.go

- Update `Add` / `Cognify` pipeline.
- Instead of blindly inserting nodes into the GraphStore:
  - Vectorize the new Entity's Data.
  - Perform `vectorStore.SimilaritySearch` with `limit=1` and `threshold` based on `opts.ConsistencyThreshold` (default ~0.1).
  - If match found, fetch the Old Entity from GraphStore.
  - Call `llmExtractor.CompareEntities()`.
  - Depending on action:
    - **UPDATE:** Merge properties or overwrite, and apply changes to GraphStore and VectorStore.
    - **CONTRADICT:** Insert new node, insert `CONTRADICTS` edge in GraphStore, insert into VectorStore.
    - **IGNORE:** Skip insertion.

### Testing

#### [NEW] examples/consistency_reasoning/main.go

- Create a test script. Insert a fact: "Apple is a fruit."
- Insert another fact: "Apple is a technology company." (This should trigger a CONTRADICT).
- Insert another fact: "Apple is a red fruit." (This should trigger an UPDATE or IGNORE).

## Verification Plan

### Automated Tests

- Run `go run examples/consistency_reasoning/main.go` using a local LLM to assert that the `CONTRADICTS` edges are natively produced on factual misalignments.
