# AI Memory Brain - Architecture Overview

This document provides a comprehensive architectural analysis and data flow specification for the `ai-memory-go` integration project. 

The system implements a unified "Memory Engine" that ingests text, extracts entities/relationships using Large Language Models (LLMs), generates vector embeddings, and stores this data in a hybrid graph-vector storage backend for semantic and relational retrieval.

## System Architecture

```mermaid
graph TD
    Client[Client App] --> |AddMemory / Search| Engine[Memory Engine]

    subgraph Memory Engine Module
        Engine --> |Submit Task| WorkerPool[Worker Pool]
        Engine --> |CRUD| RelStore[(Relational Store<br/>SQLite/Postgres)]
        
        WorkerPool --> |1. Extract Entities| Extractor[LLM Extractor]
        WorkerPool --> |2. Generate Embedding| Embedder[Auto Embedder]
        WorkerPool --> |3. Store Graph| GraphStore[(Graph Store<br/>SQLite/Neo4j)]
        WorkerPool --> |4. Store Vector| VectorStore[(Vector Store<br/>SQLite/Qdrant)]
    end

    subgraph External LLM Providers
        Extractor -.-> |OpenAI/DeepSeek/Ollama| API1[LLM APIs]
        Embedder -.-> |OpenAI/OpenRouter/Ollama| API2[Embedding APIs]
    end
```

## Data Model

The system operates on three distinct layers of data:

1.  **DataPoint (Relational):** Represents the raw input text, metadata, and processing state.
2.  **Node & Edge (Graph):** Represents the extracted concepts and their relationships.
3.  **Vector (Embedding):** Represents the semantic mathematical representation of the text or extracted chunks.

```mermaid
erDiagram
    DATAPOINT {
        string ID
        string Content
        string ContentType
        string SessionID
        string ProcessingStatus
    }
    NODE {
        string ID
        string Type
        json Properties
        float Weight
    }
    EDGE {
        string ID
        string FromNodeID
        string ToNodeID
        string Type
        float Weight
    }
    VECTOR {
        string ID
        float_array Embedding
        json Metadata
    }

    DATAPOINT ||--o{ NODE : extracts_into
    DATAPOINT ||--o| VECTOR : embeds_into
    NODE ||--o{ EDGE : connects
```

## Core Workflows

### 1. `AddMemory` Flow (Ingestion)

When new information is added, the engine responds immediately by storing the raw `DataPoint` as `pending` and asynchronously processing it via the worker pool.

```mermaid
sequenceDiagram
    participant Client
    participant Engine
    participant RelStore as Relational Store
    participant Pool as Worker Pool

    Client->>Engine: AddMemory(text, session)
    Engine->>RelStore: StoreDataPoint(status: pending)
    RelStore-->>Engine: DB ID
    Engine->>Pool: Submit AddTask
    Engine-->>Client: DataPoint (Pending)
```

### 2. `Cognify` Flow (Extraction & Vectorization)

The asynchronous background worker processes the pending data.

```mermaid
sequenceDiagram
    participant Pool as Worker Pool
    participant Extractor
    participant Embedder
    participant Stores as Storage (Graph/Vector/Rel)

    Pool->>Stores: UpdateDataPoint(status: processing)
    
    par LLM Extraction
        Pool->>Extractor: Extract(text)
        Extractor-->>Pool: Nodes + Edges
        Pool->>Stores: StoreGraph(Nodes, Edges)
    and Embedding Generation
        Pool->>Embedder: GenerateEmbedding(text)
        Embedder-->>Pool: []float32
        Pool->>Stores: StoreVector(id, embedding)
    end
    
    Pool->>Stores: UpdateDataPoint(status: completed)
```

### 3. `Search` Flow (Hybrid Retrieval)

The search pipeline takes a query and retrieves relevant data using both mathematical similarity (Vectors) and relational context (Graph).

```mermaid
sequenceDiagram
    participant Client
    participant Engine
    participant Embedder
    participant Stores as Storage Layer

    Client->>Engine: Search(query)
    
    par Semantic Similarity
        Engine->>Embedder: GenerateEmbedding(query)
        Embedder-->>Engine: []float32
        Engine->>Stores: SimilaritySearch(embedding)
        Stores-->>Engine: Top-K Vector Hits
    and Graph Context
        Engine->>Stores: Extract entities from query
        Engine->>Stores: TraverseGraph(entity_ids, hops=2)
        Stores-->>Engine: Neighborhood Nodes/Edges
    end
    
    Engine->>Engine: Rerank & Merge Results
    Engine-->>Client: Processed Results
```

## Package Dependencies & Capabilities

-   **`extractor`**: Abstraction over large language models for structured entity extraction. Implements `Anthropic`, `DeepSeek`, `Gemini`, `Ollama`, and `OpenAI`.
-   **`vector`**: Abstraction over embedding models and vector databases. Implements `AutoEmbedder` (with local caching and fallbacks), `Ollama`, `OpenAI`, and `OpenRouter` providers. Storage backends include `SQLite` (`sqlite-vec`), `Qdrant`, and `PgVector`.
-   **`graph`**: Abstraction over knowledge graph operations (node creation, edge linking, recursive BFS traversal). Implements `SQLite` (Recursive CTE) and `Neo4j`.
-   **`storage`**: Traditional relational data store for the primary `DataPoint` tracking, implementing `SQLite` and `PostgreSQL`.
-   **`engine`**: The overall orchestrator wrapping the extractor, embedder, and storage layers, maintaining a concurrent `WorkerPool`.
