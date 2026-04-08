# Copilot Accounts Daily Stats – Code Review (2026-04-08)

## Scope
- backend/internal/service/copilot_analytics_service.go
- frontend/src/api/admin/copilotAnalytics.ts
- frontend/src/components/admin/copilot/AccountsDailyChart.vue
- frontend/src/views/admin/copilot/CopilotAccountsView.vue

## Reviewer Summary
- API now exposes premium/agent splits per account-day, and the admin UI surfaces a metric toggle to visualize those series.
- The implementation is generally sound, but there are a few contract, testing, and UX gaps worth addressing before merge.
- I was not able to run automated tests in this review; recommendations below highlight the missing coverage.

## Findings

### 1. Breaking change to `/accounts/daily-stats` response schema (severity: HIGH)
- `CopilotAccountDailyEntry` removed the legacy `count` field and now only emits `premium_count` / `agent_count` (backend/internal/service/copilot_analytics_service.go:728-798).
- The handler at `/api/v1/admin/copilot/accounts/daily-stats` has no versioning or compatibility shim, so any consumer that still expects `days[].count` will now receive a 200 response that lacks the field entirely and likely crash when parsing.
- Only the admin chart was updated in this patch; please confirm no other dashboards, scripts, or exports rely on this endpoint, or add a derived `count` json tag to smooth the rollout (for example, keep a deprecated `Count int` field with ``json:"count"`` populated as `premium+agent`).

### 2. No regression tests for the new premium/agent split (severity: MEDIUM)
- There are no unit or integration tests covering `CopilotAnalyticsService.GetAccountsDailyStats`, so it is easy to regress the SQL filters when adding more initiator types or changing timezone handling (backend/internal/service/copilot_analytics_service.go:750-809).
- Likewise, no front-end tests assert that `AccountsDailyChart` switches datasets correctly when `metric` changes.
- Please add at least one backend test that seeds user/agent logs to verify the counts returned, and a lightweight component test that ensures the chart builds premium vs. agent datasets from the API payload.

### 3. Metric toggle labels bypass localization (severity: LOW)
- `METRIC_OPTIONS` hard-codes `'Premium'` and `'Agent'` strings alongside Chinese copy (frontend/src/views/admin/copilot/CopilotAccountsView.vue:521-525).
- The rest of the view already uses `t('...')`; leaving English literals here means the toggle is not translated when the admin UI switches locales.
- Pipe these labels through the existing i18n catalog (e.g., new keys under `admin.copilot.accounts.metrics.*`) to keep the UI consistent.

## Recommendation
**Status:** Request changes. Please address the above issues (especially maintaining backward compatibility) and add regression coverage before shipping.
