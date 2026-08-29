# Luma

Luma is a distributed observability platform designed to collect, process,
store, search, and stream application logs at high throughput.

The initial version focuses on building a reliable log ingestion and search
pipeline with support for near-real-time Live Tail.

## Architecture

```text
Client SDK / Agent
        ↓
    Collector
        ↓
      Kafka
        ↓
 Processing Workers
     ↙       ↘
Elasticsearch  Pub/Sub
     ↓            ↓
 Query API    WebSocket
     ↓            ↓
 Dashboard    Live Tail
```

## Core Goals

- High-throughput log ingestion
- Durable buffering using Kafka
- Horizontally scalable stateless collectors and workers
- Reliable processing with retries, idempotency, and DLQ
- Elasticsearch-based log search with hot/warm storage
- Multi-tenant isolation
- Near-real-time Live Tail
- Backpressure handling from storage to clients
- Multi-AZ high availability
- Built-in metrics, logging, and health monitoring

The project is being built as a production-oriented system with emphasis on idiomatic Go, clean architecture, automated tests, reproducible development scripts, observability, and failure handling.
