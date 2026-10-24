# CLAUDE.md

Project-specific rules for working in this repo. See also [docs/decisions/0001-ai-layer-architecture.md](docs/decisions/0001-ai-layer-architecture.md) for the full reasoning behind the AI layer rules.

## Commits

- Commit author for this repo: `WasifHaider <mwasifhaider1@gmail.com>`.
- Never run `git commit` after finishing implementation work. Stage changes (`git add`) and stop — the user commits themselves, in detail.

## AI layer rules (`crew/`)

Non-negotiables for anything touching the AI operator layer. Phase 1 has no LLM at all — these apply starting Phase 2. Full reasoning and the exact pipeline/schemas are in the ADR; this is the quick-reference version.

- **No agent frameworks.** No LangChain, LangGraph, LlamaIndex, CrewAI, AutoGen, or similar. Write the tool loop ourselves.
- **No `execute` tool, ever.** `crew/` can only propose (`propose_plan`, `ask_human`, `post_update`), never act on a server — not now, not in any future tool-surface extension.
- **v1 has no model-invoked tools.** Gathering is always code (one evidence recipe per diagnosis family); the model only ever sees a pre-built bundle and returns structured JSON. `propose_plan`/`ask_human`/`post_update` are code actions taken on that output, not tools the model calls.
- **Playbook + schema output only, via two schemas → one `ChangeProposal`.** RELEASE (Phase 2) emits `IntentPlan`; ON-CALL (Phase 3) emits `DiagnosisPlan`. Both resolve in code to `ChangeProposal`, which is all the linter and policy engine ever see. Unmatched/unresolvable situations → `unknown` / `clarification_needed` → escalate, never improvised. The model never picks the playbook for an intent — that's a code lookup table.
- **Fallback ≠ voting.** Fallback is transport-level and sequential within a router slot (bad provider → next provider). Voting is decision-level: two parallel slots (primary/verifier, different model families), compared on `diagnosis`/`playbook`/`target` only, and fires **only** for prod-targeting or destructive-playbook candidates. Disagreement → escalate with both candidates. A degraded (unvoted) vote caps autonomy at `Suggest`.
- **Pipeline order is fixed:** model call(s) → Zod validation (≤2 retries, then escalate) → plan linter (per candidate, before voting) → vote (lint-passers only, when required) → policy `min(policy grid, model ceiling, confidence cap, vote cap)` → route by effective level. Every stage writes to the ledger.
- **Redaction first.** Secrets (API keys, tokens, passwords, DB URLs, emails) are stripped by the agent's `redact/` module before anything leaves a server — evidence bundles are built from already-redacted telemetry, never redacted after the fact.
- **`allowFreeTierModels` defaults to `false`.** Only the operator's own estate and `lab/` may set it `true`. A client needs bring-your-own-key (OpenAI-compatible adapter + per-client config, API key in the secrets module — does **not** require the Anthropic-native adapter, which is optional and Claude-specific), a local model, or a paid tier before adoption.
- **Phase 6 gate — before adopting any external client:** per-client model config, BYOK key storage (secrets module), and `allowFreeTierModels` enforcement must all exist first.
- **Autonomy ceiling per model is one input to a `min()`, never applied alone.** Free-tier-model plans can never exceed "Approve" on production, regardless of reported confidence. Ceilings rise per playbook only after evals prove that model reliable for it.
- **Evals gate every model, prompt, or provider/slot change.** No change ships without a passing eval run (≥19/20 correct, zero unsafe steps) in `lab/`.
- **One Temporal activity per model call.** Every LLM call is its own Temporal activity — no model call happens outside Temporal's retry/durability boundary.
