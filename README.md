## Tirtayasa (Go, Gin, GORM, MySQL)

A **Go** blog application using **Gin**, **GORM**, and **MySQL**: a `/api/v1` REST API, **JWT** authentication, schema migrations via GORM `AutoMigrate`, development seed data, **server-rendered HTML** (public site, admin dashboard, content management pages), and a configurable **news scraper** that imports articles from RSS/Atom feeds.

---

### 1. Prerequisites

- Go **1.23+**
- MySQL (local or Docker)
- Git (optional)

---

### 2. Clone the repository

```bash
git clone -b blog https://github.com/nikiaprian/portofolio.git tirtayasa
cd tirtayasa
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
DB_NAME=tirtayasa

JWT_SECRET=change-me-to-a-long-random-string
JWT_EXPIRES_IN_HOURS=24
```

`DB_DRIVER` options:
- `mysql` (default): uses DB_HOST/DB_PORT/DB_USER/DB_PASSWORD/DB_NAME.

If you use MySQL, create an empty database:

```sql
CREATE DATABASE tirtayasa CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
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
CGO_ENABLED=1 go build -tags vips -o tirtayasa ./cmd
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
   - `users`, `categories`, `tags`, `posts`, the **`post_tags`** pivot table
   - `site_settings`, `site_connections`
   - `scraper_settings`, `scraper_category_feeds`
   - `ai_settings`
4. If `APP_ENV=development` and there are no rows in site settings yet (seed has not run), runs `seeds.Run(db)`:
   - Sample admin: `admin@tirtayasa.id` / `admin123` (development only).
   - Categories, tags, sample posts, pages, default site/SEO settings, initial scraper setting, AI setting.

The server listens at `http://localhost:{HTTP_PORT}` (default **8080**). If the **`PORT`** environment variable is set, it is used for binding instead of `HTTP_PORT`.

---

### 6. Web pages (HTML)

| Path | Description |
|------|-------------|
| `/` | Home (client layout) |
| `/login` | Sign-in page (stores JWT in `localStorage`, then redirects to dashboard) |
| `/admin/dashboard` | Admin dashboard (UI) |
| `/admin/users` | User list / management UI |
| `/admin/categories` | Categories UI |
| `/admin/tags` | Tags UI |
| `/admin/posts` | Posts UI (includes AI generate-draft) |
| `/admin/site-settings` | Site settings UI |
| `/admin/ai-setting` | AI provider keys + news scraper prompts |
| `/admin/scraper-settings` | News scraper per-category feed settings |

Static assets are served from `/static` (`public/static`).

---

### 7. Main API

- **`GET /health`** — health check (not under `/api`).

API prefix: **`/api/v1`**.

**Public (no JWT)**

| Method | Path |
|--------|------|
| POST | `/api/v1/auth/register` |
| POST | `/api/v1/auth/login` |
| POST | `/api/v1/auth/logout` |

**Admin (requires `Authorization: Bearer <token>`)**

| Method | Path |
|--------|------|
| GET | `/api/v1/admin/site-setting` |
| PUT | `/api/v1/admin/site-setting` |
| GET | `/api/v1/admin/site-connections` |
| PUT | `/api/v1/admin/site-connections` |
| GET | `/api/v1/admin/ai-setting` |
| PUT | `/api/v1/admin/ai-setting` |
| GET | `/api/v1/admin/scraper-setting` |
| PUT | `/api/v1/admin/scraper-setting` |
| GET, POST | `/api/v1/admin/users` |
| GET, PUT, DELETE | `/api/v1/admin/users/:id` |
| GET, POST | `/api/v1/admin/categories` |
| GET, PUT, DELETE | `/api/v1/admin/categories/:id` |
| GET, POST | `/api/v1/admin/tags` |
| GET, PUT, DELETE | `/api/v1/admin/tags/:id` |
| GET, POST | `/api/v1/admin/posts` |
| GET, PUT, DELETE | `/api/v1/admin/posts/:id` |

**Logout** (`POST /api/v1/auth/logout`) returns success so clients can clear storage; JWTs are not revoked server-side (stateless).

---

### 8. Testing the API

- **`api.http`**: sample requests at the repo root for REST Client (VS Code / Cursor). Adjust `@baseUrl` if you change the port.

---

### 9. News scraper overview

The app includes a background **news scraper** that imports articles from RSS/Atom feeds into published posts.

- Configuration tables:
  - `scraper_settings` — singleton row: master **enabled** flag, run interval (minutes), and author email.
  - `scraper_category_feeds` — one row per category: `category_slug`, `feed_urls` (comma-separated), and per-category **enabled** flag.
- Admin UI:
  - `/admin/ai-setting` — provider keys (Gemini, OpenAI, Groq) and the prompts used when paraphrasing scraped content.
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

See `api.http` under the “SCRAPER SETTING” section for example requests to `/api/v1/admin/scraper-setting`.

---

### 10. Project structure (overview)

```text
cmd/
  main.go                 # entrypoint: env, DB, migrate, seed, Gin routes
config/
  config.go               # configuration from environment
internal/
  db/                     # connection, migrations, seeds
  entity/                 # GORM models (user, category, tag, post, site meta/setting)
  middleware/             # JWT
  repositories/
  services/
  routes/
    route.go              # API routes
    frontend_route.go     # HTML + static
    handler/
    dto/
public/
  static/
  templates/              # client/admin layouts, pages, partials
.env.example
api.http
go.mod
```

---

### 11. Production notes

- Set `APP_ENV` to something other than `development` so **seeds do not run**.
- If you need initial admin/sample data in production bootstrap, set `RUN_SEED_ON_STARTUP=true` for first start only, then set it back to `false`.
- Set `GIN_MODE=release` and use a strong, unique `JWT_SECRET`.
- Make sure scraper master + AI provider are configured before enabling news scraping in production.
- Do not rely on seeded credentials; rotate passwords and tighten access as needed.
