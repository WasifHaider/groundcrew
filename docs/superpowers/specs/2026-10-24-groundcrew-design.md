# Groundcrew — Design Decisions (2026-10-24)

Full product spec lives in [groundcrew-project-overview.md](../../../groundcrew-project-overview.md) (architecture, safety model, tech stack, folder structure, 6-phase roadmap, screens). This file resolves the open questions from that doc's section 14 and records brainstorming decisions before implementation planning starts.

## Purpose of this project

Portfolio/resume piece for a solo developer. Goal: a finished, end-to-end system demonstrating real architecture and engineering decisions (agents, control plane, multi-tenant isolation, durable workflows, agentic AI layer) — not a CRUD app. "Finished" matters, but scope is the full 6-phase roadmap, built incrementally, with no fixed deadline.

## Resolved open questions

- **Customer:** both — a solo developer running their own projects, and an agency running multiple clients. The agency/multi-tenant model (client → project → environment) is built from day one since it's a superset: a solo dev is just an agency with one client. No architecture change needed versus the original doc; this only changes framing, not design.
- **Open-core split (free vs. paid):** deferred. Not deciding monetization now — irrelevant to a portfolio build. Revisit if/when this ever needs a business model.
- **Scope:** build toward the full 6-phase roadmap in the overview doc (Hands/memory/discovery → Release → On-call → Hygiene → Platform/FinOps → Agency layer). No tighter MVP cut — timeline is open-ended, so the existing phased roadmap stands as-is.
- **Timeline:** no fixed deadline. Pace note in the overview doc (3-4hr weekdays, 5+hr weekends) is a guideline, not a constraint.

## Repo state

The original `agencyctl`-era files (`cmd/controlplane/main.go`, `go.mod`, old `project-overview.md`, `three-project-ideas.md`) are being removed in favor of a clean start under the Groundcrew name and architecture. This is a repo-hygiene action, not a design decision — tracked separately, not blocking this spec.

## Everything else

Unchanged from `groundcrew-project-overview.md`: architecture (Hands/Memory/Brain/Leash/Voice), safety model, tech stack (Go agent/CLI, NestJS+Prisma+Postgres control plane, Temporal workflows, Claude Agent SDK crew, Vue 3 web), folder structure, and the Phase 1 build order (control plane skeleton → agent heartbeat → discovery → command dispatch → CLI → read-only web → adopt HireFlow EC2).

Next step after this spec is approved: invoke writing-plans for Phase 1, step 1 (control plane skeleton: NestJS + Prisma + Postgres, register-agent/heartbeat endpoints).
