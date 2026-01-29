---
status: resolved
priority: p3
issue_id: 005
tags: [security, logging]
dependencies: []
---

# 005 - Sensitive Credentials in Logs

## Problem Statement
The `AUTO_SETUP` feature logs the generated admin password and JWT secret to stdout.

## Findings
Logging secrets is a security anti-pattern. If logs are aggregated, these secrets persist in log management systems.

## Proposed Solutions

### Option 1: One-time File Creation
- Write the initial credentials to `/app/data/.initial_admin_password` instead of logging them.
- **Pros:** Secrets don't enter log streams.
- **Cons:** Users must `cat` a file inside the container.
- **Effort:** Small
- **Risk:** Low

## Acceptance Criteria
- [ ] Credentials are no longer visible in `docker logs`.
