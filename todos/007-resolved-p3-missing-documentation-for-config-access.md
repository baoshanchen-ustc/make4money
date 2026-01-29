---
status: resolved
priority: p3
issue_id: 007
tags: [docs, ux]
dependencies: []
---

# 007 - Missing Documentation for Config Access

## Problem Statement
Since `config.yaml` is now hidden in a volume by default, users may not know how to view/edit it.

## Findings
The transition to `AUTO_SETUP` makes the configuration less visible to the host.

## Proposed Solutions

### Option 1: Add "Maintenance" section to DOCKER.md
- Document how to `exec` into the container to see the config.
- **Effort:** Small

## Acceptance Criteria
- [x] Instructions included in `deploy/DOCKER.md`.
