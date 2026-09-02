# Bubble White — Backend API

Go + Gin + GORM + MySQL backend with role-based access control (RBAC), an
admin panel API for managing products/categories/company settings, and
image uploads to Cloudflare R2.

## Stack

- Go 1.22+, [Gin](https://gin-gonic.com/), [GORM](https://gorm.io/) + MySQL 8
- JWT auth ([golang-jwt/jwt](https://github.com/golang-jwt/jwt)) + bcrypt
- [go-playground/validator](https://github.com/go-playground/validator) for request validation
- [aws-sdk-go-v2](https://github.com/aws/aws-sdk-go-v2) `s3` client, pointed at Cloudflare R2 (S3-compatible)
- Swagger UI at `/docs`, spec at `docs/openapi.yaml`

## Project layout

```
cmd/server/main.go     entrypoint — loads config, connects DB + R2, seeds, starts server
config/                config singleton, DB connection, JWT + R2 client setup
routes/                DI wiring (repo -> service -> controller) + one file per resource
controllers/           HTTP layer — parse request, call service, respond via utils.Envelope
services/               business logic
repositories/          GORM data access (generic base + per-resource queries)
models/                GORM models, incl. generic JSONColumn[T] for array/JSON fields
dto/                   request/response structs (auth, shared pagination)
middlewares/           JWT auth, permission checks, CORS
utils/                 response envelope, pagination, JWT, bcrypt, validator, R2 uploader
seed/seed.go            permission catalog + default roles/admin/settings/catalog on first boot
migrations/init.sql    reference SQL schema (app self-migrates via GORM; this is for inspection)
docs/                  openapi.yaml + Swagger UI, served at /docs
docker-compose.yml     MySQL 8 + phpMyAdmin for local dev
Makefile               make run / build / up / down / fmt / vet
```

## Setup

```bash
cp .env.example .env
# fill in DB + R2 credentials

make up             # starts MySQL + phpMyAdmin (docker compose)
go mod tidy
make run            # or: go run ./cmd/server
```

On first boot the app seeds itself: the full permission catalog, two
built-in roles (`admin` = every permission, `editor` = product/category/
contact only), one admin login (from `SEED_ADMIN_EMAIL` /
`SEED_ADMIN_PASSWORD` in `.env`), a default company-settings row, and a
handful of starter categories/products so the API isn't empty.

**Log in immediately after first boot and change the seeded password** via
`POST /api/admin/me/change-password`.

Swagger UI: `http://localhost:8080/docs`
Health check: `GET /health`

## Role-based access control

- **Permission** — a fixed action slug like `product.update`. The full
  catalog lives in code at `seed.PermissionCatalog` and is synced into the
  `permissions` table on every boot, so it can't drift from what
  `middlewares.RequirePermission(...)` actually checks in the route files.
- **Role** — a name (e.g. "Editor") plus a JSON array of permission slugs
  (`Role.Permissions`). The special slug `"*"` (used by the built-in
  `admin` role) grants everything.
- **User** — has one `RoleID`. To change what a staff member can do,
  either edit their role's permissions (`PUT /api/admin/roles/:id`) or move
  them to a different role (`PATCH /api/admin/users/:id/role`).
- Every admin route is `AuthMiddleware()` (valid JWT) +
  `RequirePermission(db, "some.slug")` (role must grant that slug). See any
  file under `routes/` for the exact permission each endpoint requires.

Create new roles from the admin panel — `POST /api/admin/roles` with a
`permissions: ["product.view", "product.update", ...]` array, built from
whatever `GET /api/admin/permissions` returns.

## Image uploads (Cloudflare R2)

`POST /api/admin/uploads/products` (or `/categories`, `/site`) accepts a
`multipart/form-data` field named `file`. The server:

1. Sniffs the **actual file content** (not the filename/extension) to
   confirm it's really a JPEG/PNG/WEBP/GIF — rejects anything else.
2. Uploads it to the configured R2 bucket under `products/`, `categories/`,
   or `site/`.
3. Returns `{ "url": "...", "key": "...", ... }` — save that `url` onto the
   product/category (`images` array) or settings (`logoUrl`) via their
   normal update endpoints.

Set `R2_ACCOUNT_ID`, `R2_ACCESS_KEY_ID`, `R2_SECRET_ACCESS_KEY`,
`R2_BUCKET`, `R2_ENDPOINT`, and `R2_PUBLIC_URL` in `.env` — see
`.env.example` for the exact endpoint URL pattern and where to find these
in the Cloudflare dashboard.

## Company info & contact details (admin-editable)

`GET /api/settings` is public — the storefront reads company name, About
page detail text, contact email/phone/address/hours, social links, and
logo URL from here instead of hard-coding them in the frontend.

`PUT /api/admin/settings` (requires `settings.update`) lets an admin change
any of those fields — it's a partial patch, so sending just
`{ "contactPhone": "..." }` only touches that one field.

## API summary

All routes are under `/api`. Public: `POST /auth/login`, `GET /products`,
`GET /products/:id`, `GET /products/:id/related`, `GET /categories`,
`GET /categories/:id`, `GET /settings`, `POST /contact`. Everything else is
under `/api/admin/*` and requires a Bearer token plus the specific
permission noted in `routes/*.go`. Full request/response shapes are in
`docs/openapi.yaml` (or just open `/docs` once the server is running).

## Wiring up the frontend

The frontend currently imports `products`/`categories` as static arrays
from `src/data/products.js`. To switch it over:

1. Add `VITE_API_URL` to the frontend's env, pointing at this API.
2. Replace the static imports in `HomeView.vue` / `ShopView.vue` /
   `ProductView.vue` with `fetch` calls to `/api/products`,
   `/api/products/:id`, `/api/products/:id/related`, `/api/categories`.
3. Point the Contact page's submit handler at `POST /api/contact`.
4. Have the footer/contact page pull company info from `GET /api/settings`
   instead of hard-coded Khmer strings.

Happy to do that wiring as a follow-up.

## Notes / limitations

- Written and syntax-checked (`gofmt` — parses clean on all 55 files) in a
  sandboxed environment with no access to `proxy.golang.org` or
  `golang.org`, so `go mod tidy` / `go build` couldn't fully run here.
  Direct GitHub-hosted dependencies (gin, cors, gorm, etc.) resolved fine
  when tested with `GOPROXY=direct`; only a deep transitive chain
  (`golang.org/x/crypto` and friends, pulled in as *test-only* deps of
  gin/validator/aws-sdk) hit the sandbox's network allowlist. Run
  `go mod tidy && go run ./cmd/server` on your own machine as the first
  step — normal internet access means this is a non-issue there.
- No refresh-token flow — access tokens just expire after
  `JWT_EXPIRES_HOURS` and the admin logs in again. Add refresh tokens if
  the admin panel needs longer unattended sessions.
- `role.HasPermission` treats `"*"` as "grants everything" — only the
  seeded `admin` role has it; don't hand that slug to a role you don't
  fully trust.
