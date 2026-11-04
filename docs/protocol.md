# Protocol: ugs

## Hosts

Unity hostnames the service answers. The first six were measured on Gang Beasts v1.28.1 (see Captures below); `matchmaker` was seen resolving in the Harbor sweep of 2026-11-03. Any other host gets `404 unknown host`.

- `player-auth.services.api.unity.com`: anonymous sign-in, session refresh, external-token (every console is a new anonymous player), link/unlink, `GET /v1/users/{id}`. The idToken is an HS256 JWT (`sub` = player id) the other hosts check as `Authorization: Bearer`.
- `lobby.services.api.unity.com`: `/v1/lobbies` (and under `/lobby/v1`): create, get, update, delete, heartbeat, join by id/code, quickjoin, query, player update/leave. Lobbies without a heartbeat for 60 s drop out of queries.
- `relay-allocations.services.api.unity.com`: allocations, join codes, join, regions. 503 until `UGS_RELAY_HOST` is set.
- `qos-discovery.services.api.unity.com`: `GET /v1/qos-servers`.
- `matchmaker.services.api.unity.com`: `/v1|v2/tickets` create, poll, delete. Two or more waiting tickets of a queue (or one waiting 20 s) form a match whose assignment is the relay endpoint.
- `wire.unity3d.com`: configuration and analytics sink; accepted and dropped, nothing stored.
- `multiplay.services.api.unity.com`: no dedicated servers are run; every call is 404 and logged.

## Status of the shapes

Request and response shapes come from Unity's public REST documentation, not from a capture of the game; the first Harbor run against this service will show which paths and fields the game really uses (`ugs unserved` lines list every miss). Open, and needing a capture: the exact path and field set per host as the game sends them (decrypted TLS plaintext of the seven hosts, kept outside the repository), and the UDP relay transport (handshake and framing the client speaks to the endpoint it is handed). Until the relay transport exists, a match can be formed but a player cannot connect through it.

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
