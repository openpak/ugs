# ugs — OpenPak's Unity Gaming Services backend (UGS)

The service a Unity-online Switch title reaches when its Unity hostnames
resolve to OpenPak. One process serves every Unity hostname a title uses,
dispatching on the TLS SNI / HTTP Host Traefik hands it.

Status: **M1 tap** (see `prds/ugs-backend-prd.md`). It terminates the
guest-facing HTTPS the way `servers/t17` and `servers/klei` do (Traefik
ends the console TLS, plain HTTP on **22680** inside the pod network,
health on **22681**), logs the *shape* of each call — method, host, path
shape, query key names, sizes, status, duration — and forwards the call
to the real upstream untouched, so the game keeps working while the
contract is recorded. Bodies, tokens and identifiers never reach the log;
`tap` enforces that by construction with tests proving it.

First title: Gang Beasts (`0100D58012E12000`, v1.28.1) — UGS-only,
measured 2026-10-13 (no NEX dial; system `ssl` service; invite path
crashes the guest — see `docs/provenance.md`).

## Shared connectivity

Relay transport does not live here. When this backend serves relay
allocations (M2), each allocation becomes a participant lease in the
shared [`servers/connectivity`](../connectivity/) service — namespace
`ugs`, one lease per authenticated player per connection generation, even
behind a shared public IP — and same-household pairs get a verified native
pair relay whose endpoint is what the clients are handed. TURN credentials
come from `POST /v1/ice`. The Unity relay handshake the client already
speaks is the native verification path: no bind without destination proof,
per the shared plan (`docs/shared-connectivity-plan.md`).

Wiring today is the seam only: `-connectivity-url` +
`-connectivity-token-file` (0600, bearer token, one namespace per backend —
never shared across backends). Unset means relay integration is off and the
startup log says so. Live calls land with M2.

## Clean room

Read `CLEANROOM.md` before touching this tree. The short version: our own
wire captures and Unity's public REST documentation are the inputs. No
Unity SDK source — original or decompiled — enters this tree, the working
set, or the notes this code is written from. Every wire fact kept here
carries its provenance in `docs/provenance.md`.

## Build and test

```sh
go build ./...
go test ./...
```

A `v*.*.*` tag publishes `ghcr.io/openpak/ugs:latest` and deploys, like
every other OpenPak service. No route, quadlet or catalogue entry exists
yet: a local tap plus a healthy listener does not establish playability,
and the profile redirect that would point console traffic here is a
contract request to the Switch adapter, not part of this tree.
