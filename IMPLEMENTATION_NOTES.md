# Account-Level User-Agent Implementation Notes

## Completed Changes

### Backend (Go)
- ✅ Added `user_agent` field to Account schema (`backend/ent/schema/account.go`)
  - Type: optional string, nullable, max 200 chars
  - Comment: "Custom User-Agent for upstream requests"

### Frontend (TypeScript)
- ✅ Added `user_agent` field to Account interface (`frontend/src/types/index.ts`)

## TODO: Backend Integration

The following files need to be updated to read `account.UserAgent` and use it in HTTP requests:

### 1. Claude OAuth Service
**File:** `backend/internal/repository/claude_oauth_service.go`
**Lines:** 215, 253
**Current:** `SetHeader("User-Agent", "axios/1.8.4")`
**Change to:**
```go
ua := "axios/1.8.4"
if account != nil && account.UserAgent != nil && *account.UserAgent != "" {
    ua = *account.UserAgent
}
SetHeader("User-Agent", ua)
```
**Note:** Need to pass `account *ent.Account` parameter to `ExchangeCodeForToken` and `RefreshToken` methods.

### 2. OpenAI OAuth Service
**File:** `backend/internal/repository/openai_oauth_service.go`
**Lines:** (find SetHeader calls)
**Current:** `SetHeader("User-Agent", "codex-cli/0.91.0")`
**Change:** Same pattern as Claude, default to "codex-cli/0.91.0"

### 3. Gemini CLI Code Assist Client
**File:** `backend/internal/repository/geminicli_codeassist_client.go`
**Lines:** 37, 81
**Current:** `SetHeader("User-Agent", geminicli.GeminiCLIUserAgent)`
**Change:** Same pattern, default to `geminicli.GeminiCLIUserAgent`

### 4. Request Client Pool (Optional)
**File:** `backend/internal/repository/req_client_pool.go`
**Current:** `reqClientOptions` has `ProxyURL`, `Timeout`, `Impersonate`, `ForceHTTP2`
**Consideration:** Adding `UserAgent` to cache key would prevent client reuse across accounts with different UAs. This might be acceptable for correctness, but impacts performance.

**Recommendation:** Don't add UA to `reqClientOptions`. Instead, set it per-request using `R().SetHeader()` in each service method.

## TODO: Frontend UI

Need to add User-Agent input field in account edit form:

**Likely file:** `frontend/src/views/admin/AccountsView.vue` or `frontend/src/components/admin/AccountForm.vue`

**UI mockup:**
```vue
<FormField 
  label="User-Agent (可选)" 
  help="自定义请求上游 API 时的 User-Agent，留空使用默认值"
>
  <input 
    v-model="form.user_agent" 
    placeholder="留空使用默认值" 
    maxlength="200"
  />
</FormField>
```

**Default values by platform:**
- Claude: `axios/1.8.4`
- OpenAI: `codex-cli/0.91.0`
- Gemini: (see `geminicli.GeminiCLIUserAgent` constant)

## Migration

**DO NOT** create migration files in this PR. Let the maintainer handle database migrations.

The maintainer will need to add:
```sql
ALTER TABLE accounts ADD COLUMN user_agent VARCHAR(200) NULL;
```

## Testing

After backend integration:
1. Create/edit an account with custom User-Agent
2. Verify the UA is sent in upstream requests (check logs or use a proxy like mitmproxy)
3. Verify empty/null UA falls back to default values

## Related Issue

Closes #753
