# Hit Counter Go API

Simple Go HTTP API to count website visits. Maintains an in-memory atomic counter.

## Endpoints

- `POST /hit` (also accepts `GET`): increments the counter and returns `{ "hits": <newVal> }`.
- `GET /count`: returns current count without incrementing.
- `GET /healthz`: liveness probe.

All JSON responses use `application/json` except `/healthz`.