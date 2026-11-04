package main

import "context"

// eraseNothing is the account-erasure hook (card openpak/prds#120). ugs holds nothing that can be tied
// to an OpenPak account: every console signs in anonymously and gets a random player id, the ids in
// ugs.sessions, lobbies (host), tickets and allocations are those random ids, and no table has an
// account, BAAS, console or vendor-account column. The sign-in handlers read no identity from the
// request. So there is no row to find for an account, and the endpoint answers 0 rows (the finaliser
// still gets a clean answer, and the call is logged). Short-lived rows expire on their own
// (Sweep: lobbies after 10 minutes, tickets and allocations after a day).
func eraseNothing(context.Context, string) (int, error) { return 0, nil }
