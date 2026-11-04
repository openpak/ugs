# Deploy: ugs

## Image

Built from `Dockerfile`: `golang:1.27-alpine` compiles `/ugsd` with `CGO_ENABLED=0`, which is copied into `gcr.io/distroless/static` and run as `nonroot:nonroot`. Image `ghcr.io/openpak/ugs`. The image exposes TCP 22680 and 22681, with `/ugsd` as the entrypoint.

## Configuration

Flags: `-addr` (default `:22680`, HTTP; Traefik terminates guest TLS in front) and `-health-addr` (default `:22681`, `/health` pings Postgres). Environment:

| Variable | What |
| --- | --- |
| `UGS_DATABASE_URL` | Postgres DSN (required; schema `ugs`, pool of 2). Everything is stored there: signing key, players, sessions, lobbies, tickets, allocations |
| `UGS_RELAY_HOST`, `UGS_RELAY_PORT` | the relay transport endpoint handed to clients. Unset: allocations answer 503 and matchmaker tickets stay in progress |
| `UGS_REGION` | relay region name (default `openpak`) |

Nothing is forwarded to Unity. A host or path that is not served answers 404 and is logged as `ugs unserved` with the path shape and query key names only.

## Build and test

```sh
go build ./...
go test ./...
```

## Publishing

A `v*.*.*` tag publishes `ghcr.io/openpak/ugs:latest` (amd64 and arm64); the box runs it from the quadlet `openpak-ugs` (AutoUpdate=registry). Traefik routes the seven hostnames (SNI) to port 22680 with one OpenPak-CA leaf (`ugs.pem`); the Switch profile lists `.unity.com` and `.unity3d.com` (signed ceiling). The relay transport is not part of this service yet.
