---
status: resolved
priority: p2
issue_id: 003
tags: [architecture, simplicity, maintenance]
dependencies: []
---

# 003 - Docker Compose Duplication & Drift

## Problem Statement
`docker-compose-test.yml` is 90% identical to `docker-compose.yml`, leading to configuration drift.

## Findings
Minor Divergences already exist in Redis command formatting and port mappings. Maintaining two monolithic files is error-prone.

## Proposed Solutions

### Option 1: Build-only Override (Recommended)
- Replace `docker-compose-test.yml` with a minimal `docker-compose.build.yml` containing only the `build` and `image` keys.
- **Pros:** 100% parity with production base; tiny maintenance surface.
- **Cons:** Requires using two `-f` flags for local builds.
- **Effort:** Small
- **Risk:** Low

## Technical Details
- **Affected Files:** `deploy/docker-compose.build.yml` (created), `deploy/docker-compose-test.yml` (removed)

## Acceptance Criteria
- [x] `docker-compose-test.yml` is replaced or significantly reduced.
- [x] Build logic works via `docker compose -f ... -f ...`.
