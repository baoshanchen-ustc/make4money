---
status: resolved
priority: p1
issue_id: 001
tags: [security, network, deployment]
dependencies: []
---

# 001 - Dangerous Default for SSRF Protection

## Problem Statement
The `SECURITY_URL_ALLOWLIST_ALLOW_PRIVATE_HOSTS` variable was set to `true` by default in `deploy/.env.example`. 

## Findings
In cloud environments, this permissive default allowed the application to reach internal metadata services (e.g., AWS/GCP/Azure IMDS at `169.254.169.254`), which could be used to steal temporary IAM credentials if an SSRF vulnerability was exploited.

## Acceptance Criteria
- [x] `SECURITY_URL_ALLOWLIST_ALLOW_PRIVATE_HOSTS` is set to `false` in all template files.
- [x] Documentation explains when and why a user might need to enable this.
