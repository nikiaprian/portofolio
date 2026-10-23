---
description: Architecture and code navigation rules
alwaysApply: true
---

# Architecture

This repository follows strict layering.

Layer order:

Handler -> Service -> Repository -> Entity

Rules:

- Handlers must not contain business logic.
- Handlers must not query database directly.
- Services contain business logic.
- Repositories contain database access.
- Entities contain models only.

# Context Rules

Before reading files:

1. Determine feature domain.
2. Read only related files.
3. Never scan entire repository.

Maximum files to read:

- Small feature: 3 files
- Medium feature: 5 files
- Complex feature: 10 files

Never exceed 10 files unless explicitly requested.

# Forbidden Paths

Never read these folders unless user explicitly asks:

public/vendor/**
public/uploads/**
data/cache/**
db/seeds/**
db/migrations/**

# File Selection Strategy

For post-related features:

internal/routes/handler/post_handler.go
internal/services/post_service.go
internal/repositories/post_repository.go
internal/entity/post.go

For category-related features:

internal/routes/handler/category_handler.go
internal/services/category_service.go
internal/repositories/category_repository.go
internal/entity/category.go

Always prefer targeted file reading.

# Repository Reading Policy

Never scan the entire repository.

Never recursively read all files.

Before reading code:

1. Determine feature domain.
2. Identify relevant layer.
3. Read only required files.

Maximum files:

- Small change: 3 files
- Medium change: 5 files
- Large change: 10 files

If more than 10 files are needed:

Explain why before continuing.

# Forbidden Directories

Do not read these folders unless explicitly requested:

public/vendor/**
public/uploads/**
data/cache/**
db/seeds/**
db/migrations/**

These folders are not useful for understanding business logic.

# Feature Mapping

Post Feature

- internal/routes/handler/post_handler.go
- internal/services/post_service.go
- internal/repositories/post_repository.go
- internal/entity/post.go

Category Feature

- internal/routes/handler/category_handler.go
- internal/services/category_service.go
- internal/repositories/category_repository.go
- internal/entity/category.go

Tag Feature

- internal/routes/handler/tag_handler.go
- internal/services/tag_service.go
- internal/repositories/tag_repository.go
- internal/entity/tag.go

Site Setting Feature

- internal/routes/handler/site_setting_handler.go
- internal/services/site_setting_service.go
- internal/repositories/site_setting_repository.go
- internal/entity/site_setting.go

# Performance Rules

Before creating database queries:

1. Check repository implementation.
2. Check cache implementation.
3. Reuse existing cache patterns.

Avoid:

- N+1 queries
- SELECT *
- Loading unnecessary records

Prefer:

- Pagination
- Projection
- Cache reuse

Never remove caching without explicit request.

# Reuse Existing Code

Before creating:

- Service
- Repository
- Helper
- Middleware

Search for existing implementation.

Prefer extending existing code.

Avoid duplicate business logic.

Avoid duplicate helper functions.

# Agent Workflow

For every request:

1. Identify feature area.
2. Locate handler.
3. Locate service.
4. Locate repository.
5. Read only required files.
6. Propose implementation.

Never start by reading the entire repository.

Never use repository-wide searches unless explicitly requested.

Working code alone is insufficient.

Also verify:

- Architecture consistency
- Performance impact
- Cache impact
- Query efficiency
- Reuse of existing code