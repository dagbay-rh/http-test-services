# http-test-services

A stateless HTTP/gRPC mock and echo service used for testing API gateway routing on [console.redhat.com](https://console.redhat.com).

> Forked from [apicast-test-services](https://github.com/RedHatInsights/apicast-test-services) and rewritten in Go.

## Endpoints

All HTTP endpoints are available at both `/` and `/api/http-test-services/` (configurable via `API_PREFIX` env var), with optional version prefix (e.g. `/api/http-test-services/v1/...`).

| Route | Method | Description |
|-------|--------|-------------|
| `/` | GET | 302 redirect to `/api/http-test-services/request` |
| `/request` | GET | JSON with request env and headers |
| `/headers` | GET | JSON with sorted HTTP headers |
| `/env` | GET | JSON with sorted server environment variables |
| `/redirect?redirect_to=<path>` | GET | 302 redirect to the given path (400 if missing) |
| `/ping` | GET | `{"status":"available"}` |
| `/private/ping` | GET | `{"status":"available"}` |
| `/upload` | POST | Accepts multipart file upload, returns `{"status":"posted","upload_byte_size":N}` |
| `/identity` | GET | Decodes base64 `x-rh-identity` header (400 if missing) |
| `/wss` | GET | WebSocket echo server |
| `/sse` | GET | Server-Sent Events stream (ping every 3s) |
| `/{version}/openapi.json` | GET | OpenAPI spec |

All endpoints support `?sleep=N` (delay in seconds) and `?status=N` (override response status code) query parameters.

A gRPC `PingService` runs on port 50051 with a single `Ping` RPC that echoes the message back.

## Configuration

| Env var | Description | Default |
|---------|-------------|---------|
| `ACG_CONFIG` | Path to Clowder JSON config file (reads `webPort`) | `9092` |
| `API_PREFIX` | Path prefix for all routes | `/api/http-test-services` |

## Build and run

```sh
go build -o http-test-services .
./http-test-services
```

The HTTP server starts on `:9092` (or the port from `ACG_CONFIG`) and the gRPC server on `:50051`.

## Run tests

```sh
go test ./...
```

## Docker

```sh
docker build -t http-test-services .
docker run -p 9092:9092 -p 50051:50051 http-test-services
```
