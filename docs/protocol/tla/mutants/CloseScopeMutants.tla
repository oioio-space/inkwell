-------------------------- MODULE CloseScopeMutants --------------------------
(* Block PU1d, README "CloseScope": definition overrides of CloseScope's rule operators, and the witnesses       *)
(* (invariants the sound model must violate: the states the properties speak of are reached).                  *)
EXTENDS CloseScope

\* MC1 (P-16 off: spec §8.7.4 before PU1d, today's code): a refusal or the deadline closes every connection to the
\* peer (ClosePeer), as Drop does, not only the one the stream ran on.
FailClosePeer(n, c) == CloseAll(n, c)
\* MC2 (P-17 off: inkwell before the lobby fix of 2026-11-23): an opening starts only on a connectedness event; a
\* connection that arrives while the peer is connected, or that remains when the session's one is lost, starts none.
OpenOnEvent(n) == evt[n]
\* MC3 (P-16's named connection off, review finding 1: host.NewStream, today's code): the node learns the opening's
\* connection only when NewStream returns; a failure before that names none, so it can only close them all.
NamesAtReturn(st) == IF st.st = "neg" THEN NoConn ELSE st.c

\* Witnesses.
NoFailBeside == ~gh.beside
NoQuietOpen == ~gh.quiet
NoUnreturnedFail == ~gh.neg
=============================================================================
