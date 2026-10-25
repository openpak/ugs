# Deploy: ugs

## Image

Built from `Dockerfile`: `golang:1.27-alpine` compiles `/ugsd` with `CGO_ENABLED=0`, which is copied into `gcr.io/distroless/static` and run as `nonroot:nonroot`. Image `ghcr.io/openpak/ugs`. The image exposes TCP 22680 and 22681, with `/ugsd` as the entrypoint.

## Flags

The binary reads no environment variables. Flags and defaults from `cmd/ugsd/main.go`:

| Flag | Default | What |
| --- | --- | --- |
| `-addr` | `:22680` | HTTP listen address (Traefik terminates guest TLS in front) |
| `-health-addr` | `:22681` | health listen address; `/health` returns `{"status":"ok"}` |
| `-mode` | `forward` | `forward` (tap to the real upstream) or `refuse` (404 everything) |
| `-upstream-scheme` | `https` | upstream scheme (http only for local tests) |
| `-upstream-timeout` | `20s` | upstream round-trip timeout |
| `-connectivity-url` | empty | shared connectivity API base; empty disables relay integration |
| `-connectivity-token-file` | empty | file holding this backend's connectivity bearer token (0600); required with the URL |
| `-connectivity-namespace` | `ugs` | backend namespace for participant leases |

## Shared connectivity

Wiring today is the seam only: `-connectivity-url` +
`-connectivity-token-file` (0600, bearer token, one namespace per backend —
never shared across backends). Unset means relay integration is off and the
startup log says so. Live calls land with M2.

## Build and test

```sh
go build ./...
go test ./...
```

## Publishing

A `v*.*.*` tag publishes `ghcr.io/openpak/ugs:latest` and deploys, like
every other OpenPak service. No route, quadlet or catalogue entry exists
yet: a local tap plus a healthy listener does not establish playability,
and the profile redirect that would point console traffic here is a
contract request to the Switch adapter, not part of this tree.
