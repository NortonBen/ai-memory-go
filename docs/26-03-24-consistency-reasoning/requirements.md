# Consistency Reasoning Requirements

## 1. User Stories

- As an AI Memory Engine, I want to automatically resolve contradictions or updates between new memories and existing knowledge so that my memory bank remains accurate, de-duplicated, and up-to-date.
- As a Developer, I want the engine to use a configurable similarity threshold (e.g., Vector Distance < 0.1) to trigger an LLM-based comparison between two seemingly overlapping nodes before blindly inserting duplicate information.

## 2. Acceptance Criteria

- **Similarity Threshold Trigger:** During the `Add` or `Cognify` pipeline, before an extracted Entity is stored, the engine MUST perform a `SimilaritySearch` against the VectorStore using its embedding. If an existing node's similarity distance is `< 0.1` (configurable), the consistency reasoning flow is triggered.
- **LLM Comparison:** The engine MUST prompt the LLM to compare the `Existing Node` and the `New Entity`. The LLM decides whether the new info is an `UPDATE` (more recent/accurate), a `CONTRADICTION` (conflicting fact), or `IGNORE` (duplicate/irrelevant).
- **Graph Updates for UPDATE:** If the LLM determines an `UPDATE`, the engine MUST overwrite the properties of the `Existing Node` with the new data, and optionally draw an `UPDATES` edge if maintaining a history log is required. (For simplicity, overwrite old node).
- **Graph Updates for CONTRADICTION:** If the LLM determines a `CONTRADICTION`, the engine MUST insert the new node and explicitly create a `CONTRADICTS` relationship edge between the new node and the existing node so that both perspectives are retained for future LLM context retrieval.
- **Non-blocking Operations:** The consistency check MUST be integrated efficiently to avoid bottlenecking bulk inserts.

## 3. Business Rules

- AI uses Vector Search to compare new entities with old nodes.
- Distance < 0.1 triggers LLM comparison.
- LLM outputs resolution strategy (UPDATE, CONTRADICT).
- Go engine executes database commands to update Node properties or create CONTRADICTS edges.
