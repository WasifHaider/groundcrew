# Three Project Ideas — Overview

Three candidate portfolio/open-core projects, compared side by side. Each follows the "frustrated by SaaS pricing → build the open-source alternative on top of public/cheap data" pattern (the OpenSEO playbook).

---

# 1. Agency Control Plane (working name: `wardctl`)

## The Idea

**Problem:** Agencies delivering work to multiple clients manage each client's environment ad hoc — manual SSH, scattered `.env` files, no source of truth for config, a manual scramble at handover time. No tool treats "a client's environment" as a first-class, trackable object.

**What it does:** An agent + control plane that tracks, provisions, and hands off isolated client environments — deployment status, secrets, access boundaries, audit trail — from one place.

**Status:** Validated concept (real problem, lived through TAKAFO.ai/Mubadala delivery work), but the original 14-week plan had real gaps — see Critique & Fixes below.

## Critique Summary

- Core assumption unvalidated: will clients actually let an agency install an agent on their server? No customer conversations in the plan.
- Agent-pull reduces credential risk but not blast radius — an agent with Docker socket access is still root-equivalent if the control plane is compromised.
- "No credentials leave your infra" is undercut by `config-store` holding encrypted secrets on the control plane, and by Cloudflare Tunnel routes needing a Cloudflare API token.
- The differentiator (handover export) is scheduled last (wks 11–14) and specified thin (JSON/PDF from state) — the weeks most likely to get cut, building the least defensible part first.
- Two backends (Go control plane + NestJS dashboard) is unnecessary complexity for a solo build.
- SQLite→Postgres migration lands in week 7, the launch-crunch week.

## Fixes Applied to This Plan

- **Validate first:** 3–5 days talking to 5 local agencies before writing code. Two questions: "walk me through your last handover" and "would your client let you install an agent on their server?"
- **Event log is the week-1 primitive.** Every heartbeat, command, config change, deploy = an append-only event. Handover export becomes a *render* of this log, so the differentiator exists from week 3.
- **Handover is a state machine**, not a report: `onboarding → active → handover-initiated → handed-over`. Initiating handover triggers secret rotation + agent credential revocation + bundle export.
- **Signed, verb-only commands.** Agent never executes arbitrary shell — only `deploy`, `restart`, `rollback`, `export`. Control plane signs commands; agent pins the control-plane key at enrollment; client can revoke locally.
- **Secrets split:** control plane holds metadata only (key names, env, last-rotated). Values live client-side, encrypted with the agent's key.
- **One backend:** Go serves the API; Vue SPA embedded in the binary. Drop NestJS.
- **SQLite for all 14 weeks**, with Litestream for backup — no mid-project migration.
- **Support both hosting modes:** agency-owned server with client-tagged environments (primary, matches most small-agency reality) and client-owned-server (premium trust option).

## Revised Timeline (14 weeks)

| Weeks | Phase |
|---|---|
| 0 (pre) | Validation: 5 agency conversations |
| 1–3 | Control plane + agent + event log + signed verb commands + CLI, dogfooded on HireFlow EC2 |
| 4–6 | Compose provisioner + agent-local secrets + engagement states |
| 7–9 | Handover state machine + bundle export — **hard launch here** |
| 10–12 | Thin Vue dashboard (embedded in Go binary) |
| 13–14 | Buffer |

## Tech Stack

| Layer | Choice |
|---|---|
| Agent / CLI | Go (single static binary) |
| Protocol | HTTPS + JSON, signed commands |
| State store | SQLite + Litestream (all 14 weeks) |
| Environment unit | Docker Compose, swappable provisioner interface |
| Secrets | Client-side encrypted (age/sops), metadata only on control plane |
| Dashboard | Vue 3, embedded in the Go binary — no separate backend |
| Ingress | Agency-owned Cloudflare account for v1 (no client credential involved) |

## Competitive Landscape

| Tool | Gap this exploits |
|---|---|
| Coolify (~57K stars) | No client/ownership boundary; servers, not clients, are the unit |
| Dokploy, CapRover | Same gap, lighter/older footprints |
| Portainer | Docker dashboard, not a delivery workflow tool |

Positioning: don't compete on feature breadth — compete on client isolation, engagement lifecycle, and a handover process that actually executes (secret rotation, revocation), not just a PDF.

## Cost to Build

Near zero. HireFlow's existing EC2 box for dogfooding; a second small instance later to test the real network boundary (~$5–6/mo VPS). Domain ~$10–15/yr. No paid APIs required.

## Verdict

3/5 as written originally; ~4/5 with validation done first and handover moved to the front. Strongest as a portfolio piece; weakest of the three as a validated product bet — the core go-to-market assumption is still unproven until step 0 is done.

---

# 2. Open Ad Intelligence Tool

## The Idea

**Problem:** Marketers and agencies need to see what competitors' ads are running, which are new, which have run longest (a proxy for "working"), and what hooks/offers they use. This data is public — Meta Ad Library, TikTok Creative Center, Google Transparency Center — but checking it manually daily is tedious. Tools like AdSpy ($149/mo flat, no trial), Foreplay ($49/mo+), Minea ($49/mo, credit-limited), and SocialPeta (custom, hundreds/mo) exist purely to organize and alert on public data.

**What it does:** Self-hosted (or cheap hosted) competitor ad watchlist — tracks new/ongoing/ended creatives, computes longevity, saves swipe files, alerts on Slack/email, exposes an MCP server + Claude Code skills for hook/angle teardown.

## Why This Category Is Open

The open-source side is fragmented: individual scrapers, a Python OSINT toolkit covering Meta/TikTok/Google/LinkedIn/Snapchat/X, a single "experimental" Slack-alerting radar, a local Meta-only research tool. No one has consolidated it into one product with the workflows the paid tools charge for — plus agent-native access, which none of the incumbents have.

## MVP (8–10 weeks)

1. **Watchlist** — add competitor pages/domains; scheduled job pulls Meta Ad Library (TikTok second).
2. **Longevity engine** — diff runs, compute days-active per creative. The single metric paid tools monetize.
3. **Swipe file + tags** — save, folder, share link.
4. **Alerts** — new creative / long-runner crosses N days → Slack/email.
5. **MCP server + Claude Code skills** — `/competitor-teardown`, `/hook-analysis`.
6. **Hosted tier** — small monthly fee, margin on data-fetch cost (same model as OpenSEO's markup on DataForSEO).

## Tech Stack

| Layer | Choice |
|---|---|
| Data source | Apify Meta Ad Library actors (BYO API key); official Meta Ad Library API for dev/fallback |
| Backend | NestJS + BullMQ (scheduled jobs, diffing) |
| DB | Postgres + Redis |
| Frontend | Vue 3 + Vite |
| Creative storage | Cloudflare R2 (free tier, no egress fees) |
| AI teardown | BYO Anthropic/OpenAI key, Haiku-class model |
| Alerts | Slack webhooks (free), Resend (free tier) |

## Competitive Landscape

| Tool | Price | Gap |
|---|---|---|
| AdSpy | $149/mo flat | No trial, dated, Facebook/Instagram only |
| Foreplay | $49/mo+ | Creative library, not raw discovery |
| Minea | $49/mo, credit-limited | Product-discovery focus, credits cap daily use |
| SocialPeta | Custom, $100s/mo | Enterprise-only pricing |

## Cost to Build

| Phase | Monthly cost |
|---|---|
| Build (dev) | $1–2 (Apify free tier: $5 credit/mo; domain amortized) |
| Public launch, self-hosted demo | $5–15 (VPS + occasional Apify overage) |
| Hosted tier at scale | Apify Starter ~$29–39 + VPS ~$6 + payment processor fees (Paddle/Lemon Squeezy — Stripe doesn't operate in Pakistan) |

Main risk: Meta can harden the Ad Library against scraping at any time — mitigated by depending on Apify's actor (their problem to fix) rather than a self-built scraper.

## Verdict

Lower validation risk than wardctl (the market already pays $49–159/mo publicly), no trust-boundary problem (nothing installs on client infra), same "priced-out → open-sourced" narrative that worked for OpenSEO.

---

# 3. Open-Source On-Call / Incident Paging Tool

## The Idea

**Problem:** Teams need on-call schedules, escalation chains, and alert routing (PagerDuty-style), but PagerDuty runs roughly $21–41/user/month, which is expensive for what is fundamentally: rotate a schedule, retry notification if unacked, escalate to the next person.

**The opening:** Grafana OnCall OSS — the most feature-complete open-source option (schedules, escalation chains, alert grouping, deep Prometheus integration) — entered maintenance mode in March 2025 and was archived March 24, 2026. Its reliance on Grafana Cloud as the SMS/phone/push relay was cut off the same day. The repo is now read-only. Every current guide in the space says the same thing: don't build new production paging on Grafana OnCall OSS. What's left — GoAlert (Go, Postgres, functional but basic/dated), OneUptime (heavy, bundles uptime+incidents+status pages) — doesn't fill the gap a modern, focused tool would.

**What it does:** Alert ingest (Alertmanager, Grafana, Sentry, Uptime Kuma webhooks) → routing rules → on-call schedules/rotations → escalation policies with timed retries → notify via Slack/Telegram/BYO-Twilio SMS/voice → ack/resolve → incident timeline → MCP server for agent queries ("who's on call for payments?", "ack incident 42").

## Distribution Advantage

A **Grafana OnCall OSS importer** is a built-in first-user channel: OnCall's HTTP API for schedules, escalation chains, and integrations is fully documented, so a migration tool can pull directly from the ~thousands of instances that got the archive notice and have nowhere to go.

## MVP (8–10 weeks)

1. Alert ingest webhook (Alertmanager/Grafana/Sentry/Uptime Kuma formats)
2. Routing rules → on-call schedule with rotations
3. Escalation policies as delayed jobs (notify → wait N min → escalate) — this maps directly to BullMQ
4. Notification channels: Slack/Telegram free; SMS/voice via BYO Twilio key (no per-seat fee)
5. Ack/resolve + incident timeline
6. MCP server + skills (`/who-is-on-call`, `/ack`, `/postmortem` draft from timeline)
7. Grafana OnCall OSS importer (schedules, escalation chains, integrations)

Later: status pages, full postmortem workflow, HA hardening.

## Tech Stack

| Layer | Choice |
|---|---|
| API | NestJS |
| Scheduling/escalation | BullMQ (delayed jobs = escalation timers, natively) |
| DB | Postgres |
| Frontend | Vue 3 + Vite |
| Notifications | Slack/Telegram (free), BYO Twilio/Vonage (SMS/voice, pay-per-use) |
| Reliability | External dead-man's-switch heartbeat check; hosted tier for teams that don't want the pager depending on their own infra |

## Competitive Landscape

| Tool | Status | Gap |
|---|---|---|
| Grafana OnCall OSS | Archived March 2026, SMS/push relay dead | Orphaned userbase, needs a landing spot |
| GoAlert | Alive, functional | Basic API ingestion, dated UI |
| OneUptime | Alive | Heavy — bundles uptime + status pages, not focused |
| PagerDuty / incident.io / Rootly | SaaS | Per-seat pricing ($21–41+/user/mo) teams explicitly want to avoid |

## Cost to Build

Near zero during build — Slack/Telegram notifications are free, Postgres/Redis/BullMQ self-hosted on existing infra. Twilio is pay-per-message/call (fractions of a cent to a few cents), only incurred in real testing, BYO-key in the OSS version so end users pay their own. VPS ~$5–6/mo for a public demo instance. Domain ~$10–15/yr.

## Honest Risk

A paging tool that goes down is worse than no paging tool — HA is the hard part, which is exactly why the incumbents argue against self-hosting. Mitigated by: single binary + Postgres, an external heartbeat/dead-man's-switch, and treating "we host it reliably for you" as the paid tier rather than trying to make self-hosted HA perfect in v1.

## Verdict

Strongest timing of the three — a real incumbent just vanished, the buyer is a developer/DevOps engineer (your own professional circle), and it reads better on a resume ("built an incident management platform") than either of the other two. Lowest cost, clearest distribution wedge (the OnCall importer), but the HA/reliability bar is real and needs to be taken seriously, not hand-waved.

---

# Side-by-Side Comparison

| | wardctl | Ad Intelligence | On-Call/Paging |
|---|---|---|---|
| Core risk | Unvalidated: will clients accept an agent? | Low: market already pays publicly | Medium: reliability bar for a paging tool |
| Trust/security complexity | High (agent = remote execution surface) | Low (read-only public data) | Medium (must not go down) |
| Build cost | ~$0 | ~$1–15/mo (Apify) | ~$0–6/mo |
| Distribution hook | None built-in — needs outbound sales | SEO/agency word-of-mouth | Grafana OnCall OSS importer (built-in orphaned userbase) |
| Buyer | Agencies (harder sell, higher trust bar) | Marketers/agencies | Developers/DevOps (your own network) |
| Resume narrative | Infra/systems depth | Data/product depth | Infra + reliability depth |
| Overall readiness | 3/5 as-is, 4/5 after validation step | 4/5 | 4.5/5 — best-timed of the three |
