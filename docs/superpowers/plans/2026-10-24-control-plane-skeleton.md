# Control Plane Skeleton Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Stand up the first working slice of the Groundcrew control plane — a NestJS service backed by Postgres that can register an agent and record its heartbeats — satisfying Phase 1, build-order step 1 of the roadmap.

**Architecture:** A single NestJS app (`server/`) with a `fleet` module exposing `POST /fleet/agents` (register) and `POST /fleet/agents/:id/heartbeat`. Prisma talks to a Postgres instance run locally via Docker Compose. No auth, no other modules (identity/tenancy/estate/etc.) yet — those arrive in later plans as their own phases.

**Tech Stack:** NestJS 12.2.0, Prisma 7.10.1 + `@prisma/client` 7.10.1, PostgreSQL 16 (via `postgres:16-alpine` in Docker Compose), TypeScript 6.0.3, Jest 30.5.2 + ts-jest 29.4.15 + Supertest 7.3.1, pnpm as package manager.

**Spec:** [groundcrew-project-overview.md](../../../groundcrew-project-overview.md) (sections 3, 8, 10, 12) and [2026-10-24-groundcrew-design.md](../specs/2026-10-24-groundcrew-design.md)

## Global Constraints

- Package manager: `pnpm` (already installed, v12.5.1 confirmed in this environment). Node.js must be ≥20.6 for `--env-file` support (v22.23.2 confirmed in this environment).
- The app's own `npm` scripts load `server/.env` via Node's built-in `--env-file` flag (see Task 1, Step 1) — no app-level `dotenv` dependency needed for that.
- **Prisma 7 breaking changes (verified against the pinned `7.10.1` in this environment — a plain `datasource { url = env("DATABASE_URL") }` in `schema.prisma` fails with error `P1012` on that version):**
  - `schema.prisma`'s `datasource` block no longer takes a `url`. The connection string for Migrate moves to `server/prisma.config.ts`.
  - The Prisma CLI no longer auto-loads `.env` — `prisma.config.ts` must `import "dotenv/config"` itself, so `dotenv` is a real devDependency here (pinned below), used only by that config file.
  - `PrismaClient` at runtime needs a driver adapter: `@prisma/adapter-pg` wrapping `pg`, passed as `new PrismaClient({ adapter })`. Verified end-to-end in a throwaway sandbox: `prisma generate` succeeds with no `url` in the schema, and `new PrismaPg({ connectionString }) ` + `new PrismaClient({ adapter })` instantiate and attempt a real connection (confirmed by a connection-refused error with no DB listening).
  - `prisma migrate dev` / `db push` no longer auto-run `prisma generate` — Task 2, Step 10 runs both explicitly.
  - Omit `?schema=public` from `DATABASE_URL` — `@prisma/adapter-pg` has an open bug ignoring the `schema` query param ([prisma/prisma#28961](https://github.com/prisma/prisma/issues/28961) class of issues); unnecessary anyway since we only use the default `public` schema.
- **Environment prerequisite — not yet satisfied in this environment:** Docker Desktop's daemon was not running when this plan was written (`docker run` failed with `failed to connect to the docker API at npipe:////./pipe/dockerDesktopLinuxEngine`). Start Docker Desktop before Task 2, Step 2.
- **Project commit rule:** whoever executes this plan (agent or subagent) runs `git add` at the end of each task but never `git commit` — the user commits in this repo themselves, in detail, once a task's changes are staged and verified. Every "Commit" step below means "stage and stop," not "commit."
- Exact dependency versions (pin these in `server/package.json`, do not use `^` or `latest`):
  - `@nestjs/common` `12.2.0`, `@nestjs/core` `12.2.0`, `@nestjs/platform-express` `12.2.0`, `@nestjs/testing` `12.2.0`
  - `@prisma/client` `7.10.1`, `prisma` `7.10.1`, `@prisma/adapter-pg` `7.10.1`, `pg` `8.23.1`, `dotenv` `18.0.9`
  - `class-transformer` `0.5.1`, `class-validator` `0.15.1`
  - `reflect-metadata` `0.2.2`, `rxjs` `7.8.2`
  - `typescript` `6.0.3` — must stay below `7.0.0`; ts-jest 29.x's peer range is `>=4.3 <7` and does not yet support TypeScript 7.
  - `jest` `30.5.2`, `ts-jest` `29.4.15`, `ts-node` `10.9.2`, `supertest` `7.3.1`
  - `@types/jest` `30.0.0`, `@types/node` `26.6.5`, `@types/supertest` `7.2.1`, `@types/pg` `8.23.1`
- No authentication/authorization on these endpoints in this plan — identity/access modules are a later phase. Do not add auth guards or tokens here.
- Follow the repo folder structure from the overview doc: this plan only creates `server/` and `deploy/`. Do not scaffold `agent/`, `cli/`, `web/`, `crew/`, etc. — those are separate future plans (YAGNI).
- Prisma model field names, endpoint paths and response shapes defined in Task 1-4 below are final for this plan — later plans (the Go agent, the CLI) will consume `POST /fleet/agents` and `POST /fleet/agents/:id/heartbeat` exactly as specified here.

---

## File Structure

```
deploy/
  docker-compose.yml          # local Postgres for dev + testing

server/
  package.json
  tsconfig.json
  tsconfig.build.json
  jest.config.js              # unit tests (*.spec.ts under src/)
  prisma.config.ts            # Prisma 7: DATABASE_URL for Migrate lives here, not in schema.prisma
  .env.example
  .gitignore
  prisma/
    schema.prisma             # Agent model, no datasource.url (Prisma 7)
  src/
    main.ts
    app.module.ts
    health/
      health.controller.ts
    prisma/
      prisma.module.ts
      prisma.service.ts
      prisma.service.spec.ts
    modules/
      fleet/
        fleet.module.ts
        fleet.controller.ts
        fleet.service.ts
        dto/
          register-agent.dto.ts
  test/
    jest-e2e.json
    health.e2e-spec.ts
    fleet.e2e-spec.ts
```

---

### Task 1: NestJS scaffold + health endpoint

**Files:**
- Create: `server/package.json`
- Create: `server/tsconfig.json`
- Create: `server/tsconfig.build.json`
- Create: `server/jest.config.js`
- Create: `server/test/jest-e2e.json`
- Create: `server/.gitignore`
- Create: `server/src/main.ts`
- Create: `server/src/app.module.ts`
- Create: `server/src/health/health.controller.ts`
- Test: `server/test/health.e2e-spec.ts`

**Interfaces:**
- Produces: `AppModule` (`server/src/app.module.ts`) — imported by every later task's e2e tests via `import { AppModule } from '../src/app.module'`.
- Produces: `GET /health` → `{ status: 'ok' }`, HTTP 200.

- [ ] **Step 1: Create `server/package.json`**

```json
{
  "name": "@groundcrew/server",
  "version": "0.0.1",
  "private": true,
  "scripts": {
    "build": "tsc -p tsconfig.build.json",
    "start": "node --env-file=.env dist/main.js",
    "start:dev": "node --env-file=.env -r ts-node/register src/main.ts",
    "test": "node --env-file=.env node_modules/jest/bin/jest.js",
    "test:e2e": "node --env-file=.env node_modules/jest/bin/jest.js --config test/jest-e2e.json",
    "prisma:generate": "prisma generate",
    "prisma:migrate": "prisma migrate dev"
  },
  "dependencies": {
    "@nestjs/common": "12.2.0",
    "@nestjs/core": "12.2.0",
    "@nestjs/platform-express": "12.2.0",
    "class-transformer": "0.5.1",
    "class-validator": "0.15.1",
    "reflect-metadata": "0.2.2",
    "rxjs": "7.8.2"
  },
  "devDependencies": {
    "@nestjs/testing": "12.2.0",
    "@prisma/client": "7.10.1",
    "@types/jest": "30.0.0",
    "@types/node": "26.6.5",
    "@types/supertest": "7.2.1",
    "jest": "30.5.2",
    "prisma": "7.10.1",
    "supertest": "7.3.1",
    "ts-jest": "29.4.15",
    "ts-node": "10.9.2",
    "typescript": "6.0.3"
  }
}
```

- [ ] **Step 2: Create `server/tsconfig.json`**

```json
{
  "compilerOptions": {
    "module": "commonjs",
    "target": "ES2021",
    "lib": ["ES2021"],
    "experimentalDecorators": true,
    "emitDecoratorMetadata": true,
    "esModuleInterop": true,
    "skipLibCheck": true,
    "forceConsistentCasingInFileNames": true,
    "strict": true,
    "moduleResolution": "node",
    "resolveJsonModule": true,
    "outDir": "./dist",
    "baseUrl": "./"
  },
  "include": ["src/**/*.ts"]
}
```

- [ ] **Step 3: Create `server/tsconfig.build.json`**

```json
{
  "extends": "./tsconfig.json",
  "exclude": ["node_modules", "dist", "test", "**/*.spec.ts", "**/*.e2e-spec.ts"]
}
```

- [ ] **Step 4: Create `server/jest.config.js`**

```js
module.exports = {
  preset: 'ts-jest',
  testEnvironment: 'node',
  rootDir: 'src',
  testRegex: '.*\\.spec\\.ts$',
};
```

- [ ] **Step 5: Create `server/test/jest-e2e.json`**

```json
{
  "preset": "ts-jest",
  "testEnvironment": "node",
  "rootDir": "..",
  "testRegex": "test/.*\\.e2e-spec\\.ts$"
}
```

- [ ] **Step 6: Create `server/.gitignore`**

```
node_modules/
dist/
.env
```

- [ ] **Step 7: Install dependencies**

Run: `cd server && pnpm install`
Expected: lockfile `server/pnpm-lock.yaml` created, `server/node_modules` populated, no errors.

- [ ] **Step 8: Create `server/src/main.ts`**

```typescript
import 'reflect-metadata';
import { NestFactory } from '@nestjs/core';
import { AppModule } from './app.module';

async function bootstrap() {
  const app = await NestFactory.create(AppModule);
  await app.listen(process.env.PORT ?? 3000);
}

bootstrap();
```

- [ ] **Step 9: Create `server/src/app.module.ts`**

```typescript
import { Module } from '@nestjs/common';

@Module({
  imports: [],
  controllers: [],
})
export class AppModule {}
```

- [ ] **Step 10: Write the failing test — `server/test/health.e2e-spec.ts`**

```typescript
import { Test } from '@nestjs/testing';
import { INestApplication } from '@nestjs/common';
import * as request from 'supertest';
import { AppModule } from '../src/app.module';

describe('Health (e2e)', () => {
  let app: INestApplication;

  beforeAll(async () => {
    const moduleRef = await Test.createTestingModule({
      imports: [AppModule],
    }).compile();
    app = moduleRef.createNestApplication();
    await app.init();
  });

  afterAll(async () => {
    await app.close();
  });

  it('GET /health returns ok', async () => {
    const response = await request(app.getHttpServer()).get('/health');
    expect(response.status).toBe(200);
    expect(response.body).toEqual({ status: 'ok' });
  });
});
```

- [ ] **Step 11: Run the test to verify it fails**

Run: `cd server && pnpm test:e2e`
Expected: FAIL — `404` instead of `200` (no `/health` route registered yet).

- [ ] **Step 12: Create `server/src/health/health.controller.ts`**

```typescript
import { Controller, Get } from '@nestjs/common';

@Controller('health')
export class HealthController {
  @Get()
  check() {
    return { status: 'ok' };
  }
}
```

- [ ] **Step 13: Wire it into `server/src/app.module.ts`**

```typescript
import { Module } from '@nestjs/common';
import { HealthController } from './health/health.controller';

@Module({
  imports: [],
  controllers: [HealthController],
})
export class AppModule {}
```

- [ ] **Step 14: Run the test to verify it passes**

Run: `cd server && pnpm test:e2e`
Expected: PASS — `Health (e2e) › GET /health returns ok`.

- [ ] **Step 15: Stage for commit**

Per this project's commit rule (see Global Constraints), stage only — the user commits.

```bash
git add server
```

---

### Task 2: Postgres + Prisma wiring

**Files:**
- Create: `deploy/docker-compose.yml`
- Create: `server/.env.example`
- Create: `server/prisma.config.ts`
- Create: `server/prisma/schema.prisma`
- Create: `server/src/prisma/prisma.service.ts`
- Create: `server/src/prisma/prisma.module.ts`
- Modify: `server/package.json` (add `@prisma/adapter-pg`, `pg`, `dotenv`, `@types/pg`)
- Modify: `server/src/app.module.ts`
- Test: `server/src/prisma/prisma.service.spec.ts`

**Interfaces:**
- Consumes: nothing from Task 1 except `AppModule` shape (adds an import).
- Produces: `PrismaService` (`server/src/prisma/prisma.service.ts`), a global, injectable Prisma client exposing the generated `agent` model delegate (`prisma.agent.create/findUnique/update/delete`). Later tasks inject it via constructor: `constructor(private readonly prisma: PrismaService)`.
- Produces: `PrismaModule` (`@Global()`), already imported by `AppModule` — later tasks do not need to import it again.
- Produces: `Agent` Prisma model: `{ id: string, hostname: string, status: string, lastSeenAt: Date | null, createdAt: Date }`.

- [ ] **Step 1: Create `deploy/docker-compose.yml`**

```yaml
services:
  postgres:
    image: postgres:16-alpine
    restart: unless-stopped
    environment:
      POSTGRES_USER: groundcrew
      POSTGRES_PASSWORD: groundcrew
      POSTGRES_DB: groundcrew
    ports:
      - "5432:5432"
    volumes:
      - groundcrew-postgres-data:/var/lib/postgresql/data

volumes:
  groundcrew-postgres-data:
```

- [ ] **Step 2: Start Postgres**

Run: `docker compose -f deploy/docker-compose.yml up -d`
Expected: container `groundcrew-postgres-1` (or similar) running; `docker compose -f deploy/docker-compose.yml ps` shows state `running`/`healthy`.

- [ ] **Step 3: Create `server/.env.example`**

```
DATABASE_URL="postgresql://groundcrew:groundcrew@localhost:5432/groundcrew"
```

- [ ] **Step 4: Create `server/.env` (not committed — already covered by `server/.gitignore`)**

```
DATABASE_URL="postgresql://groundcrew:groundcrew@localhost:5432/groundcrew"
```

- [ ] **Step 5: Add Prisma's Postgres adapter deps to `server/package.json`**

Add to `dependencies`: `"@prisma/adapter-pg": "7.10.1"`, `"pg": "8.23.1"`.
Add to `devDependencies`: `"dotenv": "18.0.9"`, `"@types/pg": "8.23.1"`.

Run: `cd server && pnpm install`
Expected: lockfile updated, no errors.

- [ ] **Step 6: Create `server/prisma/schema.prisma`**

Prisma 7 removed the `url` field from the schema's `datasource` block (confirmed against the pinned `7.10.1`: it fails with error `P1012`, "The datasource property `url` is no longer supported in schema files"). The connection string now lives only in `prisma.config.ts` (Step 7) and is passed to `PrismaClient` via the driver adapter (Step 9).

```prisma
generator client {
  provider = "prisma-client-js"
}

datasource db {
  provider = "postgresql"
}

model Agent {
  id         String    @id @default(uuid())
  hostname   String
  status     String    @default("registered")
  lastSeenAt DateTime?
  createdAt  DateTime  @default(now())
}
```

- [ ] **Step 7: Create `server/prisma.config.ts`**

Prisma 7's CLI no longer auto-loads `.env`, so this file loads it itself via `dotenv/config`.

```typescript
import "dotenv/config";
import { defineConfig, env } from "prisma/config";

export default defineConfig({
  schema: "prisma/schema.prisma",
  migrations: {
    path: "prisma/migrations",
  },
  datasource: {
    url: env("DATABASE_URL"),
  },
});
```

- [ ] **Step 8: Write the failing test — `server/src/prisma/prisma.service.spec.ts`**

```typescript
import { Test } from '@nestjs/testing';
import { PrismaModule } from './prisma.module';
import { PrismaService } from './prisma.service';

describe('PrismaService', () => {
  let prisma: PrismaService;

  beforeAll(async () => {
    const moduleRef = await Test.createTestingModule({
      imports: [PrismaModule],
    }).compile();
    prisma = moduleRef.get(PrismaService);
    await prisma.$connect();
  });

  afterAll(async () => {
    await prisma.$disconnect();
  });

  it('creates and reads an Agent row', async () => {
    const created = await prisma.agent.create({
      data: { hostname: 'test-host' },
    });

    const found = await prisma.agent.findUnique({ where: { id: created.id } });

    expect(found?.hostname).toBe('test-host');
    expect(found?.status).toBe('registered');

    await prisma.agent.delete({ where: { id: created.id } });
  });
});
```

- [ ] **Step 9: Run the test to verify it fails**

Run: `cd server && pnpm test`
Expected: FAIL — cannot find module `./prisma.module` (files don't exist yet).

- [ ] **Step 10: Create `server/src/prisma/prisma.service.ts`**

`PrismaClient` needs a driver adapter in Prisma 7 — passing a bare connection string to the client constructor no longer works. `PrismaService` builds a `PrismaPg` adapter from `DATABASE_URL` and passes it to the `PrismaClient` superclass constructor. (`DATABASE_URL`'s availability in `process.env` at both test and runtime is explained right after the code block below.)

```typescript
import { Injectable, OnModuleDestroy, OnModuleInit } from '@nestjs/common';
import { PrismaPg } from '@prisma/adapter-pg';
import { PrismaClient } from '@prisma/client';

@Injectable()
export class PrismaService extends PrismaClient implements OnModuleInit, OnModuleDestroy {
  constructor() {
    super({
      adapter: new PrismaPg({ connectionString: process.env.DATABASE_URL }),
    });
  }

  async onModuleInit() {
    await this.$connect();
  }

  async onModuleDestroy() {
    await this.$disconnect();
  }
}
```

`DATABASE_URL` reaches this constructor at both test and runtime because every script that runs Jest or the app (`pnpm test`, `pnpm test:e2e`, `pnpm start:dev`, `pnpm start`) is defined with `node --env-file=.env ...` (Task 1, Step 1) — the variable is already in `process.env` before Jest or Nest ever starts, including inside Jest's forked workers, which inherit the parent process's environment.

- [ ] **Step 11: Create `server/src/prisma/prisma.module.ts`**

```typescript
import { Global, Module } from '@nestjs/common';
import { PrismaService } from './prisma.service';

@Global()
@Module({
  providers: [PrismaService],
  exports: [PrismaService],
})
export class PrismaModule {}
```

- [ ] **Step 12: Generate the Prisma client and apply the migration**

Run: `cd server && pnpm prisma:generate && pnpm prisma migrate dev --name init`
Expected: `server/node_modules/@prisma/client` regenerated with the `Agent` type; `server/prisma/migrations/<timestamp>_init/migration.sql` created; migration applied to the running Postgres container with no errors. (Prisma 7's `migrate dev` no longer runs `generate` automatically, which is why both are run here.)

- [ ] **Step 13: Wire `PrismaModule` into `server/src/app.module.ts`**

```typescript
import { Module } from '@nestjs/common';
import { HealthController } from './health/health.controller';
import { PrismaModule } from './prisma/prisma.module';

@Module({
  imports: [PrismaModule],
  controllers: [HealthController],
})
export class AppModule {}
```

- [ ] **Step 14: Run the test to verify it passes**

Run: `cd server && pnpm test`
Expected: PASS — `PrismaService › creates and reads an Agent row`.

- [ ] **Step 15: Stage for commit**

Per this project's commit rule (see Global Constraints), stage only — the user commits.

```bash
git add deploy server
```

---

### Task 3: `POST /fleet/agents` register endpoint

**Files:**
- Create: `server/src/modules/fleet/dto/register-agent.dto.ts`
- Create: `server/src/modules/fleet/fleet.service.ts`
- Create: `server/src/modules/fleet/fleet.controller.ts`
- Create: `server/src/modules/fleet/fleet.module.ts`
- Modify: `server/src/main.ts` (global `ValidationPipe`)
- Modify: `server/src/app.module.ts`
- Test: `server/test/fleet.e2e-spec.ts`

**Interfaces:**
- Consumes: `PrismaService` from Task 2 (`prisma.agent.create`).
- Produces: `RegisterAgentDto { hostname: string }` (`server/src/modules/fleet/dto/register-agent.dto.ts`) — Task 4 does not need this type.
- Produces: `FleetService.registerAgent(dto: RegisterAgentDto): Promise<Agent>` — Task 4 adds a sibling `heartbeat` method to this same class; do not rename `registerAgent`.
- Produces: `POST /fleet/agents` → 201, body `{ id, hostname, status: 'registered', lastSeenAt: null, createdAt }`. Missing/empty `hostname` → 400.

- [ ] **Step 1: Write the failing tests — `server/test/fleet.e2e-spec.ts`**

```typescript
import { Test } from '@nestjs/testing';
import { INestApplication, ValidationPipe } from '@nestjs/common';
import * as request from 'supertest';
import { AppModule } from '../src/app.module';
import { PrismaService } from '../src/prisma/prisma.service';

describe('Fleet (e2e)', () => {
  let app: INestApplication;
  let prisma: PrismaService;

  beforeAll(async () => {
    const moduleRef = await Test.createTestingModule({
      imports: [AppModule],
    }).compile();
    app = moduleRef.createNestApplication();
    app.useGlobalPipes(new ValidationPipe({ whitelist: true }));
    await app.init();
    prisma = moduleRef.get(PrismaService);
  });

  afterAll(async () => {
    await app.close();
  });

  it('POST /fleet/agents registers a new agent', async () => {
    const response = await request(app.getHttpServer())
      .post('/fleet/agents')
      .send({ hostname: 'lab-box-01' });

    expect(response.status).toBe(201);
    expect(response.body.hostname).toBe('lab-box-01');
    expect(response.body.status).toBe('registered');
    expect(response.body.lastSeenAt).toBeNull();

    await prisma.agent.delete({ where: { id: response.body.id } });
  });

  it('POST /fleet/agents rejects a missing hostname', async () => {
    const response = await request(app.getHttpServer())
      .post('/fleet/agents')
      .send({});

    expect(response.status).toBe(400);
  });
});
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `cd server && pnpm test:e2e`
Expected: FAIL — `404` (no `/fleet/agents` route registered yet).

- [ ] **Step 3: Create `server/src/modules/fleet/dto/register-agent.dto.ts`**

```typescript
import { IsNotEmpty, IsString } from 'class-validator';

export class RegisterAgentDto {
  @IsString()
  @IsNotEmpty()
  hostname!: string;
}
```

- [ ] **Step 4: Create `server/src/modules/fleet/fleet.service.ts`**

```typescript
import { Injectable } from '@nestjs/common';
import { PrismaService } from '../../prisma/prisma.service';
import { RegisterAgentDto } from './dto/register-agent.dto';

@Injectable()
export class FleetService {
  constructor(private readonly prisma: PrismaService) {}

  registerAgent(dto: RegisterAgentDto) {
    return this.prisma.agent.create({
      data: { hostname: dto.hostname },
    });
  }
}
```

- [ ] **Step 5: Create `server/src/modules/fleet/fleet.controller.ts`**

```typescript
import { Body, Controller, Post } from '@nestjs/common';
import { FleetService } from './fleet.service';
import { RegisterAgentDto } from './dto/register-agent.dto';

@Controller('fleet/agents')
export class FleetController {
  constructor(private readonly fleetService: FleetService) {}

  @Post()
  register(@Body() dto: RegisterAgentDto) {
    return this.fleetService.registerAgent(dto);
  }
}
```

- [ ] **Step 6: Create `server/src/modules/fleet/fleet.module.ts`**

```typescript
import { Module } from '@nestjs/common';
import { FleetController } from './fleet.controller';
import { FleetService } from './fleet.service';

@Module({
  controllers: [FleetController],
  providers: [FleetService],
})
export class FleetModule {}
```

- [ ] **Step 7: Wire `FleetModule` into `server/src/app.module.ts`**

```typescript
import { Module } from '@nestjs/common';
import { HealthController } from './health/health.controller';
import { PrismaModule } from './prisma/prisma.module';
import { FleetModule } from './modules/fleet/fleet.module';

@Module({
  imports: [PrismaModule, FleetModule],
  controllers: [HealthController],
})
export class AppModule {}
```

- [ ] **Step 8: Add the global `ValidationPipe` in `server/src/main.ts`**

```typescript
import 'reflect-metadata';
import { NestFactory } from '@nestjs/core';
import { ValidationPipe } from '@nestjs/common';
import { AppModule } from './app.module';

async function bootstrap() {
  const app = await NestFactory.create(AppModule);
  app.useGlobalPipes(new ValidationPipe({ whitelist: true }));
  await app.listen(process.env.PORT ?? 3000);
}

bootstrap();
```

- [ ] **Step 9: Run the tests to verify they pass**

Run: `cd server && pnpm test:e2e`
Expected: PASS — both `Fleet (e2e)` cases.

- [ ] **Step 10: Stage for commit**

Per this project's commit rule (see Global Constraints), stage only — the user commits.

```bash
git add server
```

---

### Task 4: `POST /fleet/agents/:id/heartbeat` endpoint

**Files:**
- Modify: `server/src/modules/fleet/fleet.service.ts`
- Modify: `server/src/modules/fleet/fleet.controller.ts`
- Modify: `server/test/fleet.e2e-spec.ts`

**Interfaces:**
- Consumes: `FleetService` from Task 3 (adds a method, does not change `registerAgent`'s signature).
- Produces: `FleetService.heartbeat(id: string): Promise<Agent>` — throws `NotFoundException` if `id` doesn't exist.
- Produces: `POST /fleet/agents/:id/heartbeat` → 201, body `{ id, hostname, status: 'online', lastSeenAt: <ISO date>, createdAt }`. Unknown `id` → 404.

- [ ] **Step 1: Write the failing tests — append to `server/test/fleet.e2e-spec.ts`**

Add these two `it` blocks inside the existing `describe('Fleet (e2e)', ...)` block, after the two tests already there:

```typescript
  it('POST /fleet/agents/:id/heartbeat marks the agent online', async () => {
    const registerResponse = await request(app.getHttpServer())
      .post('/fleet/agents')
      .send({ hostname: 'lab-box-02' });
    const agentId = registerResponse.body.id;

    const response = await request(app.getHttpServer()).post(
      `/fleet/agents/${agentId}/heartbeat`,
    );

    expect(response.status).toBe(201);
    expect(response.body.status).toBe('online');
    expect(response.body.lastSeenAt).not.toBeNull();

    await prisma.agent.delete({ where: { id: agentId } });
  });

  it('POST /fleet/agents/:id/heartbeat returns 404 for an unknown agent', async () => {
    const response = await request(app.getHttpServer()).post(
      '/fleet/agents/00000000-0000-0000-0000-000000000000/heartbeat',
    );

    expect(response.status).toBe(404);
  });
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `cd server && pnpm test:e2e`
Expected: FAIL — `404` on the heartbeat route for the first new test (route doesn't exist), and the second test fails because the route returns 404 for the wrong reason (no route at all, not the "not found" branch) — both currently fail for the same missing-route reason.

- [ ] **Step 3: Add `heartbeat` to `server/src/modules/fleet/fleet.service.ts`**

```typescript
import { Injectable, NotFoundException } from '@nestjs/common';
import { Prisma } from '@prisma/client';
import { PrismaService } from '../../prisma/prisma.service';
import { RegisterAgentDto } from './dto/register-agent.dto';

@Injectable()
export class FleetService {
  constructor(private readonly prisma: PrismaService) {}

  registerAgent(dto: RegisterAgentDto) {
    return this.prisma.agent.create({
      data: { hostname: dto.hostname },
    });
  }

  async heartbeat(id: string) {
    try {
      return await this.prisma.agent.update({
        where: { id },
        data: { status: 'online', lastSeenAt: new Date() },
      });
    } catch (error) {
      if (error instanceof Prisma.PrismaClientKnownRequestError && error.code === 'P2025') {
        throw new NotFoundException(`Agent ${id} not found`);
      }
      throw error;
    }
  }
}
```

- [ ] **Step 4: Add the route in `server/src/modules/fleet/fleet.controller.ts`**

```typescript
import { Body, Controller, Param, Post } from '@nestjs/common';
import { FleetService } from './fleet.service';
import { RegisterAgentDto } from './dto/register-agent.dto';

@Controller('fleet/agents')
export class FleetController {
  constructor(private readonly fleetService: FleetService) {}

  @Post()
  register(@Body() dto: RegisterAgentDto) {
    return this.fleetService.registerAgent(dto);
  }

  @Post(':id/heartbeat')
  heartbeat(@Param('id') id: string) {
    return this.fleetService.heartbeat(id);
  }
}
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `cd server && pnpm test:e2e`
Expected: PASS — all four `Fleet (e2e)` cases.

- [ ] **Step 6: Run the full test suite**

Run: `cd server && pnpm test && pnpm test:e2e`
Expected: PASS — health, Prisma, and fleet tests all green.

- [ ] **Step 7: Stage for commit**

Per this project's commit rule (see Global Constraints), stage only — the user commits.

```bash
git add server
```

---

## After This Plan

This delivers Phase 1, build-order step 1 only (control plane running with Postgres + register/heartbeat endpoints). Next in the roadmap: build-order step 2, "minimal agent that enrolls and sends heartbeats from a lab machine" — a Go binary calling these two endpoints — which is a separate plan under `agent/`.
