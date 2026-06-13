# Agent Demo Knowledge Base

## Eino Framework

Eino is CloudWeGo's open-source Go framework for building LLM applications.
It provides Chain, Graph, and Workflow orchestration APIs with compile-time type safety.
Core components include ChatModel, ChatTemplate, Retriever, Indexer, Embedding, Tool, and Agentic variants.

## Milvus Vector Database

Milvus is an open-source vector database optimized for AI applications.
It supports HNSW, IVF_FLAT, IVF_PQ, SCANN, and AUTOINDEX index types.
Use HNSW for high-recall approximate nearest neighbor search and IVF_FLAT for balanced speed and accuracy.
The Milvus Go SDK v2 provides a modern client with CreateCollection, Search, and Index APIs.

## MCP Protocol

The Model Context Protocol (MCP) standardizes context exchange between AI systems.
MCP servers expose tools, resources, and prompts that clients can discover and invoke.
Agents can use MCP to delegate tasks, share context, and coordinate multi-step workflows.

## Agent Collaboration Patterns

Common multi-agent patterns include supervisor-delegation, sequential pipeline, parallel voting, and router-based dispatch.
Effective collaboration requires clear agent roles, shared context, and a task scheduler.
