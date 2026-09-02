# Project Overview — Agency Multi-Client Control Plane

*(working name: `wardctl` — not finalized, see Naming section)*

---

## 1. The Idea

**Problem:** Software agencies delivering work to multiple clients (e.g. Aslase/Inception delivering TAKAFO.ai for Mubadala) manage each client's environment ad hoc — manual SSH sessions, scattered `.env` files, no single source of truth for which client has which config, and a manual doc-writing scramble at handover time. There's no tool that treats "a client's environment" as a first-class, trackable object.

**What the tool does:** A lightweight agent + control plane that an agency installs once, then uses to spin up, track, and hand off isolated client environments — deployment status, secrets, access boundaries, and a clean audit trail — all from one place instead of tribal knowledge and scattered SSH sessions.

**Why this project:**
- A real problem lived firsthand through client delivery work, not speculative
- Legitimate open-core SaaS shape (free CLI/engine, paid dashboard/hosting)
- Forces building real infra (agents, control planes, multi-tenant isolation) — stronger portfolio signal than another CRUD app
- The "multi-client delivery layer for software houses" gap was independently flagged as worth tracking

**Scope discipline:** The plan defers the dashboard to weeks 7–10 and keeps it "thin" deliberately — the CLI/engine is the actual product, the dashboard is a viewer on top of it. Avoid the temptation to build a full PaaS; that turns a portfolio project into a 6-month project that never ships.

---

## 2. Timeline & Commitment

- **Start:** after SLEUTH ships (~2–3 days out, as of the planning session)
- **Pace:** 3–4 hrs weekdays, 5+ hrs weekends
- **Duration:** 14 weeks

| Weeks | Phase |
|---|---|
| 1–3 | Core engine (agent + control plane), dogfooded on HireFlow's existing EC2 box |
| 4–6 | Environment model (Docker Compose-based provisioning, config/secrets) |
| 7–10 | Thin dashboard + hard public launch deadline |
| 11–14 | Client portal + handover export |

**Build approach:** Claude Code / Cursor / agents-first, with custom Claude skills created for the project.
**Goal:** standout portfolio project (and a legitimate shot at an open-core product).

---

## 3. Tech Stack

| Layer | Choice | Why |
|---|---|---|
| Agent / CLI (wks 1–3) | **Go** | Single static binary, no runtime dependency on client's box, matches CLI/agent conventions (`kubectl`, `terraform`), builds on existing Go reps from MiniRed |
| Agent↔control-plane protocol | HTTPS + JSON (polling/webhook) to start | Simpler than gRPC; don't add streaming complexity until actually needed |
| Control-plane state store (wks 1–6) | **SQLite** | Zero ops burden while solo; migrate to Postgres only when dashboard/multi-user (wk 7+) demands it |
| Environment unit (wks 4–6) | **Docker / Docker Compose** | Already deeply known from HireFlow deployment |
| Secrets (wks 4–6) | Encrypted-at-rest files / Vault-lite | Full HashiCorp Vault is over-engineering for solo v1 |
| Dashboard (wks 7–10) | **Vue 3 + Vite**, likely **NestJS** API layer | Reuse HireFlow patterns; NestJS depth makes it the pragmatic backend pick |
| Handover export (wks 11–14) | Structured JSON/PDF generation | Built from environment state already being tracked |
| Dogfood infra | Existing HireFlow EC2 instance (ap-south-1) | Zero-risk, real imperfect environment to prove the tool on |

---

## 4. Module List

**Core Engine (wks 1–3)**
- `agent` — lightweight process on client servers; reports status, executes control-plane commands
- `control-plane` — central service the agent talks to; owns environment state
- `registry` — tracks known environments (client, server, status, last-seen)

**Environment Model (wks 4–6)**
- `env-provisioner` — spins up/tears down isolated client environments (Docker Compose–based, via a **swappable provisioner abstraction** — see Design Decisions)
- `config-store` — per-environment secrets/env vars, encrypted
- `access-control` — who on the agency team can touch which client's environment

**Dashboard (wks 7–10)**
- `dashboard-api` — thin API layer over control-plane data
- `dashboard-ui` — status view: environments, health, last deploy, drift
- `notifications` — basic alerting (optional, cut first if timeline slips)

**Client Portal + Handover (wks 11–14)**
- `handover-export` — generates a structured handover package from environment state (key differentiator vs. generic devops tools)
- `client-portal` — limited-scope view for the client at project end
- `audit-log` — history of actions on an environment, feeds the handover export

---

## 5. Core Module Implementation — Conceptual Shape

The **engine** (agent + control plane) is the foundation everything else sits on.

1. **Agent** — small Go binary, one-line install script (curl-installed, systemd service). Job is narrow: heartbeat status to control plane, execute commands when told (deploy, restart, tear down). No decision-making — pure execution + reporting.
2. **Control plane** — central service (dogfooded on the HireFlow EC2 box). Database of environments (client, server, status, last-seen, what's deployed) + an API for heartbeats and command dispatch. Source of truth for "should be running" vs. agent-reported "is running."
3. **CLI** — `wardctl list`, `wardctl deploy client-a`, `wardctl status` — the actual product surface, talks to the control plane's API.
4. **Registration/enrollment flow** — new agent generates identity, exchanges a one-time enrollment token for a long-lived credential, registers into the environment registry.

**Weeks 1–3 build order:**
1. Control plane running with a DB + register-agent / receive-heartbeat endpoints
2. Minimal agent binary that registers and sends heartbeats from one test server
3. Command dispatch: control plane → agent → result back
4. Wrap in a CLI (stop curling the API by hand)
5. Dogfood against HireFlow's EC2 box as the first real environment

**On-demand staging environments** (e.g. `wardctl env create staging`):
1. Client repo needs a manifest (`ward.yaml`) describing what "staging" means — services, overrides, resource limits (one-time setup per project)
2. Control plane sends the agent a provisioning command; agent runs `docker compose up` with staging config locally on the client server
3. URL assignment via **dynamic Cloudflare Tunnel routes** (agent or control plane calls the Cloudflare API to create a named route on the fly) — reuses the exact pattern already hand-built for HireFlow's `api.whapps.site`. Chosen over a self-hosted reverse proxy because it needs no public IP or client-side firewall config.

---

## 6. Key Design Decisions

### Onboarding model: agent-installed, not credential-handoff
Two models were considered:
- **Model A (rejected):** Client hands over cloud/SSH credentials; tool provisions infra centrally on their behalf (SaaS-style).
- **Model B (chosen):** Client runs a one-line install script on their own server. Agent lives there, acts only locally, using permissions the server already has. **Control plane never touches client cloud credentials.**

This is also the sales pitch: "install an agent, no credentials leave your infrastructure" is a far easier trust story for an agency evaluating the tool than handing over AWS keys.

**Two separate auth problems:**
- *Agent → control plane (machine identity):* one-time enrollment token (`wardctl agent enroll --client=acme-corp`) → install script → agent exchanges token for long-lived credential on first boot. Standard machine-bootstrap pattern.
- *Developer → control plane (human identity):* MVP is a simple API key in a local CLI config file. Real multi-user permissions/SSO deferred to dashboard/portal phase (wk 7+).

### Provisioner abstraction (Kubernetes question)
Docker Compose only for the full 14 weeks — Kubernetes is a different complexity tier (cluster-level concepts, operator/controller pattern, Ingress instead of tunnel routes) and building for a client that doesn't exist yet is scope creep. The one future-proofing move: never hardcode "run docker compose" into the agent's core logic — treat "how do I bring this environment up" as a swappable **provisioner** interface with Docker Compose as the only implementation for now. Mirrors the same instinct as SLEUTH's `.scm`-based scaling design: solve today's problem, don't nail the door shut on tomorrow's.

### Push (SSH) vs. pull (agent) — informed by studying Coolify
Studied Coolify's architecture as a reference (Laravel 12 + Livewire control panel that SSHes into connected servers, holding the SSH private keys itself, running Docker commands remotely over multiplexed SSH connections). This is the opposite of our model:

| | Coolify (SSH-push) | This project (agent-pull) |
|---|---|---|
| How control plane reaches servers | Holds SSH private keys, connects outward | Agent installed on client server, phones home |
| Where credentials live | Control plane holds keys to every connected server | Control plane never holds client SSH/cloud creds |
| Trust direction | Control plane reaches into client infra | Client infra reaches out to control plane |
| Network requirement | Client server must accept inbound SSH | Client server just needs outbound internet |

Coolify's model works because it's typically one operator managing their own servers. For a multi-client agency tool, asking every client to grant an outside vendor inbound SSH access is a much harder trust ask — confirming agent-pull as the right foundation for this specific problem, not just a stylistic preference.

### Dogfooding target
HireFlow's existing EC2 instance (4 containers: backend, frontend, ml-service, redis via Docker Compose) is the first real "client server." Realistic imperfect environment (mixed-content issues, tunnel routing, Docker networking quirks already fought through once) at zero risk. Open question: run the control plane on the same box (simplest) or a separate small instance (more realistically mirrors the real network boundary agents will cross) — leaning toward separate to actually test agent-over-internet, not agent-over-localhost.

---

## 7. Competitive Landscape

None of the existing tools treat "client" or "environment ownership boundary" as a first-class concept — they think in terms of *apps* and *servers*, not *which client this belongs to* and *how do I hand it back cleanly*. That gap is the intended white space.

### Direct competitors (self-hosted PaaS / deployment control planes)

| Tool | Model | Strength | Gap (the opening) |
|---|---|---|---|
| **Coolify** | Self-hosted, Docker/Compose, SSH-push, ~57K GitHub stars | Most feature-rich, polished UI, 280+ one-click services, multi-tenant "teams" for basic client isolation | No environment-lifecycle/handover concept — treats servers as infrastructure you own, not infrastructure clients own |
| **Dokploy** | Self-hosted, Docker Swarm + Traefik, ~35K stars | Lightest footprint (~350MB RAM idle), native Compose support | Same gap — no client/ownership boundary |
| **CapRover** | Self-hosted, Docker Swarm, mature since 2017, ~15K stars | Most stable, large one-click app store | Limited Compose support, dated, slowing development |
| **Portainer** | Self-hosted Docker/K8s GUI | Solid for solo/internal server management | Not agency/client-oriented — a Docker dashboard, not a delivery workflow tool |

### Adjacent competitors (different layer)
- **Laravel Forge / RunCloud / SpinupWP / GridPane** — server provisioning + app management, but framework-specific (PHP/Laravel/WordPress), not multi-tenant-environment-aware
- **DeployHQ** — Git-triggered build/deploy tool, closer to CI/CD than environment/client management
- **Deploynix** — managed, non-Docker, Laravel/PHP-specific, Forge-style alternative

### Positioning takeaway
Don't compete with Coolify on feature breadth (template count, polish) — lose that fight day one. Compete on being the **agency delivery layer**: client isolation, environment lifecycle (dev → staging → handover), and audit trail, layered on top of Docker Compose orchestration rather than reinventing app deployment.

---

## 8. Naming (Undecided — Revisit at Project Start)

Decision deferred; verify GitHub org, npm package, and domain availability again before committing (results below were valid at time of research, may change).

| Candidate | Meaning | Status |
|---|---|---|
| **Amanat** | Urdu: a trust held in care, to be returned intact | ❌ Rejected — collides with Amanat Holdings PJSC (publicly traded UAE investment firm), especially bad given UAE-facing client work; domains taken |
| **Zimma** | Urdu: liability/responsibility | npm free, domain (.io/.com) taken, .dev looked open; GitHub org needs prefix pattern (username collision) |
| **Keyturn** | English wordplay on "turnkey" | npm free, .dev looked open; GitHub org needs prefix pattern |
| **wardctl** | "Ward" = one under your protection/care + `-ctl` devops convention (`kubectl`, `systemctl`) | ✅ Confirmed clean: npm free, GitHub org/user available, `wardctl.dev` open — strongest verified candidate so far |
| Custodian / Steward / Warden / Tenure / Handoff | English trust/care/handover semantics | Bare words all squatted on npm/GitHub — would need a compound/prefixed variant |

**Current lean:** `wardctl`, pending final confirmation.

---

## 9. Open Questions / Next Decisions

- [ ] Finalize naming (org, npm, domain — re-verify before commit)
- [ ] Control plane location for dogfooding: same EC2 box as HireFlow, or separate small instance to test the real network boundary
- [ ] Exact heartbeat/command protocol design (HTTPS polling vs. webhook vs. later gRPC)
- [ ] Repo scaffolding and initial `ward.yaml` manifest shape
- [ ] Custom Claude Code skills to build for this project
