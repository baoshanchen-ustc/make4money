#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="/opt/sub2api"
BACKEND_DIR="${ROOT_DIR}/backend"

usage() {
  cat <<'EOF'
Usage:
  run_unified_regression.sh <phase>

Phases:
  compile        Run minimal compile-only checks for key packages
  admin-tests    Run admin helper / ops handler / realtime / snapshot tests
  service-tests  Run focused service runtime anomaly tests
  api-smoke      Print suggested curl commands for manual API verification
  all            Run compile + admin-tests + service-tests in order

Notes:
  - This script does not start services for you.
  - This script does not run Redis/DB fault injection or load tests.
  - Use from a controlled window; some phases can still be expensive.
EOF
}

run_compile() {
  cd "${BACKEND_DIR}"
  go test ./internal/handler/admin -run '^$'
  go test ./internal/service -run '^$'
  go test ./internal/repository -run '^$'
}

run_admin_tests() {
  cd "${BACKEND_DIR}"
  go test ./internal/handler/admin -run 'Test(ParseOptionalID|ParsePositiveOptionalID|ParseOpsAPIKeyAndGroupID|ApplyOptionalFilters|OpsSearchHint|AttachOpsSearchLastDetailEndpoint)$'
  go test ./internal/handler/admin -run 'TestOpsBillingCompensationHandler_'
  go test ./internal/handler/admin -run 'TestOpsUsageLogNotPersistedHandler_'
  go test ./internal/handler/admin -run 'TestDashboardHandler_GetRealtimeMetrics_UsesRuntimeSnapshots$'
  go test ./internal/handler/admin -run 'Test(BillingCompensationPayload|UsageLogNotPersistedPayload|EnrichBillingCompensationFallback|EnrichUsageLogFallback)_'
}

run_service_tests() {
  cd "${BACKEND_DIR}"
  go test ./internal/service -run 'Test.*OpsDashboard.*|Test.*OpsMetricsCollector.*|Test.*WriteUsageLogBestEffort.*|Test.*BillingCompensation.*'
}

print_api_smoke() {
  cat <<'EOF'
Suggested manual API checks:

curl -sS 'http://127.0.0.1:8080/api/v1/admin/ops/billing-compensation'
curl -sS 'http://127.0.0.1:8080/api/v1/admin/ops/billing-compensation/<request_id>'
curl -sS 'http://127.0.0.1:8080/api/v1/admin/ops/usage-log-not-persisted'
curl -sS 'http://127.0.0.1:8080/api/v1/admin/ops/usage-log-not-persisted/<request_id>'
curl -sS 'http://127.0.0.1:8080/api/v1/admin/dashboard/realtime'
curl -sS 'http://127.0.0.1:8080/api/v1/admin/ops/dashboard/snapshot-v2'
EOF
}

main() {
  if [[ $# -ne 1 ]]; then
    usage
    exit 1
  fi

  case "$1" in
    compile)
      run_compile
      ;;
    admin-tests)
      run_admin_tests
      ;;
    service-tests)
      run_service_tests
      ;;
    api-smoke)
      print_api_smoke
      ;;
    all)
      run_compile
      run_admin_tests
      run_service_tests
      ;;
    -h|--help|help)
      usage
      ;;
    *)
      echo "Unknown phase: $1" >&2
      usage
      exit 1
      ;;
  esac
}

main "$@"
