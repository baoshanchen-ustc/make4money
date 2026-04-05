package admin

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"golang.org/x/sync/errgroup"
	"net/http"
	"strings"
)

const rawAPIKeyImportPageSize = 500

type RawAPIKeyImportRequest struct {
	RawText              string `json:"raw_text" binding:"required"`
	ValidateAfterImport  bool   `json:"validate_after_import"`
	SkipDefaultGroupBind bool   `json:"skip_default_group_bind"`
}

type RawAPIKeyImportLineResult struct {
	Line            int    `json:"line"`
	KeyPreview      string `json:"key_preview,omitempty"`
	Platform        string `json:"platform,omitempty"`
	AccountID       int64  `json:"account_id,omitempty"`
	StatusCode      int    `json:"status_code,omitempty"`
	Created         bool   `json:"created"`
	Checked         bool   `json:"checked"`
	Valid           bool   `json:"valid"`
	InvalidDisabled bool   `json:"invalid_disabled"`
	Error           string `json:"error,omitempty"`
	Message         string `json:"message,omitempty"`
}

type RawAPIKeyImportResult struct {
	TotalLines      int                         `json:"total_lines"`
	Created         int                         `json:"created"`
	Checked         int                         `json:"checked"`
	Valid           int                         `json:"valid"`
	InvalidDisabled int                         `json:"invalid_disabled"`
	Failed          int                         `json:"failed"`
	Results         []RawAPIKeyImportLineResult `json:"results"`
}

type APIKeyHealthCheckRequest struct {
	AccountIDs []int64 `json:"account_ids"`
}

type APIKeyHealthCheckItem struct {
	AccountID       int64  `json:"account_id"`
	Name            string `json:"name"`
	Platform        string `json:"platform"`
	StatusCode      int    `json:"status_code,omitempty"`
	Valid           bool   `json:"valid"`
	InvalidDisabled bool   `json:"invalid_disabled"`
	Error           string `json:"error,omitempty"`
	Message         string `json:"message,omitempty"`
}

type APIKeyHealthCheckResult struct {
	Total           int                     `json:"total"`
	Checked         int                     `json:"checked"`
	Valid           int                     `json:"valid"`
	InvalidDisabled int                     `json:"invalid_disabled"`
	Failed          int                     `json:"failed"`
	Results         []APIKeyHealthCheckItem `json:"results"`
}

type rawAPIKeyImportLine struct {
	Line     int
	Key      string
	BaseURL  string
	Platform string
}

func (h *AccountHandler) disableInvalidAPIKeyAccount(ctx context.Context, account *service.Account, message string) error {
	if err := h.adminService.SetAccountError(ctx, account.ID, buildInvalidAPIKeyErrorMessage(account.Platform, message)); err != nil {
		return err
	}
	if account != nil && account.Schedulable {
		if _, err := h.adminService.SetAccountSchedulable(ctx, account.ID, false); err != nil {
			return err
		}
	}
	return nil
}

func (h *AccountHandler) recoverValidAPIKeyAccount(ctx context.Context, account *service.Account) error {
	if account == nil {
		return nil
	}
	// If status is not active (error or disabled), restore it to active.
	if !account.IsActive() {
		if _, err := h.adminService.ClearAccountError(ctx, account.ID); err != nil {
			return err
		}
	}
	// Re-enable scheduling if it was turned off.
	if !account.Schedulable {
		if _, err := h.adminService.SetAccountSchedulable(ctx, account.ID, true); err != nil {
			return err
		}
	}
	return nil
}

func (h *AccountHandler) ImportRawAPIKeys(c *gin.Context) {
	var req RawAPIKeyImportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	if req.ValidateAfterImport && h.accountTestService == nil {
		response.Error(c, http.StatusServiceUnavailable, "API key health check service is unavailable")
		return
	}

	totalLines, lines, parseResults, err := parseRawAPIKeyImportLines(req.RawText)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if totalLines == 0 {
		response.BadRequest(c, "No API key lines found")
		return
	}

	result := RawAPIKeyImportResult{
		TotalLines: totalLines,
		Results:    make([]RawAPIKeyImportLineResult, 0, len(parseResults)),
	}

	result.Results = append(result.Results, parseResults...)
	for _, item := range parseResults {
		if item.Error != "" {
			result.Failed++
		}
	}

	existingByIdentity, err := h.loadExistingAPIKeyIndex(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	for _, line := range lines {
		identity := buildAPIKeyIdentity(line.Platform, line.Key, line.BaseURL)
		if existing, ok := existingByIdentity[identity]; ok && existing != nil {
			result.Failed++
			result.Results = append(result.Results, RawAPIKeyImportLineResult{
				Line:       line.Line,
				KeyPreview: maskRawAPIKey(line.Key),
				Platform:   line.Platform,
				AccountID:  existing.ID,
				Error:      "duplicate key already exists",
			})
			continue
		}

		credentials := map[string]any{
			"api_key": line.Key,
		}
		if line.BaseURL != "" {
			credentials["base_url"] = line.BaseURL
		} else if defaultBaseURL := service.DefaultAPIKeyBaseURL(line.Platform); defaultBaseURL != "" && line.Platform != service.PlatformAnthropic {
			credentials["base_url"] = defaultBaseURL
		}

		account, createErr := h.adminService.CreateAccount(c.Request.Context(), &service.CreateAccountInput{
			Name:                 buildRawAPIKeyAccountName(line.Platform, line.Key),
			Platform:             line.Platform,
			Type:                 service.AccountTypeAPIKey,
			Credentials:          credentials,
			Concurrency:          3,
			Priority:             50,
			SkipDefaultGroupBind: req.SkipDefaultGroupBind,
		})

		item := RawAPIKeyImportLineResult{
			Line:       line.Line,
			KeyPreview: maskRawAPIKey(line.Key),
			Platform:   line.Platform,
		}

		if createErr != nil {
			item.Error = createErr.Error()
			result.Failed++
			result.Results = append(result.Results, item)
			continue
		}

		item.Created = true
		item.AccountID = account.ID
		result.Created++
		existingByIdentity[identity] = account

		if req.ValidateAfterImport {
			item.Checked = true
			result.Checked++
			health, healthErr := h.accountTestService.CheckAPIKeyValidity(c.Request.Context(), account)
			if healthErr != nil {
				item.Error = healthErr.Error()
				result.Failed++
			} else {
				item.StatusCode = health.StatusCode
				item.Message = health.Message
				item.Valid = health.Valid
				if health.Valid {
					result.Valid++
				}
				if health.Invalid {
					item.InvalidDisabled = true
					result.InvalidDisabled++
					if err := h.disableInvalidAPIKeyAccount(c.Request.Context(), account, health.Message); err != nil {
						item.Error = err.Error()
						item.InvalidDisabled = false
						result.InvalidDisabled--
						result.Failed++
					}
				}
			}
		}

		result.Results = append(result.Results, item)
	}

	response.Success(c, result)
}

func (h *AccountHandler) CheckAPIKeysHealth(c *gin.Context) {
	var req APIKeyHealthCheckRequest
	if err := c.ShouldBindJSON(&req); err != nil && err.Error() != "EOF" {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	if h.accountTestService == nil {
		response.Error(c, http.StatusServiceUnavailable, "API key health check service is unavailable")
		return
	}

	accounts, err := h.resolveAPIKeyHealthCheckAccounts(c.Request.Context(), req.AccountIDs)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	result := APIKeyHealthCheckResult{
		Total:   len(accounts),
		Results: make([]APIKeyHealthCheckItem, 0, len(accounts)),
	}

	const maxConcurrency = 8
	g, ctx := errgroup.WithContext(c.Request.Context())
	g.SetLimit(maxConcurrency)

	type checkResult struct {
		item APIKeyHealthCheckItem
		// counters
		valid           bool
		invalidDisabled bool
		failed          bool
	}
	resultCh := make(chan checkResult, len(accounts))

	for _, account := range accounts {
		acc := account
		g.Go(func() error {
			item := APIKeyHealthCheckItem{
				AccountID: acc.ID,
				Name:      acc.Name,
				Platform:  acc.Platform,
			}

			health, healthErr := h.accountTestService.CheckAPIKeyValidity(ctx, acc)

			r := checkResult{item: item}
			if healthErr != nil {
				r.item.Error = healthErr.Error()
				r.failed = true
				resultCh <- r
				return nil
			}

			r.item.StatusCode = health.StatusCode
			r.item.Message = health.Message
			r.item.Valid = health.Valid

			if health.Valid {
				r.valid = true
				if !acc.IsActive() || !acc.Schedulable {
					if err := h.recoverValidAPIKeyAccount(ctx, acc); err != nil {
						r.item.Error = err.Error()
						r.failed = true
					} else if strings.TrimSpace(r.item.Message) == "" {
						r.item.Message = "account re-enabled and scheduling restored after successful health check"
					} else {
						r.item.Message = r.item.Message + " | account re-enabled and scheduling restored after successful health check"
					}
				}
			}

			if health.Invalid {
				if err := h.disableInvalidAPIKeyAccount(ctx, acc, health.Message); err != nil {
					r.item.Error = err.Error()
					r.failed = true
				} else {
					r.item.InvalidDisabled = true
					r.invalidDisabled = true
					if acc.Schedulable {
						if strings.TrimSpace(r.item.Message) == "" {
							r.item.Message = "account marked invalid and scheduling disabled after health check"
						} else {
							r.item.Message += " | scheduling disabled after failed health check"
						}
					}
				}
			}

			resultCh <- r
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	close(resultCh)

	for r := range resultCh {
		result.Checked++
		if r.valid {
			result.Valid++
		}
		if r.invalidDisabled {
			result.InvalidDisabled++
		}
		if r.failed {
			result.Failed++
		}
		result.Results = append(result.Results, r.item)
	}

	response.Success(c, result)
}

func (h *AccountHandler) resolveAPIKeyHealthCheckAccounts(ctx context.Context, accountIDs []int64) ([]*service.Account, error) {
	if len(accountIDs) > 0 {
		accounts, err := h.adminService.GetAccountsByIDs(ctx, accountIDs)
		if err != nil {
			return nil, err
		}
		return filterSupportedAPIKeyAccounts(accounts), nil
	}

	var allAccounts []*service.Account
	page := 1
	for {
		items, total, err := h.adminService.ListAccounts(ctx, page, rawAPIKeyImportPageSize, "", service.AccountTypeAPIKey, "", "", 0, "")
		if err != nil {
			return nil, err
		}
		for i := range items {
			account := items[i]
			accCopy := account
			allAccounts = append(allAccounts, &accCopy)
		}
		if len(allAccounts) >= int(total) || len(items) == 0 {
			break
		}
		page++
	}

	return filterSupportedAPIKeyAccounts(allAccounts), nil
}

func filterSupportedAPIKeyAccounts(accounts []*service.Account) []*service.Account {
	result := make([]*service.Account, 0, len(accounts))
	for _, account := range accounts {
		if account == nil || account.Type != service.AccountTypeAPIKey {
			continue
		}
		switch account.Platform {
		case service.PlatformAnthropic, service.PlatformOpenAI, service.PlatformGemini:
			result = append(result, account)
		}
	}
	return result
}

func (h *AccountHandler) loadExistingAPIKeyIndex(ctx context.Context) (map[string]*service.Account, error) {
	index := make(map[string]*service.Account)
	page := 1
	fetched := 0
	for {
		items, total, err := h.adminService.ListAccounts(ctx, page, rawAPIKeyImportPageSize, "", service.AccountTypeAPIKey, "", "", 0, "")
		if err != nil {
			return nil, err
		}
		fetched += len(items)
		for i := range items {
			account := items[i]
			if account.Type != service.AccountTypeAPIKey {
				continue
			}
			switch account.Platform {
			case service.PlatformAnthropic, service.PlatformOpenAI, service.PlatformGemini:
			default:
				continue
			}
			accCopy := account
			identity := buildAPIKeyIdentity(account.Platform, account.GetCredential("api_key"), account.GetCredential("base_url"))
			// Keep the earliest account (lowest ID) for each identity to preserve original.
			if existing, ok := index[identity]; !ok || account.ID < existing.ID {
				index[identity] = &accCopy
			}
		}
		if fetched >= int(total) || len(items) == 0 {
			break
		}
		page++
	}
	return index, nil
}

func parseRawAPIKeyImportLines(raw string) (int, []rawAPIKeyImportLine, []RawAPIKeyImportLineResult, error) {
	normalized := strings.ReplaceAll(raw, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "，", ",")
	rawLines := strings.Split(normalized, "\n")

	lines := make([]rawAPIKeyImportLine, 0, len(rawLines))
	results := make([]RawAPIKeyImportLineResult, 0, len(rawLines))
	total := 0

	for idx, rawLine := range rawLines {
		lineNo := idx + 1
		line := strings.TrimSpace(rawLine)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "//") {
			continue
		}
		total++

		parts := strings.SplitN(line, ",", 3)
		if len(parts) > 2 {
			results = append(results, RawAPIKeyImportLineResult{
				Line:  lineNo,
				Error: "invalid line format, expected key or key,base_url",
			})
			continue
		}

		key := strings.TrimSpace(parts[0])
		baseURL := ""
		if len(parts) == 2 {
			baseURL = strings.TrimSpace(parts[1])
		}
		if key == "" {
			results = append(results, RawAPIKeyImportLineResult{
				Line:  lineNo,
				Error: "key cannot be empty",
			})
			continue
		}

		platform, ok := service.DetectAPIKeyPlatform(key)
		if !ok {
			results = append(results, RawAPIKeyImportLineResult{
				Line:       lineNo,
				KeyPreview: maskRawAPIKey(key),
				Error:      "unsupported key format, could not detect platform",
			})
			continue
		}

		lines = append(lines, rawAPIKeyImportLine{
			Line:     lineNo,
			Key:      key,
			BaseURL:  baseURL,
			Platform: platform,
		})
	}

	return total, lines, results, nil
}

func buildRawAPIKeyAccountName(platform, key string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(key)))
	return fmt.Sprintf("%s-apikey-%s", platform, hex.EncodeToString(sum[:])[:10])
}

func buildAPIKeyIdentity(platform, key, baseURL string) string {
	normalizedPlatform := strings.TrimSpace(platform)
	normalizedKey := strings.TrimSpace(key)
	normalizedBaseURL := strings.TrimSuffix(strings.TrimSpace(baseURL), "/")
	if normalizedBaseURL == "" {
		normalizedBaseURL = service.DefaultAPIKeyBaseURL(normalizedPlatform)
	}
	return normalizedPlatform + "|" + normalizedKey + "|" + normalizedBaseURL
}

func maskRawAPIKey(key string) string {
	key = strings.TrimSpace(key)
	if len(key) <= 10 {
		return key
	}
	return key[:6] + "..." + key[len(key)-4:]
}

func buildInvalidAPIKeyErrorMessage(platform, message string) string {
	prefix := fmt.Sprintf("API key auto-disabled after health check (%s)", platform)
	if strings.TrimSpace(message) == "" {
		return prefix
	}
	return prefix + ": " + strings.TrimSpace(message)
}
