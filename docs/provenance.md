# Provenance — Unity Gaming Services facts

Each fact below names its source. Sources in this tree are our own captures
(Harbor desktop, owner's title builds) and Unity's public REST documentation.
No Unity SDK source of any form was used; see `CLEANROOM.md`.

## Capture 1 — Gang Beasts v1.28.1 online boot (2026-10-13)

Source: owner's Harbor desktop run, title `0100D58012E12000` at installed
version 786432 (v1.28.1). Raw log stays outside every repository
(`~/.local/share/harbor/log/`); only the sanitized facts below enter.

- The game resolves six Unity hostnames in this order, all to real Unity
  addresses (none is in the OpenPak redirect profile, so no OpenPak service
  was in the path): `player-auth.services.api.unity.com`,
  `wire.unity3d.com`, `lobby.services.api.unity.com`,
  `relay-allocations.services.api.unity.com`,
  `qos-discovery.services.api.unity.com`,
  `multiplay.services.api.unity.com`.
- It opens one TLS connection per hostname through the console's system
  `ssl` service (not a bundled TLS stack), with no handshake failure, then
  UDP datagram sockets consistent with relay transport.
- No `g<id>-lp1.s.n.srv.nintendo.net` dial exists anywhere in the run: the
  linked NEX 4.6.7 / Pia 5.45.0 SDKs carry no live login on this title.
- Static cross-check (owner's client binary,
  `scratch/switch-multiplayer-sweep-20261012/0100D58012E12000/`, same
  version): managed `Coatsink.UnityServices.Authentication / .Matchmaking /
  .QoS`, `Unity.Services.Matchmaker / .Multiplay / .Relay`,
  `RelayServerEndpoint`; no PRUDP access-key literal and no server-ID login
  pair in the native binary.

## Capture 2 — Gang Beasts invite crash (2026-10-13, same run)

- Sending an invite ends in a guest memory fault (`HW.Memory Unmapped`)
  and the application exits to HOME. No network error precedes it in the
  log. The invite UI path is untraced; cause unknown and unexplained.
