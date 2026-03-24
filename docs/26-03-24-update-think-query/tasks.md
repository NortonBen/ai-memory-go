# Tasks: Update ThinkQuery and Graph Traversal

## Backend

- [x] Define `ThinkQuery` struct in `schema/schema.go`
  - Make sure it has `HopDepth` and `Limit` configuration.
- [x] Update `MemoryEngine` interface in `engine/engine.go` to use `*schema.ThinkQuery`
- [x] Refactor `Think` method to explicitly execute entity extraction on query text
- [x] Refactor `Think` method to fetch explicitly related Anchor Nodes
- [x] Refactor `Think` method to perform Graph Traversal (hop=HopDepth) and append nodes to the prompt context
- [x] Reconcile `Think` signature across example scripts (`knowledge_graph_builder`, `sqlite_lmstudio`, `in_memory_lmstudio`)

## Verification

- [ ] Run `examples/knowledge_graph_builder` and verify that the LLM explanation explicitly references the traversed nodes.
