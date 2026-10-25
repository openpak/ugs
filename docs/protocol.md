# Protocol: ugs

## Hosts

Unity hostnames the service answers, from `knownHosts` in `cmd/ugsd/main.go`. The surface was measured on Gang Beasts v1.28.1 (see [provenance](provenance.md)). Any other host, and every host in `-mode refuse`, gets `404 unknown host`.

- `player-auth.services.api.unity.com`
- `wire.unity3d.com`
- `lobby.services.api.unity.com`
- `relay-allocations.services.api.unity.com`
- `qos-discovery.services.api.unity.com`
- `multiplay.services.api.unity.com`

## Forward mode

In the default `-mode forward`, a call to a known host goes to the same host over `-upstream-scheme` with method, path, query, headers and body unchanged. The upstream status, headers and body stream back to the caller. Hop-by-hop headers are not copied. Only the call's shape is logged (see the `tap` package).

## Endpoints and message shapes

Nothing recorded yet.
