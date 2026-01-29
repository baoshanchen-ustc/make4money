---
status: resolved
priority: p2
issue_id: 002
tags: [security, architecture, network]
dependencies: [001]
---

# 002 - Insufficient Network Isolation for Tunnels

## Problem Statement
The Cloudflare and Tailscale sidecars are attached to the same network as the database and cache, creating a risk of lateral movement.

## Findings
An attacker compromising the tunnel container or gaining access via a misconfigured tunnel policy can directly probe internal ports (5432, 6379) on the shared `sub2api-network`.

## Proposed Solutions

### Option 1: Dual-Network Segmentation (Recommended)
- Create a `frontend-network` for Tunnels <-> App.
- Create a `backend-network` for App <-> Postgres/Redis.
- **Pros:** Strongest isolation; sidecars cannot see the database.
- **Cons:** Slightly more complex YAML.
- **Effort:** Medium
- **Risk:** Low

### Option 2: Container-level Firewalling
- Use `iptables` or Docker network policies.
- **Pros:** No change to network structure.
- **Cons:** Hard to manage and verify in Compose.
- **Effort:** Large
- **Risk:** High

## Technical Details
- **Affected Files:** `deploy/docker-compose.yml`, `deploy/docker-compose.tunnel-*.yml`

## Acceptance Criteria
- [ ] Tunnels only share a network with the `sub2api` service.
- [ ] Database/Redis are isolated on a network not visible to sidecars.
