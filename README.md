# Shadcn Admin Dashboard

Admin Dashboard UI crafted with Shadcn and Vite. Built with responsiveness and accessibility in mind.

![alt text](public/images/shadcn-admin.png)

[![Sponsored by Clerk](https://img.shields.io/badge/Sponsored%20by-Clerk-5b6ee1?logo=clerk)](https://go.clerk.com/GttUAaK)

I've been creating dashboard UIs at work and for my personal projects. I always wanted to make a reusable collection of dashboard UI for future projects; and here it is now. While I've created a few custom components, some of the code is directly adapted from ShadcnUI examples.

> This is not a starter project (template) though. I'll probably make one in the future.

## Features

- Light/dark mode
- Responsive
- Accessible
- With built-in Sidebar component
- Global search command
- 10+ pages
- Extra custom components
- RTL support

<details>
<summary>Customized Components (click to expand)</summary>

This project uses Shadcn UI components, but some have been slightly modified for better RTL (Right-to-Left) support and other improvements. These customized components differ from the original Shadcn UI versions.

If you want to update components using the Shadcn CLI (e.g., `npx shadcn@latest add <component>`), it's generally safe for non-customized components. For the listed customized ones, you may need to manually merge changes to preserve the project's modifications and avoid overwriting RTL support or other updates.

> If you don't require RTL support, you can safely update the 'RTL Updated Components' via the Shadcn CLI, as these changes are primarily for RTL compatibility. The 'Modified Components' may have other customizations to consider.

### Modified Components

- scroll-area
- sonner
- separator

### RTL Updated Components

- alert-dialog
- calendar
- command
- dialog
- dropdown-menu
- select
- table
- sheet
- sidebar
- switch

**Notes:**

- **Modified Components**: These have general updates, potentially including RTL adjustments.
- **RTL Updated Components**: These have specific changes for RTL language support (e.g., layout, positioning).
- For implementation details, check the source files in `src/components/ui/`.
- All other Shadcn UI components in the project are standard and can be safely updated via the CLI.

</details>

## Tech Stack

**UI:** [ShadcnUI](https://ui.shadcn.com) (TailwindCSS + RadixUI)

**Build Tool:** [Vite](https://vitejs.dev/)

**Routing:** [TanStack Router](https://tanstack.com/router/latest)

**Type Checking:** [TypeScript](https://www.typescriptlang.org/)

**Linting/Formatting:** [ESLint](https://eslint.org/) & [Prettier](https://prettier.io/)

**Icons:** [Lucide Icons](https://lucide.dev/icons/), [Tabler Icons](https://tabler.io/icons) (Brand icons only)

**Auth (partial):** [Clerk](https://go.clerk.com/GttUAaK)

## Run Locally

This repo is a pnpm monorepo:

- `apps/web` — React 19 frontend (Vite + TanStack Router + Shadcn)
- `apps/api` — Go 1.25 backend (Gin + GORM + PostgreSQL)
- `packages/shared` — shared types and OpenAPI schema (placeholder)

### Prerequisites

- Node 22+, pnpm 10+
- Go 1.25+
- Docker Desktop (for postgres + full stack via `docker compose`)

### One-shot dev environment

```bash
make up          # builds images, starts postgres+api+web
curl localhost:8080/health
open http://localhost:5173
make down        # stop
```

### Hot-reload dev (recommended)

```bash
docker compose up -d postgres   # only postgres
make api                        # go run backend, watches nothing — restart manually
make dev                        # vite dev server (HMR)
```

### Tests

```bash
make test
cd apps/api && go test -tags=integration ./...   # requires Docker
```

### Auth smoke test (manual)

```bash
make up
sleep 15   # wait for migrations + bootstrap admin

# login; cookie jar gets tm_access_token / tm_refresh_token / tm_csrf_token
curl -sS -c /tmp/tm-cookies.txt \
     -H 'Content-Type: application/json' \
     -d '{"email":"admin@example.com","password":"change-me-on-first-login"}' \
     http://localhost:8080/api/v1/auth/login

# current user
curl -sS -b /tmp/tm-cookies.txt http://localhost:8080/api/v1/auth/me

# admin: list users (needs CSRF for non-GET; GET omitted)
curl -sS -b /tmp/tm-cookies.txt 'http://localhost:8080/api/v1/admin/users'

make down
rm /tmp/tm-cookies.txt
```

## Sponsoring this project ❤️

If you find this project helpful or use this in your own work, consider [sponsoring me](https://github.com/sponsors/satnaing) to support development and maintenance. You can [buy me a coffee](https://buymeacoffee.com/satnaing) as well. Don’t worry, every penny helps. Thank you! 🙏

For questions or sponsorship inquiries, feel free to reach out at [satnaingdev@gmail.com](mailto:satnaingdev@gmail.com).

### Current Sponsor

- [Clerk](https://go.clerk.com/GttUAaK) - authentication and user management for the modern web

## Author

Crafted with 🤍 by [@satnaing](https://github.com/satnaing)

## License

Licensed under the [MIT License](https://choosealicense.com/licenses/mit/)
