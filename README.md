# Unity Gaming Services

OpenPak's Unity Gaming Services Server.

| | |
| --- | --- |
| Networks | Nintendo Switch |
| Protocol | Unity REST hostnames answered from OpenPak's own state; TLS ended by Traefik |
| Image | `ghcr.io/openpak/ugs` |
| Licence | AGPL-3.0-only |

## Titles

| Game | Network | Title ID | Module |
| --- | --- | --- | --- |
| [Gang Beasts](https://openpak.org/titles#t-125) | Nintendo Switch | `0100D58012E12000` | player-auth, lobby, matchmaker, relay allocations (relay transport pending a capture) |

## Networks

| Network | Reached through |
| --- | --- |
| Nintendo Switch | Unity hostnames, TLS ended by Traefik |

## What it serves

| Listener | What | Port |
| --- | --- | --- |
| HTTP | Unity hostnames, answered here, never forwarded | TCP 22680 |
| Health | `/health` | TCP 22681 |

## Run

```sh
go build ./...
go test ./...
```

## Docs

- [Protocol](docs/protocol.md)
- [Deploy](docs/deploy.md)

Live compatibility status: [openpak.org/titles](https://openpak.org/titles).

## Licence

AGPL-3.0-only. See [LICENSE](LICENSE).
