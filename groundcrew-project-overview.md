# Groundcrew — Project Overview

**An AI DevOps team for software agencies.**
*"The ops team your agency never hired."*

Status: not started. Scope redesigned in October 2026 from the original agency control-plane idea (working names `agencyctl` / `wardctl`).

---

## 1. The Idea

### The problem
Software agencies delivering work to multiple clients (e.g. Aslase/Inception delivering TAKAFO.ai for Mubadala) manage each client's environment ad hoc: manual SSH sessions, scattered `.env` files, no single source of truth for which client has which config, and a scramble to write documentation at handover time. Most agencies can't afford a dedicated DevOps team, so this work falls on developers who'd rather be building.

### The redesign
The original plan was a **tool that DevOps engineers use**: an agent plus a control plane to spin up, track and hand off client environments.

Groundcrew is **the DevOps team itself**. A crew of AI operators runs every client's infrastructure: deploys, incidents, certificates, backups, costs and handovers. They act within policies the agency sets. The agency's humans move into the head-of-ops seat: they set policy, approve risky actions, and read the reports.

### Why this project
- A real problem lived firsthand through client delivery work, not speculative
- A legitimate open-core SaaS shape (free engine/CLI, paid hosting/dashboard)
- Forces building real infrastructure (agents, control planes, multi-tenant isolation, durable workflows) plus a real agentic AI layer, which is a far stronger portfolio signal than another CRUD app
- Opens a business model where agencies sell managed-ops retainers to their clients, powered by Groundcrew

---

## 2. What It Replaces — The Six Jobs of a DevOps Team

Each job becomes an AI operator ("crew member") in the product.

| Operator | Job it replaces | Responsibilities |
|---|---|---|
| **RELEASE** | Release engineer | Builds, deploys, staging and preview environments, rollbacks |
| **ON-CALL** | On-call SRE | Detects breakages, diagnoses root cause, remediates, writes postmortems |
| **PLATFORM** | Platform engineer | Servers, DNS, tunnels, databases, buckets, provisioning |
| **HYGIENE** | Security & maintenance | Certificates, OS/image patches, CVEs, secret rotation, access reviews, backups and restore drills |
| **FINOPS** | Cost management | Idle resources, oversized servers, cost per client |
| **DESK** | Ops help desk | Developer requests ("give Ali access to acme staging logs", "why is staging broken?"), handover docs |

---

## 3. Architecture — Five Layers

### Hands — the agent (Go)
A small binary installed on each client server with a one-line script. It phones home over outbound HTTPS and exposes a **fixed set of typed capabilities**, and nothing else:

`deploy` · `rollback` · `restart` · `scale` · `snapshot` · `restore` · `renew-cert` · `rotate-secret` · `run-diagnostic`

The AI never gets a raw shell. Diagnostics run in a read-only sandbox (logs, `docker inspect`, disk and network checks). The agent also collects telemetry, runs brownfield discovery, and **redacts secrets** before anything leaves the server.

### Memory — the control plane
- **Estate graph:** client → project → environment → service → server → cloud resource
- **Desired state:** `groundcrew.yaml` is the contract for what should be running
- **Change ledger:** every action ever taken, who took it (human or AI), why, and what happened afterward

### Brain — the crew
Each operator is a set of **watchers** (signals it subscribes to), **playbooks**, and the **capabilities it is allowed to call**. The LLM plans; a deterministic engine executes. The model proposes "roll back acme-api to v41," but execution, verification and rollback are plain code running as durable workflows.

### Leash — policy and autonomy
Autonomy is set **per client, per environment, per action**:

| Level | Meaning | Example |
|---|---|---|
| **Observe** | Crew watches and reports only | New client, first two weeks |
| **Suggest** | Crew proposes, humans execute | Scaling decisions on prod |
| **Approve** | Crew executes after a human approves | Running a migration on prod |
| **Autonomous** | Crew acts alone, reports after | Restarting a crashed container on staging |

Some combinations are permanently locked (e.g. "delete resource" on prod can never be Autonomous).

### Voice — interfaces
- **Chat first:** Slack / Teams (WhatsApp later) — "spin up staging for feature/payments on client-b"
- **CLI:** the `groundcrew` command for power users
- **Web dashboard:** the whole picture, reviews and approvals
- **Client portal:** a limited, agency-branded view for the agency's clients

### How a change flows

```
Crew (AI operator)       proposes a plan
        │
        ▼
Control plane            checks policy, gets approval if needed, records it in the ledger
        │
        ▼
Workflows                executes deterministically: precheck → execute → verify → rollback or commit
        │
        ▼
Agent (client server)    pulls typed commands over outbound HTTPS, runs them, reports results
```

---

## 4. Safety Model

The safety model is the product's real core.

1. **Every change follows one loop:** plan → precheck → execute → verify against health signals → auto-rollback or commit → write to ledger.
2. **Typed capabilities only.** The crew can only call the agent's fixed capability set. No arbitrary commands.
3. **Separate processes as the safety boundary.** `crew/` proposes, `server/` checks policy and records, `workflows/` executes. The LLM process can never touch a server directly; if it crashes or hallucinates, the worst it can do is submit a bad plan that policy rejects.
4. **No credentials leave client infrastructure.** Agent-pull model: the control plane never holds SSH keys or cloud credentials. Cloud provisioning uses the server's own instance role / OIDC.
5. **Secrets never reach the model.** The agent scrubs logs and environment variables before sending context upstream.
6. **Trust is earned in order.** The product's phases go observe → advise → act, so the crew watches an estate for weeks before touching it.

---

## 5. Key Features

### Estate discovery and adoption (brownfield support)
Agencies don't start from clean servers. A typical agency has client apps running on servers set up by hand years ago, by developers who may have left: hand-edited nginx configs, cron jobs for backups, `.env` files on disk, containers started manually, certbot SSL. If Groundcrew only worked on fresh servers, adopting it would mean migrating every live production app first, so nobody would adopt it.

Two entry paths:
- **Greenfield (fresh server):** install the agent, write `groundcrew.yaml`, Groundcrew sets everything up.
- **Brownfield (existing server):** install the agent; it scans read-only (containers, systemd units, nginx/Caddy configs, cron jobs, open ports, certificates, volumes), shows a map and a draft `groundcrew.yaml`, and a human confirms. The server is then "adopted." **Adoption changes nothing on the server.** From then on, all changes go through Groundcrew.

The discovery output doubles as handover documentation from day one. HireFlow's EC2 box (4 containers via Docker Compose plus a hand-built Cloudflare tunnel) is itself a brownfield case and the first adoption target.

### "What changed?" as the incident engine
The first question in nearly every incident is what changed recently. Every deploy, config edit and secret rotation goes through the ledger, so the ON-CALL operator answers that instantly. Tools that bolt onto someone else's pipeline have to reconstruct it.

### Hygiene autopilot
The boring work that actually causes agency outages: expiring certificates, filling disks, unpatched images, and backups nobody has ever restored. **Weekly restore drills into a throwaway environment** are something almost no tool offers.

### The agency layer
- Per-client isolation as a first-class concept
- Cost per client
- Monthly ops reports to send to clients
- A handover package that is always ready, not written in a scramble at project end
- **Managed-ops retainer mode:** agencies sell ongoing operations to their clients, run by Groundcrew. This makes Groundcrew a revenue line for the buyer rather than a cost.

---

## 6. Market and Positioning

### AI SRE / AI ops tools (crowded, enterprise-focused)
The AI-for-ops space is crowded: one buyer's guide counts 40+ vendors across 7 archetypes, from full-platform incumbents (Dynatrace, Datadog, New Relic) to pure-play AI SRE startups and cloud-native agents from AWS and Azure.

| Tool | Focus |
|---|---|
| Resolve.ai, Cleric, Traversal | Autonomous incident investigation from logs, metrics and traces; moving from read-only toward remediation. Resolve is described as best for organizations with homogenous tooling and strong instrumentation. |
| Komodor (Klaudia) | Multi-agent autonomous remediation specialized for Kubernetes |
| New Relic SRE Agent | Incident triage, change management, root cause analysis inside New Relic (launched Feb 2026) |
| PagerDuty | Incident management + AIOps, with a newer SRE agent |
| Better Stack | All-in-one observability + incident response with built-in AI SRE, aimed at startups and mid-size teams |
| DuploCloud | Positions an "AI DevOps Engineer" as a stateful, always-running operational layer |
| DataAgent | Autonomous remediation platform; raised $10M pre-seed in Sept 2026 |

Practitioner consensus: fully autonomous remediation should stay narrow (restart a pod, revert a specific deploy) with tight guardrails.

### Self-hosted PaaS / deployment control planes (carried over from original research)
| Tool | Model | Strength | Gap |
|---|---|---|---|
| Coolify | Self-hosted, Docker/Compose, SSH-push, ~57K GitHub stars | Most feature-rich, polished UI, 280+ one-click services | No environment-lifecycle/handover concept; treats servers as infrastructure you own |
| Dokploy | Docker Swarm + Traefik, ~35K stars | Lightest footprint, native Compose support | No client/ownership boundary |
| CapRover | Docker Swarm, since 2017, ~15K stars | Stable, large app store | Limited Compose support, slowing development |
| Portainer | Docker/K8s GUI | Solid for internal server management | Not agency/client-oriented |

Adjacent: Laravel Forge, RunCloud, SpinupWP, GridPane (framework-specific), DeployHQ (Git-triggered deploys), Deploynix (managed Laravel/PHP).

*(Star counts from the original research session; re-verify before citing publicly.)*

### Positioning
The AI SRE tools assume an enterprise that already has Datadog, Kubernetes and an SRE team to augment. The PaaS tools think in apps and servers, not clients and handovers.

Groundcrew's customer has none of that: **a 15-person agency running 12 clients on EC2/VPS boxes with Docker Compose and no observability.** Groundcrew isn't "AI SRE for your existing stack." It's "the ops team you never hired": it brings its own telemetry, treats the client as a first-class boundary, and owns the full lifecycle from adoption to handover.

Don't compete on feature breadth or integrations. Compete on being the agency's entire ops function.

---

## 7. Key Design Decisions

### Agent-pull, not SSH-push (informed by studying Coolify)
| | Coolify (SSH-push) | Groundcrew (agent-pull) |
|---|---|---|
| How control plane reaches servers | Holds SSH private keys, connects outward | Agent installed on client server, phones home |
| Where credentials live | Control plane holds keys to every server | Control plane never holds client SSH/cloud creds |
| Trust direction | Control plane reaches into client infra | Client infra reaches out to control plane |
| Network requirement | Server must accept inbound SSH | Server only needs outbound internet |

Coolify's model works for one operator managing their own servers. For a multi-client agency, asking every client to grant an outside vendor inbound SSH is a much harder trust ask. With AI acting on production, agent-pull matters even more.

### Onboarding and auth
- **Agent → control plane (machine identity):** one-time enrollment token → install script → agent exchanges it for a long-lived credential on first boot.
- **Developer → control plane (human identity):** API key in the local CLI config for MVP; SSO and fine-grained roles later.

### Provisioner abstraction
Docker Compose is the only implementation at first, behind a swappable provisioner interface. OpenTofu is added later as a second provisioner for cloud resources. Kubernetes stays out of scope until a real client needs it, but the interface keeps that door open.

### URL assignment
Dynamic Cloudflare Tunnel routes for preview/staging environments, reusing the pattern hand-built for HireFlow's `api.whapps.site`. No public IP or client firewall changes needed.

### LLM plans, deterministic code executes
The crew produces plans; Temporal workflows execute, verify and roll back. Non-determinism stays in planning, never in execution.

### Go at the edge, TypeScript at the center
Anything running on a client server or developer laptop is Go: a single static binary with no runtime. Everything server-side is TypeScript, where NestJS/Prisma depth makes development fastest and where the Claude Agent SDK and Temporal both have solid SDKs. Two languages, one clear boundary.

### Web-first frontend
Considered a desktop-first app and rejected it as the primary surface:
- The control plane can't live on a laptop: agents need an always-on endpoint, and the crew must act at 2am when the laptop is closed. A desktop app would just be another window onto the server.
- Clients won't install software to see their status, and 2am approvals happen on a phone.
- Desktop distribution (code signing, notarization, SmartScreen, updates) slows adoption; web trials are just a link.
- It's a team product; everyone needs the same live view.

**Plan:** web as the main app → PWA for mobile approvals → optional desktop companion later by wrapping the same Vue app in **Tauri** (tray status, native notifications, global hotkey to approve). One frontend codebase for all three.

### Dogfooding target
HireFlow's existing EC2 instance (ap-south-1; backend, frontend, ml-service, redis via Docker Compose). Leaning toward running the control plane on a separate small instance so the agent crosses a real network boundary.

---

## 8. Tech Stack

| Layer | Choice | Why |
|---|---|---|
| Agent + CLI | **Go** | Single static binary, no runtime dependency on client servers, matches `kubectl`/`terraform` conventions |
| Contracts | **Protobuf + buf**, served via **ConnectRPC** | One definition generates Go and TS types; still runs over plain HTTPS + JSON |
| Control plane | **NestJS + Prisma + PostgreSQL** | Strongest existing stack; Postgres from day one since the estate graph and ledger are relational |
| Workflows | **Temporal** (TypeScript SDK) | Durable plan → execute → verify → rollback workflows |
| Crew | **TypeScript + Claude Agent SDK** | Agent loop with tools restricted to typed capabilities |
| Telemetry | **OpenTelemetry collector** in the agent → **VictoriaMetrics** (metrics) + **Loki** (logs) | Brings its own observability to clients who have none |
| Environments | **Docker Compose** via provisioner interface; **OpenTofu** later | Known deeply from HireFlow; swappable |
| Secrets | Encrypted-at-rest config store | Full Vault is overkill for v1 |
| URLs | **Cloudflare Tunnel** dynamic routes | No public IP or firewall config |
| Web | **Vue 3 + Vite**, PWA | Reuses HireFlow patterns; mobile approvals |
| Desktop (later) | **Tauri** | Small binary, wraps the same Vue app |
| Chat | Slack / Teams gateway | Chat-first ops |
| Monorepo | `go.work` + pnpm workspaces | Go and TS side by side |
| Build approach | Claude Code / Cursor, agents-first, with project skills in `.claude/skills/` | |

---

## 9. Modules

**Agent (hands):** enrollment, link (heartbeat + command pull), capabilities, provisioner, discovery, telemetry, redaction, diagnostic sandbox, installer.

**CLI:** `adopt`, `env`, `deploy`, `status`, `logs`, `approve`, `handover`.

**Control plane (memory + leash):** identity, tenancy, fleet, estate, manifest, ledger, policy, approvals, secrets, access, signals, incidents, reports, notifications.

**Workflows (executor):** change lifecycle, adoption, preview environments, drills (restore drills, cert renewals).

**Crew (brain):** runtime, context builder, memory (incident/runbook retrieval), the six operators, evals.

**Interfaces:** chat gateway, web dashboard, client portal.

**Testing:** lab of fake client servers and broken scenarios.

---

## 10. Folder Structure

```
groundcrew/
├── agent/                        # Go: runs on client servers
│   ├── cmd/groundcrew-agent/
│   ├── internal/
│   │   ├── enroll/               # one-time token → long-lived identity
│   │   ├── link/                 # heartbeat, pulls commands, sends results
│   │   ├── capabilities/         # deploy, rollback, restart, scale, snapshot,
│   │   │                         # restore, renew-cert, diagnose… nothing else
│   │   ├── provisioner/          # interface + compose/ (opentofu/ later)
│   │   ├── discovery/            # brownfield scan: containers, units, nginx,
│   │   │                         # crons, ports, certs, volumes
│   │   ├── telemetry/            # embedded OTel collector + shipping
│   │   ├── redact/               # scrubs secrets before anything leaves the box
│   │   └── sandbox/              # read-only diagnostic runner
│   └── install/install.sh        # curl | sh installer + systemd unit
│
├── cli/                          # Go: the `groundcrew` command
│   ├── cmd/groundcrew/
│   └── internal/commands/        # adopt, env, deploy, status, logs,
│                                 # approve, handover
│
├── proto/                        # contracts shared by Go and TS (buf)
│   ├── agent/v1/                 # enroll, heartbeat, command, result
│   └── capability/v1/            # typed capability requests/results
│
├── server/                       # TypeScript (NestJS): control plane
│   ├── src/modules/
│   │   ├── identity/             # users, API keys (SSO later)
│   │   ├── tenancy/              # agency → clients → projects
│   │   ├── fleet/                # servers, agents, enrollment, last-seen
│   │   ├── estate/               # infra graph: env → service → server → resource
│   │   ├── manifest/             # groundcrew.yaml, desired state, drift
│   │   ├── ledger/               # every change: who, what, why, outcome
│   │   ├── policy/               # autonomy level per client/env/action
│   │   ├── approvals/            # approve/deny from chat, web, phone
│   │   ├── secrets/              # encrypted config store
│   │   ├── access/               # who can touch which client
│   │   ├── signals/              # telemetry queries, alerts in
│   │   ├── incidents/            # incident records, postmortems
│   │   ├── reports/              # monthly client ops reports, handover export
│   │   └── notifications/
│   └── prisma/schema.prisma
│
├── workflows/                    # TypeScript (Temporal workers)
│   └── src/
│       ├── change/               # plan → precheck → execute → verify
│       │                         # → rollback or commit → record
│       ├── adoption/             # discovery → draft manifest → human confirms
│       ├── preview-envs/         # create/destroy + Cloudflare tunnel routes
│       └── drills/               # backup restore drills, cert renewals
│
├── crew/                         # TypeScript: the AI operators
│   └── src/
│       ├── runtime/              # agent loop; tools = capabilities only
│       ├── context/              # builds redacted context from estate,
│       │                         # ledger and signals
│       ├── memory/               # past incidents, runbooks (retrieval)
│       ├── operators/
│       │   ├── release/
│       │   ├── oncall/
│       │   ├── platform/
│       │   ├── hygiene/
│       │   ├── finops/
│       │   └── desk/
│       └── evals/                # replay recorded incidents, score decisions
│
├── gateways/chat/                # Slack/Teams ↔ crew + approvals
├── web/                          # Vue 3: dashboard + client portal
│
├── lab/                          # fake client servers for e2e tests + evals
│   ├── machines/                 # docker-in-docker "client boxes"
│   └── scenarios/                # disk-full, crash-loop, bad-deploy,
│                                 # cert-expiring, failed-backup…
│
├── deploy/                       # self-host: Postgres, Temporal,
│                                 # VictoriaMetrics, Loki via compose
├── packages/                     # shared TS: generated types, manifest schema
├── examples/                     # sample groundcrew.yaml (HireFlow first)
├── docs/
├── .claude/skills/               # project skills for building with Claude Code
├── go.work
└── pnpm-workspace.yaml
```

**Notes**
- `lab/` is the most underrated folder. You can't test an AI ops team on real client servers. Lab scenarios serve as both end-to-end tests and the eval suite proving each operator makes the right call.
- Don't scaffold everything on day one. Create folders when their phase arrives so the repo shows real progress, not empty directories.

---

## 11. Frontend

Day-to-day work happens in chat and the CLI. The web app is for **seeing the whole picture, reviewing before trusting the crew, and showing clients what's happening.**

### Screens
| Screen | Purpose |
|---|---|
| **Estate map** | Visual graph of every client, environment, service and server with health on top. Also the main demo screen. |
| **Adoption review** | Review what discovery found on an existing server and the draft `groundcrew.yaml` before confirming. |
| **Approvals inbox** | Pending crew actions with plan, reason, blast radius, verification criteria and rollback path. One tap to approve or deny. Must work on a phone. |
| **Change ledger** | Timeline of everything that happened per client, filterable by human or AI. The audit trail. |
| **Incident view** | What broke, the evidence gathered, what the crew did, the auto-written postmortem. |
| **Autonomy policy** | Grid of clients × environments × actions with Observe / Suggest / Approve / Autonomous per cell. |
| **Client portal** | Agency-branded view for clients: status, uptime, monthly reports, handover package. |

### Audiences
- **Agency owners / leads:** estate map, approvals inbox, reports
- **Developers:** mostly chat and CLI; ledger and incidents on the web
- **Clients:** portal only

### Timing
- Phase 1: read-only estate map + adoption review
- Phase 2: approvals inbox
- Phase 3: incidents + policy grid
- Phase 6: client portal

---

## 12. Roadmap

Ordered by trust rather than week numbers. Each phase ships one working "team member."

Pace: 3–4 hours on weekdays, 5+ hours on weekends.

| Phase | Ships | Main folders |
|---|---|---|
| **1. Hands, memory, discovery (read-only)** | Agent, control plane, discovery, adoption, read-only web. Dogfood on HireFlow EC2. Groundcrew watches and documents before touching anything, building the baseline data the crew needs. | `agent/`, `cli/`, `proto/`, `server/` (tenancy, fleet, estate, ledger), `lab/`, `web/` |
| **2. Release engineer** | Deploys, preview environments via Cloudflare tunnel routes, verification, auto-rollback, approvals | `workflows/change`, `workflows/preview-envs`, `crew/operators/release`, approvals |
| **3. On-call SRE** | Detect → diagnose with the ledger → remediate within policy → postmortems | `crew/operators/oncall`, incidents, signals, policy grid |
| **4. Hygiene & security autopilot** | Certs, patches, CVEs, secret rotation, backup restore drills | `crew/operators/hygiene`, `workflows/drills` |
| **5. Platform + FinOps** | OpenTofu provisioner, cloud resources via instance roles, cost per client | `provisioner/opentofu`, `crew/operators/platform`, `finops` |
| **6. Agency layer** | Client portal, monthly ops reports, handover export, retainer mode | `reports`, client portal in `web/` |

**Gate before Phase 6** (first external client adopted): per-client model config, BYOK key storage (secrets module), and `allowFreeTierModels` enforcement (default `false`) must all exist. See [docs/decisions/0001-ai-layer-architecture.md](docs/decisions/0001-ai-layer-architecture.md), Decision 11.

### Phase 1 build order
1. Control plane running with Postgres and register-agent / heartbeat endpoints
2. Minimal agent that enrolls and sends heartbeats from a lab machine
3. Discovery scan feeding the estate graph
4. Command dispatch, starting with read-only diagnostics
5. CLI wrapping the API (stop curling by hand)
6. Read-only web: estate map + adoption review
7. Adopt HireFlow's EC2 box as the first real client server

---

## 13. Naming

**Final name: Groundcrew.** Aviation ground crews keep aircraft airworthy between flights; the agency flies the plane, Groundcrew keeps it airworthy.

- CLI: `groundcrew`
- Agent binary: `groundcrew-agent`
- Manifest: `groundcrew.yaml`
- AI operators folder: `crew/`

Known trade-off: "crew" in the AI-agent space is strongly associated with CrewAI, and as a common English compound the obvious domains may be taken.

### Naming history
| Name | Outcome |
|---|---|
| agencyctl | Original working name |
| wardctl / Ward | Strong candidate; verified clean on npm, GitHub and wardctl.dev at the time. Superseded by Groundcrew. |
| Amanat | Rejected: collides with Amanat Holdings PJSC (UAE investment firm); domains taken |
| Zimma, Keyturn | Backups; Zimma's .com/.io taken |
| Pehra, Nigraan | Urdu options considered in the redesign; not chosen |
| Pitcrew | Rejected: already used by several devtools (GitHub Actions runner orchestrator, a Python SSH task runner with a `crew` CLI, a Jira release-risk AI) |
| Bosun | Rejected: collides with Stack Exchange's open-source monitoring/alerting system |
| Berth, Slipway, Baton | Rejected: crowded in devtools |
| Mandate | Rejected: domain taken |
| Custodian, Steward, Warden, Tenure, Handoff | Bare words squatted on npm/GitHub |

---

## 14. Open Questions

- [ ] **Customer:** agencies only (keeps multi-client + handover as the moat) or any small software team without a DevOps person (bigger market, head-on with all-in-one platforms)?
- [ ] **Groundcrew availability:** GitHub org, npm package, domain — verify before committing
- [ ] **Open-core split:** which parts are free (agent, CLI, engine?) and which are paid (crew, hosted control plane, portal?)
- [ ] **LLM cost model:** cost per client per month, and how it's priced
- [ ] **Control plane location for dogfooding:** same EC2 box as HireFlow, or a separate small instance (leaning separate)
- [ ] **Protocol details:** heartbeat interval, command polling vs. long-poll, retry semantics
- [ ] **`groundcrew.yaml` manifest shape**
- [ ] **Custom Claude Code skills** to build for the project

---

## Appendix A — Claude Design Prompt

```
Design the web app for Groundcrew, an AI DevOps team for software agencies.

## What the product is
Groundcrew replaces a software agency's DevOps team. The agency installs a small agent on each client's server. The agent connects out to Groundcrew and can only perform a fixed set of safe, typed actions: deploy, rollback, restart, scale, snapshot, restore, renew certificate, rotate secret, and run read-only diagnostics.

A "crew" of AI operators watches every client's systems and acts within policies the agency sets:
- RELEASE: deploys, staging/preview environments, rollbacks
- ON-CALL: detects incidents, diagnoses, fixes, writes postmortems
- PLATFORM: servers, DNS, tunnels, databases
- HYGIENE: certificates, patches, CVEs, backups and restore drills, secret rotation
- FINOPS: idle resources, oversized servers, cost per client
- DESK: answers developer requests ("give Ali access to acme staging logs")

Every action follows one loop: plan → precheck → execute → verify → auto-rollback or commit → recorded in a change ledger. Autonomy is set per client, per environment, per action, at one of four levels: Observe, Suggest, Approve, Autonomous. Two core promises: client credentials never leave their servers, and secrets never reach the AI.

Day-to-day work happens in Slack and the CLI. The web app is for three things: seeing the whole picture, reviewing before trusting the crew, and showing clients what is happening.

## Who uses it
- Agency owner / tech lead: oversees all clients, approves risky actions (often from their phone at night), reads reports
- Developers: check the ledger and incidents
- The agency's clients: see a limited, agency-branded portal

## Brand direction
The name comes from aviation ground crews who keep aircraft airworthy between flights. Borrow the feel of ground operations without any literal planes or airport illustrations:
- hi-vis safety orange as the single accent
- signal-light status colors
- call-sign-style labels
- tarmac-marking-inspired dividers

Mood: a calm control room at 2am, not a flashy AI startup. Information-dense but never cluttered.

Colors:
- Neutrals: deep graphite/charcoal for dark mode, warm off-white for light mode
- Accent: safety orange, used sparingly (primary actions and the crew's presence)
- Status: green = healthy, amber = degraded/warning, red = down/failed, blue = in progress, grey = unknown. Always pair color with an icon and a label, never color alone.

Type: a sturdy utilitarian sans for UI, and a monospace for hostnames, versions, commands, YAML and timestamps.

Avoid: purple/blue AI gradients, glassmorphism, robot or sparkle icons, emoji, stock illustrations.

## Humans vs. crew
Every action and message shows who did it:
- Humans: avatar + name
- Crew operators: a compact role badge (ON-CALL, RELEASE, etc.) in one consistent shape. Make this badge a signature component of the system.

Also design one clear visual for the four autonomy levels (Observe / Suggest / Approve / Autonomous) and reuse it everywhere.

## Sample data (use throughout, keep consistent)
- Agency: Saffron Labs
- Clients: Acme Logistics, Bloom Skin Clinic, Northwind Retail, TaskFlow HR
- Each client has dev / staging / prod environments
- Services: api, web, worker, redis, postgres
- Servers: acme-prod-01 (AWS EC2, ap-south-1), bloom-vps-02 (Hetzner)

Include these realistic states:
- one prod environment degraded
- one TLS cert expiring in 9 days
- two pending approvals
- one open incident on Northwind Retail
- one server mid-adoption

## Global layout
- Left sidebar: Estate, Approvals (with count badge), Ledger, Incidents, Policy, Reports, Clients, Settings
- Top bar: agency switcher, client filter, command palette (⌘K), and an "Ask the crew" button that opens a right-side chat drawer

## Screens

1. ESTATE (home)
- Summary strip: clients, environments, open incidents, pending approvals, expiring certs, monthly cost
- Main area toggles between two views:
  - Client cards: each shows its environments with health, last deploy, current autonomy level, and crew activity count
  - Graph view: client → environment → service → server, with health on every node
- Collapsible right rail with a live crew activity feed, e.g. "ON-CALL restarted worker on bloom-staging · verified healthy · 14s"

2. ADOPTION REVIEW (existing server just scanned by the agent)
- Left panel: discovered items grouped by type (containers, systemd services, nginx/Caddy sites, cron jobs, open ports, certificates, volumes). Each item has a confidence indicator and a toggle: Manage / Ignore.
- Highlighted warnings:
  - "TLS cert for api.acme.io expires in 9 days"
  - "Backup cron has failed for 23 days"
  - "Port 5432 is exposed publicly"
- Right panel: the generated groundcrew.yaml with syntax highlighting. Hovering an item on the left highlights its lines in the YAML.
- Clear reassurance that adopting changes nothing on the server. Primary CTA: "Adopt server".

3. APPROVALS INBOX (must also work well at 390px mobile width)
- List of pending action cards. Each card shows:
  - operator badge, action and target
  - the reason in plain English
  - the plan as numbered steps
  - blast radius (which services and users are affected)
  - verification criteria
  - rollback path
  - an expires-in timer
- Card buttons: Approve, Deny, Ask the crew (opens chat scoped to this action)
- Example cards:
  - RELEASE wants to run a database migration on northwind-prod
  - ON-CALL wants to roll back taskflow-api from v42 to v41 because the error rate rose to 8% after the last deploy

4. CHANGE LEDGER
- Dense timeline of every change across all clients
- Filters: client, environment, actor (human/crew), operator, outcome
- Each row: monospace timestamp, actor, action, target, and an outcome chip (Verified, Rolled back, Failed, Awaiting approval)
- Expanding a row shows the plan, verification results, and a config/manifest diff

5. INCIDENT VIEW (also needs a mobile layout)
- Header: severity, affected client/service, duration, status
- Timeline from detection to resolution
- "What changed" panel: ledger entries near the incident start, with the likely culprit highlighted
- The crew's hypotheses, each with evidence (log excerpts, metric sparklines) and a confidence level
- Actions taken and how each was verified
- An auto-drafted postmortem in an editable panel

6. AUTONOMY POLICY
- A grid. Rows are actions: restart, deploy, rollback, scale, rotate secret, renew cert, run migration, restore backup, provision resource, delete resource. Columns are environments, grouped by client.
- Each cell shows Observe / Suggest / Approve / Autonomous with clearly different visual weight. Click a cell to change it; cells can be bulk-selected.
- Presets: Cautious, Standard, Trusted
- Visible guardrails: some cells are locked (e.g. "delete resource" on prod can never be Autonomous), with a lock icon and tooltip

7. CLIENT PORTAL (client-facing, agency-branded)
- Uses the agency's logo and accent color. Groundcrew branding stays minimal ("Operated by Groundcrew").
- Shows:
  - plain-language status and this month's uptime
  - recent changes written for non-engineers
  - the latest monthly ops report
  - a handover package download
- No internal jargon, no approvals, no policy

## Deliverables
1. A compact component sheet first: colors (light + dark), type scale, status chips, operator badges, the autonomy-level control, an approval card, a ledger row
2. All 7 screens at 1440px desktop in dark mode, plus light-mode versions of Estate and Client portal
3. Mobile (390px) versions of Approvals and Incident

Keep components and sample data consistent across every screen.
```

Tip: if the first pass comes out thin, split it in two — component sheet plus Estate and Approvals first, then the remaining screens based on those.

---

## Sources

Market research (October 2026):
- [AI SRE buyer's guide — TierZero](https://www.tierzero.ai/guides/buyers-guide/)
- [Best AI SRE Tools 2026 — wetheflywheel](https://wetheflywheel.com/en/guides/best-ai-sre-tools-2026)
- [Best AI SRE Tools in 2026 — Mezmo](https://www.mezmo.com/learn/best-ai-sre-tools-00035)
- [Top 20 AI SRE Tools in 2026 — NeuBird](https://neubird.ai/blog/top-ai-sre-tools)
- [SRE for AI: multi-agent incident response](https://learn.traeai.com/t/ai-engineering/phases/17-infrastructure-and-production/23-sre-for-ai)
- [New Relic SRE Agent press release (Feb 2026)](https://newrelic.com/press-release/20260224)
- [DataAgent — CB Insights](https://www.cbinsights.com/company/dataagent)
- [What is an AI DevOps Engineer — DuploCloud](https://duplocloud.com/article/what-is-an-ai-devops-engineer/)

Naming collision checks:
- [PitCrew (GitHub Actions runners) — Dev Leader](https://www.devleader.ca/projects/pitcrew)
- [pitcrew — PyPI](https://pypi.org/project/pitcrew/0.0.2/)
- [PitCrew AI — Devpost](https://devpost.com/software/pitcrew-ai-delivery-risk-intelligence-for-jira)
- [CrewAI Enterprise](https://crewai.com/enterprise)
