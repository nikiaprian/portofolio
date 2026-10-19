# Project Architecture

Backend:

- Go
- GORM

Flow:

Handler
↓
Service
↓
Repository
↓
Database

Folders:

internal/routes/handler
internal/services
internal/repositories
internal/entity

Caching:

internal/cache
internal/routes/client_template_cache.go
internal/repositories/site_setting_cache.go

Media:

internal/media

Storage:

internal/storage

Scraping:

internal/scraper

AI:

internal/ai

Rules:

- Database only in repository layer.
- Business logic only in service layer.
- Handlers orchestrate only.