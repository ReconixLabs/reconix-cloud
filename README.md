# Reconix Cloud

Reconix Cloud is the controlled API and job execution layer for the independent Go Reconix reconnaissance CLI.

```text
Reconix -> Reconix Cloud -> Normalized Findings v1 -> ThreatLens
```

Reconix remains a standalone CLI. Cloud does not contain reconnaissance algorithms or ThreatLens integration.

## Architecture

The API validates an allowlisted target, persists a queued scan, and returns immediately. A separate bounded worker claims jobs from PostgreSQL using transactional row locking, invokes Reconix using direct process arguments and `--json`, normalizes the versioned native result, and stores both raw and normalized output.

The API and worker are separate processes. PostgreSQL is the durable queue and result store; no external broker is required. The worker reclaims stale `starting` or `running` jobs after the configured stale timeout.

## Configuration

Copy `.env.example` to `.env` and set an explicit target allowlist. `TARGET_POLICY_MODE=allowlist` is the default. Domain entries include subdomains; CIDR entries allow literal IP targets. Private, loopback, link-local, multicast, unspecified, and metadata addresses are rejected.

Important settings:

| Variable | Purpose |
|---|---|
| `API_KEY` | Bearer token for non-public endpoints |
| `DATABASE_URL` | PostgreSQL connection string; required by API and worker |
| `RECONIX_BINARY` | Reconix executable |
| `RECONIX_CONFIG` | Standard profile config |
| `RECONIX_WORKDIR` | Reconix working directory |
| `ALLOWED_DOMAINS` | Comma-separated domain allowlist |
| `ALLOWED_CIDRS` | Comma-separated CIDR allowlist |
| `MAX_CONCURRENT_SCANS` | Worker concurrency |
| `MAX_SCAN_DURATION_SECONDS` | Per-process timeout |
| `MAX_OUTPUT_SIZE` | Maximum Reconix stdout bytes |
| `WORKER_POLL_INTERVAL_SECONDS` | Worker queue polling interval |
| `STALE_JOB_TIMEOUT_SECONDS` | Age after which unfinished jobs may be reclaimed |

Profiles are controlled mappings, not arbitrary CLI flags:

| Profile | Current behavior |
|---|---|
| `safe` | Ports 80 and 443, lower concurrency |
| `standard` | Uses `RECONIX_CONFIG` |
| `deep` | Common infrastructure ports, higher concurrency |

## API

Public:

```text
GET /api/v1/health
GET /openapi.json
```

Authenticated with `Authorization: Bearer $API_KEY`:

```text
POST /api/v1/scans
GET  /api/v1/scans
GET  /api/v1/scans/{scan_id}
GET  /api/v1/scans/{scan_id}/results
POST /api/v1/scans/{scan_id}/cancel
```

Create a scan:

```bash
curl -H "Authorization: Bearer change-me" \
  -H "Content-Type: application/json" \
  -d '{"target":"example.com","profile":"standard"}' \
  http://localhost:8080/api/v1/scans
```

The lifecycle is `queued`, `starting`, `running`, `completed`, `failed`, or `cancelled`. Progress is represented by `stage`; no percentage is fabricated.

## Local Docker

```bash
Copy-Item .env.example .env
docker compose build
docker compose up
```

The compose build context is the parent `ReconixLabs` directory because the image includes both repositories. The API and worker use the same non-root image; PostgreSQL is the durable queue and persistent result store.

## Development

```bash
go test ./...
go vet ./...
go build ./...
```

Run those commands from this repository. Reconix itself is tested independently from its repository.

## Security limitations

The service performs resolution-time target checks and rechecks before execution. A future hardened deployment should place workers in a network namespace or egress proxy that enforces the same policy at connection time, because application-level DNS validation cannot eliminate every TOCTOU race. Process cancellation uses Go's context-backed process termination; a dedicated Unix process-group supervisor is recommended before untrusted multi-tenant deployment.
