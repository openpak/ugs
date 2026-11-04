# Changelog

## v0.2.0 - 2026-11-04
- ugs now answers the Unity hostnames itself instead of tapping them to Unity: anonymous player-auth, lobby, matchmaker tickets, relay allocations, qos-discovery and the wire sink, all stored in Postgres (`UGS_DATABASE_URL`, schema `ugs`). Forward mode, `-mode`, the connectivity flags and the per-request httplog are gone; each request logs host, method, path shape, status and ms.
- Release workflow added (tag builds `ghcr.io/openpak/ugs`).
- Shapes follow Unity's public docs and are unchecked against the game; the relay transport awaits a capture (docs/protocol.md).


## v0.1.0 — 2026-11-03
- Per-request logging: one slog line per HTTP request (method, path, status, ms) on stderr via internal/httplog. Query strings, headers and bodies are never logged; /healthz and /metrics are skipped.

