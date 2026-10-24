# 0001 — AI Layer Architecture

Status: Accepted (2026-10-24), revised 2x (2026-10-24 — see Revisions)

## Context

Groundcrew's crew (`crew/`) is the AI operator layer that proposes changes to client infrastructure — it never executes anything directly. Phase 1 of the roadmap has no LLM at all; the crew starts in Phase 2 with the RELEASE operator (intent parsing, summaries) and matters most from Phase 3 onward (ON-CALL), where diagnosis quality under cheap models is the hard problem.

This is currently a personal tool run by a single developer against their own infrastructure (HireFlow's EC2 box first), so free-tier LLM providers are an acceptable starting point. The safety model established elsewhere in the project (plan → precheck → execute → verify → rollback or commit, recorded in the ledger) depends on the crew being a *proposal-only* participant: `server/` checks policy and records, `workflows/` (Temporal) executes, and the LLM process can never touch a server. The AI layer design below has to hold even when the models involved are weak, rate-limited, free-tier models whose outputs cannot be fully trusted.

## Decision

### 1. Free-tier, model-agnostic LLM layer

Providers for now: Groq, OpenRouter (free models) and Mistral (free plan), all through their OpenAI-compatible APIs. Ollama runs locally for offline eval runs.

`crew/runtime/` defines a `ModelProvider` interface with two adapters:
1. **OpenAI-compatible** (Ollama, vLLM, Groq, Cerebras, OpenRouter, Mistral), selected by base URL.
2. **Anthropic native**, added later for users who want strict tool use, prompt caching and task budgets. (Bring-your-own-key does *not* wait for this adapter — see Decision 5.)

Models and providers live in config, never hardcoded. Exact model IDs are pinned.

Embeddings use a local model through Ollama, stored in Postgres + pgvector. No separate vector database.

### 2. No agent frameworks

Do **not** add LangChain, LangGraph, LlamaIndex, CrewAI, AutoGen, or similar. Reasons:
- Temporal already owns state, retries and durability. A framework would add a second state machine.
- The safety model requires knowing exactly what prompt was sent and exactly which tools existed on every call. Framework abstractions hide that.
- Provider switching is already covered by the OpenAI-compatible adapter.
- These are large, fast-moving dependencies — a poor fit for the safety-critical core.

Groundcrew writes its own tool loop (roughly 150–200 lines). Each model call runs as a Temporal activity.

### 3. Model call routing: fallback vs. voting

These are two separate mechanisms, at two different layers.

**Fallback is transport-level.** Every model call goes through `router.call(slot, request)`. A *slot* is an ordered provider list. On a 429, 5xx, or timeout, the router tries the next provider in that slot, sequentially. Fallback is about one call surviving a bad provider — it has no opinion about correctness.

**Voting is decision-level.** It runs two slots in parallel — `primary` and `verifier` — configured with different model families (e.g. a Llama model on Groq vs. a Mistral model on Mistral). Agreement is compared on structured fields only: `diagnosis`, `playbook`, and `target` (client/env/service/server). Other params come from the primary.

- Voting fires **only** when the candidate plan targets prod **or** its playbook is marked destructive. It never fires for intent parsing, summaries, or postmortems.
- If the voters disagree → escalate via `ask_human` with both candidates.
- If a vote can't be formed (one slot exhausted, or fallback collapsed both slots onto the same model family) → continue single-voter, mark the plan `unvoted`, and cap its vote-derived autonomy at `Suggest`.

### 4. Pipeline order

Every stage below writes to the ledger (provider, model ID, prompt hash, token usage, and the stage's own outcome).

1. **Model call(s) via the router** (Decision 3).
2. **Schema validation (Zod).** On failure, feed the exact error back to the model and retry at most 2 times, then escalate.
3. **Plan linter** — deterministic, runs on **each candidate individually, before voting**. A candidate that fails lint counts as invalid, not as a vote. The linter rejects a candidate where:
   - a step doesn't relate to the triggering event
   - no rollback path exists
   - the target isn't in the affected environment
   - a step touches prod when the event didn't come from prod
4. **Vote**, if Decision 3 requires it, among lint-passing candidates only.
5. **Policy engine** computes the effective autonomy level:
   ```
   effective = min(
     policy grid level for client/env/action,
     model ceiling,
     confidence cap,
     vote cap
   )
   ```
   The model ceiling (Decision 6) is an input to this `min()`, not a separate stage. Confidence caps: `high` → no cap, `medium` → `Approve`, `low` → `Suggest`.
6. **Route by effective level:**
   - `Autonomous` → `workflows/`
   - `Approve` → approvals inbox
   - `Suggest` → posted to a human
   - `Rejected` → escalate

### 5. "The model picks, code does"

Free-tier models are weaker, so intelligence moves out of the model and into code.

1. **Playbooks first.** Each known failure mode is a deterministic playbook: `disk_full`, `crash_loop`, `oom_killed`, `cert_expiring`, `bad_deploy`, `service_down`. The model only matches the situation to a playbook, fills its params, and writes the human-readable explanation. Anything unmatched → diagnosis `unknown` → escalate to a human. Weak models never improvise.
2. **Constrained output — two schemas, one proposal type** (full shapes in Decision 7). The model never produces free-form steps.
3. **Pre-digested context, built by code, not the model** (see Decision 8 — gathering has no model involvement in v1).
4. **Narrow steps.** `classify → gather (code) → select playbook → fill params` as separate small calls with deterministic code between them. Each operator gets a hard limit of 6 steps per run.
5. **Voting** — see Decision 3.
6. **Model-aware autonomy ceiling.** The policy engine knows which provider and model produced a plan. Plans from free-tier models can never exceed `Approve` on production, whatever confidence they report. A ceiling can be raised per playbook only after evals prove that model reliable for that playbook. This feeds the `min()` in Decision 4, Step 5.
7. **Few-shot from history.** Include 2–3 similar past incidents with their correct resolutions, retrieved from the ledger or `lab/` golden examples.
8. **Deterministic plan linter** — see Decision 4, Step 3.

### 6. Model-aware autonomy ceiling (cross-reference)

See Decision 5, item 6 and Decision 4, Step 5. Stated once here for clarity since it's referenced from two places: the ceiling is a per-(provider, model, playbook) value, defaulting to `Approve` for any free-tier model on production, and is one of four inputs `min()`'d together to get the effective autonomy level — never applied in isolation.

### 7. Two output schemas, one proposal

**`IntentPlan`** — request-driven, used by RELEASE in Phase 2:

```json
{
  "intent": "deploy | rollback | create_preview | destroy_preview | restart | status | unknown",
  "client": "string",
  "environment": "string",
  "service": "string | null",
  "ref": "string | null",
  "version": "string | null",
  "confidence": "high | medium | low",
  "clarification_needed": "string | null"
}
```

Rules:
- `ref` is required for `deploy` and `create_preview`; `version` is required for `rollback`.
- Code resolves `client`/`environment`/`service` names to estate-graph IDs. If a name doesn't resolve → `clarification_needed`.
- `unknown`, or `clarification_needed` set → reply in the Slack thread with the question.
- Intent → playbook mapping is a code lookup table. **The model never picks the playbook for an intent.**
- Guardrail for intents is a **confirmation echo, not voting**: before executing, post the parsed command back (e.g. "Deploy acme/api @ feature/payments to staging?"). Prod always requires explicit human confirmation, whatever the policy grid says.

**`DiagnosisPlan`** — event-driven, used by ON-CALL in Phase 3:

```json
{
  "diagnosis": "disk_full | crash_loop | oom_killed | cert_expiring | bad_deploy | service_down | unknown",
  "evidence": ["string", "..."],
  "playbook": "string",
  "params": { "...": "validated with Zod per-playbook schema" },
  "confidence": "high | medium | low"
}
```

(`playbook` must be valid for the given `diagnosis`.)

**Both resolve, in code, to the same `ChangeProposal` type:**

```typescript
type ChangeProposal = {
  source: 'intent' | 'diagnosis';
  playbook: string;
  params: Record<string, unknown>;
  target: { client: string; environment: string; service?: string; server?: string };
  reason: string;
  confidence: 'high' | 'medium' | 'low';
  model: string;
  provider: string;
  promptHash: string;
};
```

The plan linter (Decision 4, Step 3) and the policy engine (Decision 4, Step 5) only ever see `ChangeProposal` — neither knows or cares whether it came from RELEASE's intent parsing or ON-CALL's diagnosis.

### 8. Gathering is code; v1 has no model-invoked tools

- **Gather is always code.** One evidence recipe per diagnosis family builds the evidence bundle before the first model call. Nothing in the pipeline asks a model to decide what to gather.
- **v1: every model call is pure structured output.** Input is the evidence bundle (or the Slack message plus estate context); output is JSON (`IntentPlan` or `DiagnosisPlan`). The model invokes **no tools**. `propose_plan`, `ask_human`, and `post_update` are actions *code* takes based on the model's output — not tools the model calls.
- The read functions (`queryLedger`, `searchLogs`, `queryMetrics`, `getEstate`, `runDiagnostic`) are plain functions used directly by the evidence recipes, never exposed to the model as callable tools in v1.
- **Decision 9's tool surface is the upper bound for later, not v1's shape.** Bounded follow-up reads (max 2 per run, per-operator allowlist, size-capped, redacted output) may be added only when evals show bundles are missing information the recipes can't anticipate. Each such tool would be a thin wrapper over the same plain functions above — not a new capability.

### 9. Tool surface (future upper bound — see Decision 8)

- **Read tools** (free to call, once/if added per Decision 8): `get_estate`, `query_ledger`, `query_metrics`, `search_logs`, `run_diagnostic` (read-only).
- **Write tool** (the only one, ever): `propose_plan`.
- **Communication:** `ask_human`, `post_update`.
- There is **no `execute` tool** anywhere in `crew/` — not now, not in any future bounded-tool version.

### 10. Free-tier engineering

- Router slots as defined in Decision 3 (fallback chains per slot, e.g. Groq → OpenRouter → Mistral for a single-model slot).
- A token-bucket rate limiter per provider. Temporal retries with exponential backoff on 429.
- A response cache keyed by a hash of the prompt.
- OpenRouter's free model list rotates. If a pinned model disappears, fall to the next provider in the slot and trigger an eval rerun before trusting the replacement.
- Every ledger entry the crew produces records provider, model ID, prompt hash, and token usage (per Decision 4's "every stage writes to the ledger").

### 11. Bring-your-own-model ordering

- **BYOK does not require the Anthropic-native adapter.** The OpenAI-compatible adapter plus per-client model config (base URL, model, API key stored in the secrets module) covers paid OpenAI-compatible providers and a client's own Ollama/vLLM, independent of whether the Anthropic adapter has shipped.
- **Hard prerequisite before any external client is adopted:**
  - per-client model config
  - BYOK key storage (in the secrets module)
  - `allowFreeTierModels` enforcement (default `false`)
  This is a gate before Phase 6 (the agency layer) in the roadmap — see `groundcrew-project-overview.md`, section 12.
- **The Anthropic-native adapter is optional**, only for clients who want Claude specifically. It is never on the path required for BYOK or for client adoption.

### 12. Data and security

- Redaction runs before anything leaves a server: API keys, tokens, passwords, database URLs, emails. The agent's `redact/` module owns this. Evidence bundles (Decision 8) are built from already-redacted telemetry — redaction happens at the source, not at bundle-build time.
- Free tiers may use prompts to improve the provider's products. This is enforced in code, not by convention: each client has a model config with `allowFreeTierModels`, which defaults to `false`. Only the operator's own estate and `lab/` get `true`. A client must be on bring-your-own-key (Decision 11), a local model, or a paid tier before adoption.
- Prompt injection: logs contain attacker-controlled text. All tool output wrapped into prompts is clearly marked as untrusted data. The plan linter (Decision 4, Step 3) is the second line of defense.

### 13. Evals gate everything

- Every playbook gets `lab/` scenarios (docker-in-docker fake servers with injected faults).
- A model is trusted for a playbook only after it proposes the correct plan in at least 19 of 20 runs with zero unsafe steps.
- Evals rerun on every prompt change, model change, or provider/slot fallback configuration change.

### Phasing

- **Phase 1:** no LLM.
- **Phase 2:** RELEASE operator. `IntentPlan` (Decision 7), confirmation-echo guardrail, no voting (intent parsing never votes — Decision 3). Small models handle this well.
- **Phase 3:** ON-CALL operator. `DiagnosisPlan` (Decision 7), full pipeline (Decision 4) including voting for prod/destructive candidates.
- **Build order for the AI layer, when Phase 2 starts:**
  1. Provider router (Decision 3's fallback slots)
  2. `disk_full` playbook
  3. `lab/` disk-full scenario
  4. Eval harness, run 20× across all three providers

## Consequences

- **No crew code ships before Phase 2.** Phase 1 (control plane skeleton, agent, discovery) proceeds with zero AI dependency, exactly as already planned.
- **Every model call is auditable and replayable.** Because there's no framework and no model-invoked tools in v1, any plan the crew ever produced can be traced to an exact prompt, exact input bundle, and exact model/provider/promptHash — a hard requirement for a safety-critical system, at the cost of writing and maintaining the tool loop, router, and schemas by hand instead of importing them.
- **Weak models become viable** because correctness is pushed into playbooks, schemas, the linter, and now an explicit pipeline order (Decision 4) rather than trusted to model judgment. An `unknown`/`clarification_needed` result is a feature (escalate), not a gap to paper over with a smarter prompt.
- **Voting only applies where it matters.** Restricting it to prod/destructive candidates (Decision 3) avoids doubling cost and latency on the common case (staging restarts, intent parsing, summaries), while still catching single-model hallucination exactly where a wrong plan is most expensive.
- **Two schemas, one proposal type keeps the linter and policy engine source-agnostic.** `ChangeProposal` (Decision 7) means RELEASE and ON-CALL can evolve independently without ever touching the safety-critical code that lints and routes plans.
- **Free-tier operation is a real constraint on day one**, not an afterthought: the router's fallback slots, rate limiter, response cache, and the `allowFreeTierModels` flag all need to exist before the crew can run at all, even in Phase 2's simplest form.
- **Evals become a release gate**, not a nice-to-have: no model/prompt/provider/slot change ships without a passing eval run. This adds process overhead to every crew change but is what makes "trust a cheap model enough to raise its autonomy ceiling" a defensible, provable claim rather than a vibe.
- **BYOK is available from Phase 2**, not gated on the Anthropic adapter (Decision 11) — a client willing to supply their own OpenAI-compatible-API key can be onboarded without waiting for that adapter to ship.
- **Phase 6 now has an explicit AI-layer gate**, not just the client/portal/reporting work already on the roadmap: per-client model config, BYOK key storage, and `allowFreeTierModels` enforcement must exist before any external client (anyone other than the operator's own estate or `lab/`) is adopted.

## Alternatives rejected

- **LangChain / LangGraph.** Rejected per Decision 2: duplicates Temporal's state/retry/durability ownership, hides exactly which prompt and tools were used on each call (unacceptable for the safety model), and is a large, fast-moving dependency surface for a safety-critical core. Provider abstraction, the main thing these would buy, is already covered by the OpenAI-compatible adapter.
- **Claude Managed Agents / other hosted agent runtimes.** Rejected for the same reason as frameworks: they run the tool loop on infrastructure Groundcrew doesn't control, which conflicts with "the LLM process can never touch a server directly" and with owning exact prompt/tool provenance for every call. Also presently paid-only, conflicting with the free-tier-first starting point. Revisit only for bring-your-own-key users who explicitly want it, as a provider adapter — never as the default runtime.
- **Paid-API-only (e.g., Anthropic or OpenAI from day one).** Rejected because this is currently a personal tool with no revenue; free-tier providers make the AI layer viable to build and dogfood immediately. The Anthropic-native adapter is deferred, not abandoned — it arrives later for users who want its specific features (strict tool use, prompt caching, task budgets), but BYOK itself does not wait on it (Decision 11).
- **Free-form agent loops (model decides arbitrary next steps).** Rejected per Decision 5: free-tier models are not reliable enough to improvise multi-step plans safely. Constraining the model to playbook/intent selection + param filling, with deterministic code everywhere else (including gathering — Decision 8), is the entire point of "the model picks, code does."
- **Model-invoked tool calling in v1 (model decides what to read).** Considered and deferred per Decision 8: giving the model read tools now would reintroduce exactly the "framework hides what happened" problem Decision 2 rejects frameworks for, before there's eval evidence that pre-digested bundles are ever insufficient. Revisit only with evidence, as a tightly bounded, size-capped, redacted extension — never as open-ended tool use.

## Revisions

- **2026-10-24 — R5 completed** (the first pass had an incomplete R5, cut off mid-sentence): BYOK key storage confirmed as the secrets module; added the hard prerequisite (per-client model config, BYOK key storage, `allowFreeTierModels` enforcement) as an explicit gate before Phase 6; confirmed the Anthropic-native adapter is optional, Claude-specific, and never a blocker for BYOK or client adoption.
- **2026-10-24 — R1–R5 applied**, resolving the open questions from this ADR's first draft:
  - **R1** (→ Decision 3): split fallback (transport-level, sequential, per-slot) from voting (decision-level, parallel primary/verifier slots, structured-field agreement only), scoped voting to prod/destructive candidates, and defined the unvoted/degraded-vote case.
  - **R2** (→ Decision 4): made the pipeline order explicit — model call → schema validation → per-candidate linting → voting (lint-passers only) → policy `min()` → route by effective level — with ledger writes at every stage.
  - **R3** (→ Decision 7): introduced `IntentPlan` (RELEASE, Phase 2) alongside the existing `DiagnosisPlan` (ON-CALL, Phase 3), unified in code to a single `ChangeProposal` type that the linter and policy engine consume, source-agnostic. Added the confirmation-echo guardrail for intents.
  - **R4** (→ Decision 8, Decision 9 demoted to "future upper bound"): confirmed gathering is 100% code in v1, and that the model calls no tools at all in v1 — `propose_plan`/`ask_human`/`post_update` are code actions on the model's structured output, not model-invoked tools. The original tool-surface decision becomes a bounded future extension, gated on eval evidence.
  - **R5** (→ Decision 11): confirmed BYOK does not wait on the Anthropic-native adapter — the OpenAI-compatible adapter plus per-client config (API key stored in the secrets module) is sufficient. Named the hard prerequisite for adopting any external client (per-client model config, BYOK key storage, `allowFreeTierModels` enforcement) as a gate before Phase 6 in the roadmap. The Anthropic-native adapter is confirmed optional, for clients who specifically want Claude.
