# Requirements - Core Modules Refactoring

## Overview
The goal is to refactor the `vector`, `storage`, and `graph` directories to improve code organization. Currently, these directories contain many files at the root level, mixing interfaces, core logic, and multiple provider implementations (adapters/embedders).

## User Stories
- As a developer, I want a clean directory structure so that I can easily find and focus on specific provider implementations.
- As a maintainer, I want implementations to be isolated in their own packages to reduce the risk of accidental dependencies and make the codebase more modular.

## Acceptance Criteria
- [ ] `vector/` directory is reorganized:
    - [ ] Adapters (pgvector, qdrant, sqlite, redis, inmemory) are moved to sub-directories.
    - [ ] Embedders (openai, ollama, openrouter, lmstudio) are moved to sub-directories.
- [ ] `storage/` directory is reorganized:
    - [ ] Adapters (postgresql, sqlite) are moved to sub-directories.
    - [ ] Utilities (pooling, health) are clearly separated from core logic.
- [ ] `graph/` directory is reorganized:
    - [ ] Adapters (neo4j, sqlite, redis, inmemory) are moved to sub-directories.
- [ ] Package names are updated to match their new directory structure (following Go conventions).
- [ ] All import statements across the entire project are updated and correct.
- [ ] The project builds successfully without errors.
- [ ] All existing tests pass.

## Business Rules (to be saved in Memory)
1. Every provider implementation MUST reside in its own sub-directory.
2. Core interfaces and shared types SHOULD remain at the root of the module (e.g., `vector/vector.go` in package `vector`).
3. Factory patterns SHOULD be updated to import and use the new sub-packages.
