# Orders Microservice

## Overview

This microservice provides a simple HTTP API for managing orders. It supports creating new orders, retrieving existing orders by ID, and health checks. The service is implemented in Go using Gin for HTTP routing and PostgreSQL for persistence. It features idempotency logic based on `customerName`, `item`, and creation timestamp and uses zerolog for structured logging.

---

## Getting Started

### Prerequisites

- Go 1.25+
- Docker and Docker Compose
- PostgreSQL (if running outside Docker)

### Running with Docker Compose (recommended)

1. Clone the repo.
2. Run:

docker-compose up --build

This will start:

- PostgreSQL database on port `5432`.
- Orders service on port `8080`.
- Automatically run database migrations on startup.

### Running Manually

1. Run PostgreSQL locally or use Docker:

docker run -e POSTGRES_PASSWORD=postgres -p 5432:5432 -d postgres

2. Run migrations manually (example):

migrate -path ./migration -database postgres://postgres:postgres@localhost:5432/ordersdb?sslmode=disable up

3. Build and run service:

go build -o orders-service ./cmd/server
./orders-service

4. Service listens on `http://localhost:8080`.

---

## API Endpoints

- `POST /orders` — Create new order. Requires JSON body with `customer_name` and `item`. Is idempotent based on these fields and creation timestamp.
- `GET /orders/{id}` — Retrieve order by UUID.
- `GET /healthz` — Health check endpoint.

---

## Short Design Notes

### Key Decisions

- **Idempotency implementation**: Instead of relying on a client-generated idempotency key, idempotency is based on the combination of `customerName`, `item`, and creation timestamp. This simplifies client usage but requires careful handling of timestamp equality.
- **Zerolog for logging**: Chosen for its fast, structured, leveled logging to ease troubleshooting and observability.
- **Docker Compose with migrations**: Automated database migration on container startup streamlines deployment.
- **SQLMock in tests**: Chosen to isolate DB calls in unit tests, improving test reliability and speed.

### Trade-offs

- **Timestamp precision for idempotency**: Using exact timestamps as part of idempotency may lead to duplicates if clients send slightly different times. A dedicated idempotency key header might offer better guarantees but was omitted to reduce complexity.
- **Simple migration approach**: Using raw SQL migrations with `migrate` CLI is straightforward but could be enhanced with more robust schema versioning tools.
- **No advanced tracing yet**: Basic logging implemented but distributed tracing (e.g., OpenTelemetry) was deferred due to time constraints.

### If I Had One More Day…

- Add **distributed tracing** with OpenTelemetry to visualize request flows and database calls.
- Implement **rate limiting** and authentication for better production readiness.
- Add **prometheus metrics** for HTTP request counts, latencies, and error rates.
- Extensive **integration/e2e tests** including real DB instances via testcontainers.
- Improve **error handling** by introducing a rich error taxonomy and consistent API error responses.
- Provide a **Swagger UI** endpoint for interactive API exploration.

---
