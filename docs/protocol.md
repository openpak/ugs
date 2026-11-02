# Protocol: ugs

## Hosts

Unity hostnames the service answers, from `knownHosts` in `cmd/ugsd/main.go`. The surface was measured on Gang Beasts v1.28.1 (see Captures below). Any other host, and every host in `-mode refuse`, gets `404 unknown host`.

- `player-auth.services.api.unity.com`
- `wire.unity3d.com`
- `lobby.services.api.unity.com`
- `relay-allocations.services.api.unity.com`
- `qos-discovery.services.api.unity.com`
- `multiplay.services.api.unity.com`

## Forward mode

In the default `-mode forward`, a call to a known host goes to the same host over `-upstream-scheme` with method, path, query, headers and body unchanged. The upstream status, headers and body stream back to the caller. Hop-by-hop headers are not copied. Only the call's shape is logged (see the `tap` package).

## Endpoints and message shapes

Nothing recorded yet. The service is a tap: it logs the shape of each call (method, host, path
shape, query key names, sizes, status, duration) and forwards it untouched, so the game keeps
working while the contract is recorded. Bodies, tokens and identifiers never reach the log; `tap`
enforces that by construction with tests.

## Shared connectivity

Relay transport does not live here. When this backend serves relay allocations, each allocation
becomes a participant lease in the shared [`connectivity`](https://github.com/openpak/connectivity)
service (namespace `ugs`, one lease per authenticated player per connection generation, even
behind a shared public IP). Same-household pairs get a verified native pair relay whose endpoint
is what the clients are handed; TURN credentials come from `POST /v1/ice`. The Unity relay
handshake the client already speaks is the native verification path: no bind without destination
proof.

## Captures

Sources are our own captures (Harbor desktop, owner's title builds) and Unity's public REST
documentation; no Unity SDK source of any form was used (`CLEANROOM.md`). Raw logs stay outside
every repository.

**Gang Beasts v1.28.1 online boot (2026-10-13)**, title `0100D58012E12000`, version 786432. First
title; UGS-only.

- It resolves six Unity hostnames in this order, all to real Unity addresses (none in the OpenPak
  redirect profile, so no OpenPak service was in the path): the six listed under Hosts.
- It opens one TLS connection per hostname through the console's system `ssl` service (not a
  bundled TLS stack), with no handshake failure, then UDP datagram sockets consistent with relay
  transport.
- No `g<id>-lp1.s.n.srv.nintendo.net` dial exists in the run: the linked NEX 4.6.7 / Pia 5.45.0
  SDKs carry no live login on this title.
- Static cross-check of the client binary (same version): managed `Coatsink.UnityServices.
  Authentication / .Matchmaking / .QoS`, `Unity.Services.Matchmaker / .Multiplay / .Relay`,
  `RelayServerEndpoint`; no PRUDP access-key literal and no server-ID login pair in the native
  binary.
- Sending an invite ends in a guest memory fault (`HW.Memory Unmapped`) and the application exits
  to HOME. No network error precedes it; the invite UI path is untraced and the cause unknown.
