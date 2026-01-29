---
status: resolved
priority: p3
issue_id: 004
tags: [architecture, portability]
dependencies: []
---

# 004 - Fragile Network Naming in Tunnels

## Problem Statement
Tunnel overrides use hardcoded network names like `deploy_sub2api-network`.

## Findings
The `deploy_` prefix is derived from the directory name. If a user clones the repo into a different directory or uses `-p` flag, the tunnels fail to connect to the app network.

## Proposed Solutions

### Option 1: Merge-based Network Referencing (Recommended)
- Remove the top-level `networks` block and the `external: true` definition. 
- Use native Compose merging logic where the override service implicitly joins the base network.
- **Pros:** Portable across any project name/directory.
- **Cons:** None.
- **Effort:** Small
- **Risk:** Low

## Acceptance Criteria
- [x] Tunnels work regardless of the parent directory name.
