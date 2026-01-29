---
status: resolved
priority: p3
issue_id: 006
tags: [architecture, observability]
dependencies: []
---

# 006 - Missing Sidecar Healthchecks

## Problem Statement
The tunnel sidecars lack healthchecks, making it hard to monitor remote access status.

## Findings
The app can be healthy but the tunnel can be disconnected. Currently, only the app's health is monitored.

## Proposed Solutions

### Option 1: Add Sidecar Healthchecks
- Add healthchecks to `cloudflared` (pinging its local management port) and `tailscale`.
- **Pros:** Better observability.
- **Effort:** Small
- **Risk:** Low

## Acceptance Criteria
- [ ] `docker ps` shows health status for all sidecars.
