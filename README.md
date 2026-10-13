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
