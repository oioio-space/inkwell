---
name: new-gate
description: Use when starting a block (lot, protocol step, tooling lot) — scaffolds the frozen gate gates/<bloc>.toml and the /simplify report gates/<bloc>/SIMPLIFY.md in the shape `mise run done` expects, before any code. User-invoked.
disable-model-invocation: true
---

# New gate for a block

Step 1 of the definition of « fini » (CLAUDE.md): the gate is written and **committed before the code**,
then frozen (`.claude/hooks/gate-freeze.sh` refuses any later edit). Argument: the block name, e.g. `P4`.

1. Read the block's plan in `docs/plans/` and the requirements it closes (`docs/exigences.md`).
   Every criterion is a machine-checkable command; no criterion is "looks fine".
2. Write `gates/<bloc>.toml`, following an existing gate (`gates/P3.toml`):

```toml
# Frozen acceptance for <block> (<one line>), written before implementation.
# Plan: docs/plans/<date>-<block>-<topic>.md. Closes <requirement ids>.
raw_results = ["docs/plans/<plan>.md", "docs/objectif-en-cours.md", "gates/<bloc>/*"]

[[criteria]]
# <id>: what it proves, which spec rule, why this command proves it.
id = "<id>"
command = "<command; tests go through ../scripts/heavy.sh or a mise task>"

[[criteria]]
id = "<previous-block>-regression"
command = "mise run gate -- <previous block>"
```

   - Spec first: vectors (`protocol/vectors`), TLA+/Alloy models with mutants (agent `formal-modeler`),
     model-based tests — the gate runs them.
   - Every test command is memory-capped (`heavy.sh` or a mise task), or `heavy-guard.sh` refuses it.
   - Keep the regression criterion on the previous block.
3. Create `gates/<bloc>/SIMPLIFY.md` with this skeleton (filled at step 3, after implementation;
   `mise run done` requires it newer than every code change, and a `mutation <package>` section for
   any `protocol/*` package below 95 %):

```markdown
# /simplify — <bloc>

Diff : <base>..HEAD (<n> fichiers). Relecteurs : <who, model/effort>.

## Appliqué
## Écarté (avec la raison)
## mutation <package> (si < 95 % : survivants équivalents, un par ligne, avec la preuve)
## Vérifié
```

4. Show the gate to the user, then commit it alone (`test(gates): freeze <bloc> acceptance`) before
   any implementation commit — `mise run done` step 1 checks that order.
