# Clean room — Unity Gaming Services work

This tree replaces Unity's cloud for the titles that use it. Unity's SDK
packages are proprietary works under Unity's own licence terms. They are
not an input here — not as source, not as decompiled output, not as
structure to mirror. The working set for this tree is:

1. **Our own wire.** Requests a console we own sends to a service we run,
   or transparently through the M1 tap to the real upstream while the game
   keeps working. A path, field name or flow observed there is a fact about
   the console.
2. **Unity's public REST documentation** (docs.unity.com and the same class
   of published API references). Field semantics from public docs, never
   implementation from SDK source.
3. **Compatibly licensed code and our own repositories**, per
   `docs/clean-room-policy.md`.

Rules for every change in this tree:

- No Unity SDK file — original, decompiled, disassembled or transcribed —
  is read with the intent to reproduce it, stored anywhere in the working
  set (including `scratch/` notes this code is written from), or committed.
- What transfers from any outside reading is wire facts only: hostnames,
  endpoint paths, field names and types, flows, error codes. Never file
  layout, package splits, type or function names beyond the wire, table
  schemas, or handler shapes.
- Every non-obvious fact kept here is recorded in `docs/provenance.md`
  with its source: our capture (date, title, build) or the public document
  (title, section). A fact the wire contradicts loses, whoever stated it.
- Bodies, tokens, identifiers and player data never enter logs or committed
  fixtures. `tap` exists to make that structural: if a value can reach a
  log line, the change is wrong. Tests prove it.

The test from the workspace policy applies verbatim: could you have written
this line having only read a *description* of what the server does? If yes,
write it. If you are reproducing how someone else chose to express it, stop.
