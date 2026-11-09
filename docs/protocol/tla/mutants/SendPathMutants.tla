------------------------- MODULE SendPathMutants -------------------------
(* MUTANTS (README.md, SendPath): broken send-path rules that the model      *)
(* must reject. A mutant config either changes the rule-set constants of    *)
(* SendPath (the v1 rules, a window size) or overrides one of its names     *)
(* with one of these (TLC definition override, "Name <- Mutant"); the sound *)
(* module is never edited.                                                  *)
EXTENDS SendPath

\* M-SilentAbandon: a cut resets the stream without rule L (no epoch change,
\* the webview is not told): the object is never sent again (sync/1 before RL1).
NoRuleL == FALSE

\* M-NoCut: no progress timer at all (a plain blocking write).
NeverExpired == FALSE

\* M-ZeroOnly: the timer is re-armed by every unit (an idle timer of one byte's
\* quantum): a reader that drips one unit per T_p holds the lane S * T_p.
UnitQuantum == 1

\* M-TrustCredit: a credit is believed whatever it says (no SP-8 check).
TrustAll(v) == TRUE

\* M-NoTotal (block SP1): v1 without the WB-2 total budget.
NoTotal == 0
=============================================================================
