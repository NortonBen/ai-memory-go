# SurrealDB Adapter Requirements

## User Stories

1. As a memory engine, I want to use SurrealDB to store both nodes and relationships so that I can traverse graphs efficiently using SurrealQL.
2. As a system, I want to connect to a SurrealDB instance using standard connection strings and credentials.
3. As a platform, I want to support hybrid vector-graph capabilities without needing separate databases, leveraging SurrealDB's upcoming vector support or basic properties.
4. As a developer, I want to execute batch inserts for nodes and edges to optimize data ingestion speed.

## Acceptance Criteria

- [ ] Connect successfully to a SurrealDB database using either WS (WebSocket) or HTTP protocols.
- [ ] Implement `StoreNode`, `GetNode`, `UpdateNode`, `DeleteNode` using SurrealDB tables and Record IDs.
- [ ] Implement `CreateRelationship`, `GetRelationship`, `UpdateRelationship`, `DeleteRelationship` using SurrealDB graph relational tables (`RELATE` statement).
- [ ] Support complex queries such as `FindConnected` and `TraverseGraph` utilizing SurrealDB graph traversals (`->`, `<-`, `<->`).
- [ ] Perform batch operations utilizing `transaction` statements or bulk inserts in SurrealQL.
- [ ] Track entity counts efficiently.
- [ ] Support SurrealDB namespaces and databases configuration seamlessly.
