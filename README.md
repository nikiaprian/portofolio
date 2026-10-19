## Bebii Blog (Go, Gin, GORM, MySQL)

A **Go** blog application using **Gin**, **GORM**, and **MySQL**: a `/api/v1` REST API, **JWT** authentication, schema migrations via GORM `AutoMigrate`, development seed data, **server-rendered HTML** (public site, admin dashboard, content management pages), and a configurable **news scraper** that imports articles from RSS/Atom feeds.

---

### 1. Prerequisites

- Go **1.23+**
- MySQL (local or Docker)
- Git (optional)

---

### 2. Clone the repository

```bash
git clone https://github.com/Bebii-Digital/bebii-blog.git
cd bebii-blog
```

Skip this step if you already have the project directory.

---

### 3. Environment

Copy the example environment file:

```bash
cp .env.example .env
# Windows PowerShell
copy .env.example .env
```

Edit `.env` for your database mode and JWT settings:

```env
APP_ENV=development
HTTP_PORT=8080

DB_DRIVER=mysql
DB_HOST=127.0.0.1
DB_PORT=3306
DB_USER=root
DB_PASSWORD=root
DB_NAME=bebii_blog

JWT_SECRET=change-me-to-a-long-random-string
JWT_EXPIRES_IN_HOURS=24
```

`DB_DRIVER` options:
- `mysql` (default): uses DB_HOST/DB_PORT/DB_USER/DB_PASSWORD/DB_NAME.

If you use MySQL, create an empty database:

```sql
CREATE DATABASE bebii_blog CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
```

---

### 4. Go dependencies

```bash
go mod tidy
```

#### Image resize (self-hosted, govips optional)

**Delivery (browser):** With **Image resize** enabled in site settings, public images under `/static/uploads/` and `/static/images/` are served via `GET /img?src=...&w=...&q=...`. Width can follow the rendered size (responsive thumbnails in JS). Output is WebP when possible.

**Storage (upload/scraper):** `stored_image_max_width` caps files on disk. Optional **[govips](https://github.com/davidbyttow/govips)** (`-tags vips` + libvips) speeds resize/encode on the server.

**Ubuntu / Debian (recommended for production):**

```bash
sudo apt install -y libvips-dev pkg-config build-essential
CGO_ENABLED=1 go build -tags vips -o bebii-blog ./cmd
```

**Local dev without libvips:** default build still provides `/img` using the Go image stack:

```bash
go run ./cmd
```

---

### 5. Run the server

From the project root:

```bash
go run ./cmd
```

### Run seed only (manual)

```bash
go run ./cmd --seed-only
```

Command ini akan connect DB, jalankan migration, lalu eksekusi `seeds.Run(db)` tanpa menyalakan HTTP server.

On startup, the app:

1. Loads `.env` with `godotenv`.
2. Connects to MySQL via `db.Connect(cfg)`.
3. Runs `migrations.Run(db)` — `AutoMigrate` for:
   - `users`, `categories`, `tags`, `posts`, `post_tags`, `post_views`, `pages`
   - `site_settings`, `site_connections`, `site_cloud_storages`, `site_web_image_providers`
   - `scraper_settings`, `scraper_category_feeds`, `scraper_logs`
   - `login_logs`, `bulk_post_logs`, `manual_post_ai_logs`, `ai_settings`
4. If `APP_ENV=development` and there are no rows in site settings yet (seed has not run), runs `seeds.Run(db)`:
   - Creates seed admin `admin@bebii.com` / `admin123` (only when that email does not exist yet).
   - Categories, tags, sample posts, pages, default site/SEO settings, initial scraper setting, AI setting.

**Admin account for local testing:** `api.http` and the admin UI use **`admin@bebii.com`** / **`admin123`** (typically user id `1`). On a fresh seed-only database, use the seed account above until you create or promote an admin with that email.

The server listens at `http://localhost:{HTTP_PORT}` (default **8080**). If the **`PORT`** environment variable is set, it is used for binding instead of `HTTP_PORT`.

---

### 6. Web pages (HTML)

| Path | Description |
|------|-------------|
| `/` | Home (client layout) |
| `/login` | Sign-in (JWT in `localStorage`). Local dev: `admin@bebii.com` / `admin123` |
| `/admin/dashboard` | Dashboard |
| `/admin/profile` | Profile (admin or writer) |
| `/admin/users` | Users (admin only) |
| `/admin/pages` | Static pages list; `/admin/pages/create`, `/admin/pages/:id/edit` |
| `/admin/categories` | Categories (admin only) |
| `/admin/tags` | Tags (admin or writer) |
| `/admin/posts` | Posts list; `/admin/posts/create`, `/admin/posts/:id/edit` |
| `/admin/bulk-posts` | Bulk title + generate UI |
| `/admin/bulk-post-logs` | Bulk generate logs (writers see own rows) |
| `/admin/manual-post-ai-logs` | Manual post AI logs |
| `/admin/site-settings` | Branding, SEO defaults, image resize (admin only) |
| `/admin/ai-setting` | AI keys, models, scraper + generate prompts (admin only) |
| `/admin/scraper-settings` | RSS scraper feeds (admin only) |
| `/admin/scraper-logs` | Scraper run logs (admin only) |
| `/admin/login-logs` | Sign-in audit (admin only) |
| `/admin/restore-backup` | Database export/import UI (admin only) |

Static assets are served from `/static` (`public/static`).

---

### 7. Main API

- **`GET /health`** — health check (not under `/api`).

API prefix: **`/api/v1`**. Protected admin routes require **`Authorization: Bearer <token>`** from `POST /api/v1/auth/login`. JWTs are stateless (logout does not revoke tokens server-side).

**Public (no JWT)**

| Method | Path | Notes |
|--------|------|--------|
| GET | `/api/v1/posts` | Published posts (`page`, `size`) |
| GET | `/api/v1/posts/popular` | Popular published posts |
| GET | `/api/v1/posts/slug/:slug` | Single published post |
| GET | `/api/v1/pages/slug/:slug` | Public static page |
| GET | `/api/v1/categories` | Category list |
| GET | `/api/v1/tag/:slug` | Public tag by slug |
| GET | `/api/v1/authors/:slug` | Public author profile |
| POST | `/api/v1/auth/register` | Creates **writer** account |
| POST | `/api/v1/auth/login` | Returns `{ token, user }` |
| POST | `/api/v1/auth/logout` | Client should discard token |

**Admin — authenticated (admin or writer)**

| Method | Path |
|--------|------|
| GET | `/api/v1/admin/profile` |
| PUT | `/api/v1/admin/profile` |
| POST | `/api/v1/admin/profile/upload-image` |

**Admin — role `admin` only**

| Method | Path |
|--------|------|
| GET, PUT | `/api/v1/admin/site-setting` |
| PUT | `/api/v1/admin/site-setting/scripts` |
| POST | `/api/v1/admin/site-setting/upload-image` |
| POST | `/api/v1/admin/site-setting/post-date-url-recycle/run` |
| GET, PUT | `/api/v1/admin/site-connections` |
| POST | `/api/v1/admin/site-connections/test-autopost` |
| GET, PUT | `/api/v1/admin/site-cloud-storage` |
| GET, PUT | `/api/v1/admin/site-web-image-providers` |
| GET, PUT | `/api/v1/admin/ai-setting` |
| GET, PUT | `/api/v1/admin/scraper-setting` |
| POST | `/api/v1/admin/scraper-setting/run-now` |
| GET, DELETE | `/api/v1/admin/scraper-logs` |
| GET, DELETE | `/api/v1/admin/login-logs` |
| DELETE | `/api/v1/admin/bulk-post-logs` |
| DELETE | `/api/v1/admin/manual-post-ai-logs` |
| GET | `/api/v1/admin/database/export` |
| POST | `/api/v1/admin/database/import` |
| GET, POST | `/api/v1/admin/users` |
| GET, PUT, DELETE | `/api/v1/admin/users/:id` |
| GET, POST | `/api/v1/admin/pages` |
| GET, PUT, DELETE | `/api/v1/admin/pages/:id` |
| POST | `/api/v1/admin/pages/upload-image` |
| POST | `/api/v1/admin/pages/bulk-status` |
| POST | `/api/v1/admin/categories` |
| GET, PUT, DELETE | `/api/v1/admin/categories/:id` |

**Admin — role `admin` or `writer`**

| Method | Path |
|--------|------|
| GET | `/api/v1/admin/categories` |
| GET, POST | `/api/v1/admin/tags` |
| GET, PUT, DELETE | `/api/v1/admin/tags/:id` |
| GET, POST | `/api/v1/admin/posts` |
| GET, PUT, DELETE | `/api/v1/admin/posts/:id` |
| POST | `/api/v1/admin/posts/upload-image` |
| POST | `/api/v1/admin/posts/bulk-status` |
| POST | `/api/v1/admin/posts/generate-draft` |
| POST | `/api/v1/admin/posts/generate-image` |
| POST | `/api/v1/admin/posts/bulk-suggest-titles` |
| POST | `/api/v1/admin/posts/bulk-regenerate-titles` |
| POST | `/api/v1/admin/posts/bulk-scrape-xml-titles` |
| POST | `/api/v1/admin/posts/bulk-check-titles` |
| POST | `/api/v1/admin/posts/bulk-generate` |
| GET | `/api/v1/admin/posts/bulk-generate-jobs-active` |
| GET | `/api/v1/admin/posts/bulk-generate-jobs/:id` |
| POST | `/api/v1/admin/posts/bulk-generate-jobs/:id/cancel` |
| PUT | `/api/v1/admin/ai-setting/bulk-post-template` |
| POST | `/api/v1/admin/ai-setting/upload-bulk-cover-background` |
| GET | `/api/v1/admin/bulk-post-logs` |
| GET | `/api/v1/admin/manual-post-ai-logs` |

Writers can manage their own posts via the post endpoints; list/detail rules are enforced in handlers.

---

### 8. Testing the API

**`api.http`** (repo root) contains sample requests for REST Client (VS Code / Cursor) or compatible `.http` clients.

1. Start the server (`go run ./cmd`).
2. Adjust variables at the top of `api.http` if needed:
   - `@baseUrl` — default `http://localhost:8080`
   - `@adminEmail` / `@adminPassword` — default **`admin@bebii.com`** / **`admin123`**
   - `@userId`, `@categoryId`, `@tagId`, `@postId`, `@pageId` — resource ids for GET/PUT/DELETE (default `1`)
3. Run the **Login** request (`# @name login`). Copy the `token` field from the JSON response into `@token`.
4. Run any admin request. Full admin-only routes (users, pages, category write, site/scraper/AI settings, log clears, DB backup) need an **`admin`** JWT; tags, posts, and bulk/manual AI endpoints also allow **`writer`**.

`api.http` sections: **PUBLIC**, **AUTH**, **USERS**, **CATEGORIES**, **TAGS**, **AI SETTING**, **SITE SETTING**, **SITE CONNECTIONS**, **SCRAPER**, scraper/login/bulk/manual AI **LOGS**, **BULK POST**, **POSTS** (CRUD + generate-draft/image), **PAGES**. Not every route in §7 has a sample yet (e.g. `site-cloud-storage`, `database/export`); add requests there as needed.

Prompt templates in JSON bodies use Unicode escapes (`\u007b\u007btitle\u007d\u007d`) so the HTTP client does not treat `{{title}}` as a file variable; the API still receives literal `{{title}}` placeholders.

---

### 9. News scraper overview

The app includes a background **news scraper** that imports articles from RSS/Atom feeds into published posts.

- Configuration tables:
  - `scraper_settings` — singleton row: master **enabled** flag, run interval (minutes), and author email.
  - `scraper_category_feeds` — one row per category: `category_slug`, `feed_urls` (comma-separated), and per-category **enabled** flag.
- Admin UI:
  - `/admin/ai-setting` — provider keys (Gemini, OpenAI, Groq, Claude) and prompts for scraper paraphrase + manual/bulk generate.
  - `/admin/scraper-settings` — scraper master switch and per-category feed URLs:
    - **Enable scraper (master)** — turns the worker on/off and toggles all category switches.
    - `Interval (minutes)` — how often the worker runs.
    - `Author email` — which user will be used as the author for scraped posts.
    - Per-category blocks:
      - **Enabled** toggle for that category.
      - Textarea of comma-separated RSS/Atom URLs.
- Behavior (per run):
  - Only runs when:
    - Scraper is globally **enabled**.
    - AI provider is configured and has a valid key (see `/admin/ai-setting`).
  - For each category with **Enabled + non-empty feed URLs**:
    - Downloads each feed and skips items whose `source` link already exists in `posts`.
    - Optionally fetches the full article HTML when feed content is too short.
    - Picks a hero image from feed HTML `<img>` or article `og:image` / `twitter:image`.
    - Extracts meta keywords from the article and:
      - Stores them in `MetaKeywords`.
      - Tries to attach up to **3 existing tags** (by slug) to the new post.
    - Paraphrases title + body via the configured AI provider and saves a **published** post.

See `api.http` under **SCRAPER** for example requests to `/api/v1/admin/scraper-setting` (`author_email` uses `{{adminEmail}}`, i.e. `admin@bebii.com`).

---

### 10. Project structure (overview)

Backend layering: **Handler → Service → Repository → Entity** (see `docs/architecture.md` and `.cursor/project-architecture.md`).

```text
cmd/
  main.go                      # entrypoint: env, DB, migrate, seed, workers, Gin

config/
  config.go                    # env (DB, JWT, Redis cache, ports, retention, …)

db/
  database.go                  # GORM connect
  migrations/                  # AutoMigrate + schema tweaks
  seeds/                       # dev/sample data (users, posts, AI/scraper defaults, …)

internal/
  entity/                      # GORM models (user, category, tag, post, page, site_*, logs, ai_setting)
  repositories/                # DB access only
  services/                    # business logic
  routes/
    route.go                   # REST /api/v1 (public + /admin)
    frontend_route.go          # SSR HTML (client + /admin), /static, /img resize
    client_template_cache.go   # parsed template cache
    handler/                   # HTTP handlers (auth, CRUD, AI, scraper, logs, backup, …)
    dto/                       # request/response validation structs
  middleware/                  # JWT, roles, rate limits, gzip helpers
  ai/                          # multi-provider generator (gemini, openai, groq, claude)
  scraper/                     # RSS news scraper + bulk image/cover helpers
  social/                      # autopost to webhooks (site connections)
  media/                       # image resize (govips optional), favicon
  storage/                     # local uploads + optional R2
  cache/                       # Redis HTTP cache helpers
  helper/                      # post HTML, meta, titlesim, autohref, scheduled publish, log prune, …

public/
  static/                      # CSS/JS (client + admin), vendor (e.g. TinyMCE)
  templates/
    layouts/                   # client + admin shells
    pages/client/              # home, post, category, tag, author, page, …
    pages/admin/               # dashboard, CRUD, AI/scraper/logs, backup, …
    pages/auth/                # login
    partials/                  # reusable fragments

data/cache/                    # runtime file cache (gitignored)
docs/                          # architecture.md, feature-map.md
scripts/                       # production setup/deploy helpers
tests/                         # integration tests

.env.example                   # environment template
api.http                       # REST Client samples (admin + public)
go.mod
```

**API vs UI:** JSON admin API lives under `internal/routes/route.go`; matching admin pages under `public/templates/pages/admin/` and `frontend_route.go`. Shared AI/scraper config is the singleton `ai_settings` row plus `scraper_settings` / `scraper_category_feeds`.

---

### 11. Production notes

- Set `APP_ENV` to something other than `development` so **seeds do not run**.
- If you need initial admin/sample data in production bootstrap, set `RUN_SEED_ON_STARTUP=true` for first start only, then set it back to `false`.
- Set `GIN_MODE=release` and use a strong, unique `JWT_SECRET`.
- Make sure scraper master + AI provider are configured before enabling news scraping in production.
- Do not rely on seeded credentials; rotate passwords and tighten access as needed.
