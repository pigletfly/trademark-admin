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
