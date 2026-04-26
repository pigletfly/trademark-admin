# @trademark/e2e

Playwright E2E covering the full business happy path:
login → customer → wizard → submit → review → adjust → approve → download PDF.

## Prerequisites

Start the full stack first:

    docker compose up -d

Then verify the API is healthy:

    curl http://localhost:8080/health

The compose file already provides a bootstrap admin
(`admin@example.com` / `change-me-on-first-login`) — no extra seeding needed.

Install workspace dependencies (first time only):

    pnpm install

Install the browser binary (first time only):

    pnpm -C packages/e2e install:browsers

## Running

From repo root:

    make e2e                # preflight + run all 4 specs
    pnpm -C packages/e2e test           # skip preflight
    pnpm -C packages/e2e test:headed    # watch the browser
    pnpm -C packages/e2e test:ui        # interactive UI mode
    pnpm -C packages/e2e report         # open last HTML report

## Execution order

Tests run serially via `fullyParallel: false` + `workers: 1`. Files are picked
up in filename order:

1. `01-admin-setup.spec.ts` — bootstrap admin logs in via API, creates 2
   users (salesperson, reviewer), 2 pricing entries, 1 customer. Writes
   `state/test-run.json`.
2. `02-salesperson-journey.spec.ts` — UI login as salesperson, runs the 5-step
   wizard, saves + submits. Writes `quotationId` back into the state file.
3. `03-reviewer-journey.spec.ts` — UI login as reviewer, adjusts pricing,
   approves.
4. `04-export.spec.ts` — UI login as salesperson, triggers PDF bilingual
   export, asserts the download URL request fires.

If any spec fails, later specs won't find the expected state and will fail
with "Run 01-admin-setup first" or similar. Rerun from `01-` after fixing.

## Data isolation

Each run generates a random 8-hex suffix (e.g. `ab12cd34`). All created
entities carry it (`salesperson-ab12cd34@example.com`, `客户-ab12cd34 有限公司`, etc).
Multiple runs accumulate data in the DB but never collide. If you want a
clean DB:

    docker compose down -v && docker compose up -d

## Debugging a failure

    pnpm -C packages/e2e report     # HTML: screenshots, video, trace viewer

Click through to the failing test for trace + screenshot + console output.

## Troubleshooting

### "API not responding" from `make e2e`

Check compose status:

    docker compose ps

If api is unhealthy, tail logs:

    docker compose logs api

### "quotationId missing — run 02-salesperson-journey first"

Spec 03 or 04 executed before 02 succeeded. Start fresh:

    pnpm -C packages/e2e test

### Flaky "timeout waiting for /quotations/<uuid>"

If preview/submit is slow on first run (cold postgres), bump the wizard
timeout in `fixtures/pages/wizard.page.ts` `saveAndSubmit`, or restart
the compose stack with a warm DB.

### Browser binary not found

    pnpm -C packages/e2e install:browsers

### Port 3000 already allocated (gotenberg can't start)

OrbStack or another container runtime may have a separate container
holding port 3000. Either free the port, or modify `docker-compose.yml`
to `expose: ["3000"]` instead of `ports: ["3000:3000"]` — the api
container reaches gotenberg on the docker network regardless.

### DB gets polluted across runs

Each run generates a fresh 8-hex suffix, so test data never collides. But
rows accumulate. Clean slate:

    docker compose down -v && docker compose up -d
