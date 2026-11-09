------------------------ MODULE WriteBudgetMutants ------------------------
(* MUTANTS (README.md, WriteBudget): broken write-budget rules that the     *)
(* model must reject. Each mutant config overrides one name of WriteBudget  *)
(* with one of these (TLC definition override, "Name <- Mutant"); the sound *)
(* model itself is never edited.                                            *)
EXTENDS WriteBudget

CONSTANT FixedRate \* today's frame.MinRate (16 KiB/s), in units / tick

\* M-FixedFloor: today's rule, WriteIdle + size / MinRate, with MinRate above
\* the slowest link promised.
FixedLimit == Idle + CeilDiv(S, FixedRate)

\* M-IdleOnly: no total bound (nginx send_timeout style): a reader that drips
\* just inside the idle bound holds the write for as long as it likes.
IdleOnlyExpired == IdleExpired

\* M-TotalOnly: no idle bound: a reader that stops reading holds the write
\* until the total deadline.
TotalOnlyExpired == TotalExpired

\* M-NoCut: neither bound (a plain blocking write): a reader that stops holds
\* the sender forever.
NeverExpired == FALSE
=============================================================================
