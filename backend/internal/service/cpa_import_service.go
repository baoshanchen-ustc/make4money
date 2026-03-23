package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/httpclient"
	"github.com/Wei-Shaw/sub2api/internal/pkg/openai"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/util/urlvalidator"
)

const (
	defaultCPAImportConcurrency = 10
	defaultCPAImportPriority    = 1
)

type CPAImportService struct {
	accountRepo  AccountRepository
	adminService AdminService
	cfg          *config.Config
}

func NewCPAImportService(accountRepo AccountRepository, adminService AdminService, cfg *config.Config) *CPAImportService {
	return &CPAImportService{
		accountRepo:  accountRepo,
		adminService: adminService,
		cfg:          cfg,
	}
}

type PreviewFromCPAInput struct {
	FileName string
	RawJSON  string
}

type ImportFromCPAInput struct {
	FileName            string
	RawJSON             string
	ProxyID             *int64
	Concurrency         int
	UseDefaultGroupBind bool
	GroupIDs            []int64
}

type PreviewRemoteFromCPAInput struct {
	BaseURL       string
	ManagementKey string
}

type ImportRemoteFromCPAInput struct {
	BaseURL             string
	ManagementKey       string
	SelectedSourceKeys  []string
	ProxyID             *int64
	Concurrency         int
	UseDefaultGroupBind bool
	GroupIDs            []int64
}

type CPAPreviewAccount struct {
	SourceKey string   `json:"cpa_source_key"`
	Provider  string   `json:"provider"`
	Name      string   `json:"name"`
	Email     string   `json:"email,omitempty"`
	Platform  string   `json:"platform"`
	Type      string   `json:"type"`
	FileName  string   `json:"file_name"`
	Warnings  []string `json:"warnings,omitempty"`
}

type CPALocalAccountRef struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Platform string `json:"platform"`
	Type     string `json:"type"`
	Status   string `json:"status"`
}

type PreviewFromCPAResult struct {
	Account         CPAPreviewAccount   `json:"account"`
	ExistingAccount *CPALocalAccountRef `json:"existing_account,omitempty"`
}

type PreviewRemoteFromCPAResult struct {
	Items              []PreviewFromCPAResult `json:"items"`
	Total              int                    `json:"total"`
	Importable         int                    `json:"importable"`
	SkippedNonNormal   int                    `json:"skipped_non_normal"`
	SkippedUnsupported int                    `json:"skipped_unsupported"`
}

type SyncFromCPAItemResult struct {
	SourceKey string `json:"cpa_source_key"`
	Provider  string `json:"provider"`
	Name      string `json:"name"`
	Action    string `json:"action"` // created/updated/failed
	Error     string `json:"error,omitempty"`
	AccountID int64  `json:"account_id,omitempty"`
}

type SyncFromCPAResult struct {
	Created int                     `json:"created"`
	Updated int                     `json:"updated"`
	Failed  int                     `json:"failed"`
	Items   []SyncFromCPAItemResult `json:"items"`
}

type cpaImportCandidate struct {
	provider    string
	name        string
	email       string
	platform    string
	accountType string
	fileName    string
	sourceKey   string
	matchKeys   []string
	priority    int
	credentials map[string]any
	extra       map[string]any
	warnings    []string
}

type cpaImportOptions struct {
	ProxyID             *int64
	Concurrency         int
	UseDefaultGroupBind bool
	GroupIDs            []int64
}

type cpaRemoteAuthFile struct {
	Name        string `json:"name"`
	Provider    string `json:"provider"`
	Type        string `json:"type"`
	Status      string `json:"status"`
	Disabled    bool   `json:"disabled"`
	Unavailable bool   `json:"unavailable"`
}

type cpaRemoteListResponse struct {
	Files []cpaRemoteAuthFile `json:"files"`
}

type cpaRemotePreviewStats struct {
	Total              int
	Importable         int
	SkippedNonNormal   int
	SkippedUnsupported int
}

type cpaResolvedRemoteCandidate struct {
	candidate *cpaImportCandidate
	existing  *Account
}

func (s *CPAImportService) PreviewFromCPA(ctx context.Context, input PreviewFromCPAInput) (*PreviewFromCPAResult, error) {
	candidate, err := parseCPAImportCandidate(input.FileName, input.RawJSON)
	if err != nil {
		return nil, err
	}

	return s.buildPreviewResult(ctx, candidate)
}

func (s *CPAImportService) ImportFromCPA(ctx context.Context, input ImportFromCPAInput) (*SyncFromCPAResult, error) {
	candidate, err := parseCPAImportCandidate(input.FileName, input.RawJSON)
	if err != nil {
		return nil, err
	}

	result := &SyncFromCPAResult{
		Items: make([]SyncFromCPAItemResult, 0, 1),
	}

	existing, err := s.findExistingAccountBySourceKeys(ctx, candidate.platform, candidate.matchKeys)
	if err != nil {
		appendCPAItemResult(result, SyncFromCPAItemResult{
			SourceKey: candidate.sourceKey,
			Provider:  candidate.provider,
			Name:      candidate.name,
			Action:    "failed",
			Error:     "lookup existing account failed: " + err.Error(),
		})
		return result, nil
	}

	appendCPAItemResult(result, s.syncCandidate(ctx, candidate, existing, cpaImportOptions{
		ProxyID:             input.ProxyID,
		Concurrency:         input.Concurrency,
		UseDefaultGroupBind: input.UseDefaultGroupBind,
		GroupIDs:            input.GroupIDs,
	}))
	return result, nil
}

func (s *CPAImportService) findExistingAccountBySourceKey(ctx context.Context, platform, sourceKey string) (*Account, error) {
	return s.findExistingAccountBySourceKeys(ctx, platform, []string{sourceKey})
}

func (s *CPAImportService) findExistingAccountBySourceKeys(ctx context.Context, platform string, sourceKeys []string) (*Account, error) {
	if s.accountRepo == nil || strings.TrimSpace(platform) == "" {
		return nil, nil
	}
	sourceKeySet := make(map[string]struct{}, len(sourceKeys))
	for _, sourceKey := range sourceKeys {
		sourceKey = strings.TrimSpace(sourceKey)
		if sourceKey == "" {
			continue
		}
		sourceKeySet[sourceKey] = struct{}{}
	}
	if len(sourceKeySet) == 0 {
		return nil, nil
	}

	page := 1
	for {
		accounts, pageInfo, err := s.accountRepo.ListWithFilters(ctx, pagination.PaginationParams{
			Page:     page,
			PageSize: 100,
		}, platform, "", "", "", 0)
		if err != nil {
			return nil, err
		}
		for i := range accounts {
			account := accounts[i]
			if _, exists := sourceKeySet[strings.TrimSpace(extraString(account.Extra, "cpa_source_key"))]; exists {
				return &account, nil
			}
		}
		if pageInfo == nil || pageInfo.Total <= int64(page*pageInfo.PageSize) || len(accounts) == 0 {
			break
		}
		page++
	}

	return nil, nil
}

func (s *CPAImportService) PreviewRemoteFromCPA(ctx context.Context, input PreviewRemoteFromCPAInput) (*PreviewRemoteFromCPAResult, error) {
	normalizedURL, remoteFiles, err := s.fetchCPARemoteAuthFiles(ctx, input.BaseURL, input.ManagementKey)
	if err != nil {
		return nil, err
	}

	filteredFiles, stats := filterImportableCPARemoteFiles(remoteFiles)
	result := &PreviewRemoteFromCPAResult{
		Items:              make([]PreviewFromCPAResult, 0, len(filteredFiles)),
		Total:              stats.Total,
		Importable:         stats.Importable,
		SkippedNonNormal:   stats.SkippedNonNormal,
		SkippedUnsupported: stats.SkippedUnsupported,
	}

	for _, remoteFile := range filteredFiles {
		rawJSON, downloadErr := s.downloadCPARemoteAuthFile(ctx, normalizedURL, input.ManagementKey, remoteFile.Name)
		if downloadErr != nil {
			return nil, fmt.Errorf("download CPA auth %s failed: %w", remoteFile.Name, downloadErr)
		}
		candidate, parseErr := parseCPAImportCandidate(remoteFile.Name, rawJSON)
		if parseErr != nil {
			return nil, fmt.Errorf("parse CPA auth %s failed: %w", remoteFile.Name, parseErr)
		}
		previewItem, previewErr := s.buildPreviewResult(ctx, candidate)
		if previewErr != nil {
			return nil, previewErr
		}
		result.Items = append(result.Items, *previewItem)
	}

	return result, nil
}

func (s *CPAImportService) ImportRemoteFromCPA(ctx context.Context, input ImportRemoteFromCPAInput) (*SyncFromCPAResult, error) {
	normalizedURL, remoteFiles, err := s.fetchCPARemoteAuthFiles(ctx, input.BaseURL, input.ManagementKey)
	if err != nil {
		return nil, err
	}

	filteredFiles, _ := filterImportableCPARemoteFiles(remoteFiles)
	selectedSet, restrictNewAccounts := buildSelectedStringSet(input.SelectedSourceKeys)
	resolved := make([]cpaResolvedRemoteCandidate, 0, len(filteredFiles))
	result := &SyncFromCPAResult{
		Items: make([]SyncFromCPAItemResult, 0, len(filteredFiles)),
	}

	for _, remoteFile := range filteredFiles {
		rawJSON, downloadErr := s.downloadCPARemoteAuthFile(ctx, normalizedURL, input.ManagementKey, remoteFile.Name)
		if downloadErr != nil {
			appendCPAItemResult(result, SyncFromCPAItemResult{
				SourceKey: remoteFile.Name,
				Provider:  normalizeCPARemoteProvider(remoteFile),
				Name:      remoteFile.Name,
				Action:    "failed",
				Error:     "download failed: " + downloadErr.Error(),
			})
			continue
		}

		candidate, parseErr := parseCPAImportCandidate(remoteFile.Name, rawJSON)
		if parseErr != nil {
			appendCPAItemResult(result, SyncFromCPAItemResult{
				SourceKey: remoteFile.Name,
				Provider:  normalizeCPARemoteProvider(remoteFile),
				Name:      remoteFile.Name,
				Action:    "failed",
				Error:     "parse failed: " + parseErr.Error(),
			})
			continue
		}

		existing, lookupErr := s.findExistingAccountBySourceKeys(ctx, candidate.platform, candidate.matchKeys)
		if lookupErr != nil {
			appendCPAItemResult(result, SyncFromCPAItemResult{
				SourceKey: candidate.sourceKey,
				Provider:  candidate.provider,
				Name:      candidate.name,
				Action:    "failed",
				Error:     "lookup existing account failed: " + lookupErr.Error(),
			})
			continue
		}

		if existing == nil && restrictNewAccounts {
			if _, selected := selectedSet[candidate.sourceKey]; !selected {
				continue
			}
		}

		resolved = append(resolved, cpaResolvedRemoteCandidate{
			candidate: candidate,
			existing:  existing,
		})
	}

	if err := validateCPAManualGroups(resolved, input.UseDefaultGroupBind, input.GroupIDs); err != nil {
		return nil, err
	}

	for _, item := range resolved {
		appendCPAItemResult(result, s.syncCandidate(ctx, item.candidate, item.existing, cpaImportOptions{
			ProxyID:             input.ProxyID,
			Concurrency:         input.Concurrency,
			UseDefaultGroupBind: input.UseDefaultGroupBind,
			GroupIDs:            input.GroupIDs,
		}))
	}

	return result, nil
}

func (s *CPAImportService) buildPreviewResult(ctx context.Context, candidate *cpaImportCandidate) (*PreviewFromCPAResult, error) {
	existing, err := s.findExistingAccountBySourceKeys(ctx, candidate.platform, candidate.matchKeys)
	if err != nil {
		return nil, err
	}

	result := &PreviewFromCPAResult{
		Account: CPAPreviewAccount{
			SourceKey: candidate.sourceKey,
			Provider:  candidate.provider,
			Name:      candidate.name,
			Email:     candidate.email,
			Platform:  candidate.platform,
			Type:      candidate.accountType,
			FileName:  candidate.fileName,
			Warnings:  append([]string(nil), candidate.warnings...),
		},
	}
	if existing != nil {
		result.ExistingAccount = &CPALocalAccountRef{
			ID:       existing.ID,
			Name:     existing.Name,
			Platform: existing.Platform,
			Type:     existing.Type,
			Status:   existing.Status,
		}
	}
	return result, nil
}

func (s *CPAImportService) syncCandidate(ctx context.Context, candidate *cpaImportCandidate, existing *Account, options cpaImportOptions) SyncFromCPAItemResult {
	item := SyncFromCPAItemResult{
		SourceKey: candidate.sourceKey,
		Provider:  candidate.provider,
		Name:      candidate.name,
	}

	if existing != nil {
		updateExtra := mergeExtraMaps(existing.Extra, candidate.extra)
		updateCreds := MergeCredentials(existing.Credentials, candidate.credentials)

		updated, updateErr := s.adminService.UpdateAccount(ctx, existing.ID, &UpdateAccountInput{
			Credentials: updateCreds,
			Extra:       updateExtra,
		})
		if updateErr != nil {
			item.Action = "failed"
			item.Error = updateErr.Error()
			return item
		}

		item.Action = "updated"
		item.AccountID = updated.ID
		return item
	}

	concurrency := options.Concurrency
	if concurrency <= 0 {
		concurrency = defaultCPAImportConcurrency
	}
	priority := candidate.priority
	if priority <= 0 {
		priority = defaultCPAImportPriority
	}
	rateMultiplier := 1.0

	var proxyID *int64
	if options.ProxyID != nil && *options.ProxyID > 0 {
		proxyID = options.ProxyID
	}

	groupIDs := normalizeInt64Slice(options.GroupIDs)
	skipDefaultGroupBind := !options.UseDefaultGroupBind
	if len(groupIDs) > 0 {
		skipDefaultGroupBind = true
	}

	created, createErr := s.adminService.CreateAccount(ctx, &CreateAccountInput{
		Name:                 candidate.name,
		Platform:             candidate.platform,
		Type:                 candidate.accountType,
		Credentials:          candidate.credentials,
		Extra:                candidate.extra,
		ProxyID:              proxyID,
		Concurrency:          concurrency,
		Priority:             priority,
		RateMultiplier:       &rateMultiplier,
		GroupIDs:             groupIDs,
		SkipDefaultGroupBind: skipDefaultGroupBind,
	})
	if createErr != nil {
		item.Action = "failed"
		item.Error = createErr.Error()
		return item
	}

	item.Action = "created"
	item.AccountID = created.ID
	return item
}

func appendCPAItemResult(result *SyncFromCPAResult, item SyncFromCPAItemResult) {
	if result == nil {
		return
	}
	switch item.Action {
	case "created":
		result.Created++
	case "updated":
		result.Updated++
	default:
		result.Failed++
	}
	result.Items = append(result.Items, item)
}

func (s *CPAImportService) fetchCPARemoteAuthFiles(ctx context.Context, baseURL, managementKey string) (string, []cpaRemoteAuthFile, error) {
	if s.cfg == nil {
		return "", nil, errors.New("config is not available")
	}

	normalizedURL := strings.TrimSpace(baseURL)
	if strings.TrimSpace(managementKey) == "" {
		return "", nil, errors.New("management_key is required")
	}

	if s.cfg.Security.URLAllowlist.Enabled {
		normalized, err := normalizeBaseURL(normalizedURL, s.cfg.Security.URLAllowlist.CRSHosts, s.cfg.Security.URLAllowlist.AllowPrivateHosts)
		if err != nil {
			return "", nil, err
		}
		normalizedURL = normalized
	} else {
		normalized, err := urlvalidator.ValidateURLFormat(normalizedURL, s.cfg.Security.URLAllowlist.AllowInsecureHTTP)
		if err != nil {
			return "", nil, fmt.Errorf("invalid base_url: %w", err)
		}
		normalizedURL = normalized
	}

	client, err := httpclient.GetClient(httpclient.Options{
		Timeout:            20 * time.Second,
		ValidateResolvedIP: s.cfg.Security.URLAllowlist.Enabled,
		AllowPrivateHosts:  s.cfg.Security.URLAllowlist.AllowPrivateHosts,
	})
	if err != nil {
		return "", nil, fmt.Errorf("create http client failed: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, normalizedURL+"/v0/management/auth-files", nil)
	if err != nil {
		return "", nil, err
	}
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(managementKey))

	resp, err := client.Do(req)
	if err != nil {
		return "", nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", nil, fmt.Errorf("cpa auth list failed: status=%d body=%s", resp.StatusCode, string(raw))
	}

	var wrapped cpaRemoteListResponse
	if err := json.Unmarshal(raw, &wrapped); err == nil && wrapped.Files != nil {
		return normalizedURL, wrapped.Files, nil
	}

	var files []cpaRemoteAuthFile
	if err := json.Unmarshal(raw, &files); err == nil {
		return normalizedURL, files, nil
	}

	return "", nil, errors.New("parse cpa auth list failed")
}

func (s *CPAImportService) downloadCPARemoteAuthFile(ctx context.Context, baseURL, managementKey, fileName string) (string, error) {
	if s.cfg == nil {
		return "", errors.New("config is not available")
	}

	client, err := httpclient.GetClient(httpclient.Options{
		Timeout:            20 * time.Second,
		ValidateResolvedIP: s.cfg.Security.URLAllowlist.Enabled,
		AllowPrivateHosts:  s.cfg.Security.URLAllowlist.AllowPrivateHosts,
	})
	if err != nil {
		return "", fmt.Errorf("create http client failed: %w", err)
	}

	query := url.Values{}
	query.Set("name", strings.TrimSpace(fileName))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/v0/management/auth-files/download?"+query.Encode(), nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(managementKey))

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("cpa auth download failed: status=%d body=%s", resp.StatusCode, string(raw))
	}

	return string(raw), nil
}

func filterImportableCPARemoteFiles(files []cpaRemoteAuthFile) ([]cpaRemoteAuthFile, cpaRemotePreviewStats) {
	stats := cpaRemotePreviewStats{
		Total: len(files),
	}
	filtered := make([]cpaRemoteAuthFile, 0, len(files))

	for _, file := range files {
		if !isCPARemoteFileNormal(file) {
			stats.SkippedNonNormal++
			continue
		}
		if !isSupportedCPAProvider(normalizeCPARemoteProvider(file)) {
			stats.SkippedUnsupported++
			continue
		}
		filtered = append(filtered, file)
	}

	stats.Importable = len(filtered)
	return filtered, stats
}

func isCPARemoteFileNormal(file cpaRemoteAuthFile) bool {
	return strings.EqualFold(strings.TrimSpace(file.Status), "active") && !file.Disabled && !file.Unavailable
}

func isSupportedCPAProvider(provider string) bool {
	switch strings.TrimSpace(strings.ToLower(provider)) {
	case "claude", "codex", "gemini", "antigravity":
		return true
	default:
		return false
	}
}

func normalizeCPARemoteProvider(file cpaRemoteAuthFile) string {
	return strings.ToLower(strings.TrimSpace(chooseFirstNonEmpty(file.Provider, file.Type)))
}

func buildSelectedStringSet(values []string) (map[string]struct{}, bool) {
	if values == nil {
		return nil, false
	}
	normalized := uniqueNonEmptyStrings(values...)
	out := make(map[string]struct{}, len(normalized))
	for _, value := range normalized {
		out[value] = struct{}{}
	}
	return out, true
}

func validateCPAManualGroups(resolved []cpaResolvedRemoteCandidate, useDefaultGroupBind bool, groupIDs []int64) error {
	groupIDs = normalizeInt64Slice(groupIDs)
	if useDefaultGroupBind || len(groupIDs) == 0 {
		return nil
	}

	platforms := make(map[string]struct{})
	for _, item := range resolved {
		if item.candidate == nil || item.existing != nil {
			continue
		}
		platforms[strings.TrimSpace(item.candidate.platform)] = struct{}{}
		if len(platforms) > 1 {
			return errors.New("manual group selection only supports new CPA accounts from a single platform; use default group binding or narrow the selection")
		}
	}
	return nil
}

func parseCPAImportCandidate(fileName, rawJSON string) (*cpaImportCandidate, error) {
	fileName = strings.TrimSpace(filepath.Base(fileName))
	if strings.TrimSpace(rawJSON) == "" {
		return nil, errors.New("empty auth file")
	}

	payload, err := decodeCPAImportPayload(rawJSON)
	if err != nil {
		return nil, fmt.Errorf("parse auth json failed: %w", err)
	}

	provider := strings.ToLower(strings.TrimSpace(anyToString(payload["type"])))
	if provider == "" {
		return nil, errors.New("missing auth type")
	}

	switch provider {
	case "claude":
		return parseClaudeCPACandidate(fileName, payload)
	case "codex":
		return parseCodexCPACandidate(fileName, payload)
	case "gemini":
		return parseGeminiCPACandidate(fileName, payload)
	case "antigravity":
		return parseAntigravityCPACandidate(fileName, payload)
	default:
		return nil, fmt.Errorf("unsupported CPA auth type: %s", provider)
	}
}

func parseClaudeCPACandidate(fileName string, payload map[string]any) (*cpaImportCandidate, error) {
	accessToken := strings.TrimSpace(anyToString(payload["access_token"]))
	if accessToken == "" {
		return nil, errors.New("missing access_token")
	}
	email := strings.TrimSpace(anyToString(payload["email"]))
	sourceKey, warning := buildCPASourceKey("claude", chooseFirstNonEmpty(email, fallbackCPAIdentity(fileName)))
	warnings := make([]string, 0, 2)
	if warning != "" {
		warnings = append(warnings, warning)
	}
	if strings.TrimSpace(anyToString(payload["refresh_token"])) == "" {
		warnings = append(warnings, "refresh_token missing; account may stop working after access_token expires")
	}

	credentials := map[string]any{
		"access_token":              accessToken,
		"intercept_warmup_requests": false,
	}
	assignIfNotEmpty(credentials, "refresh_token", payload["refresh_token"])
	assignIfNotEmpty(credentials, "id_token", payload["id_token"])
	assignIfNotEmpty(credentials, "email", email)
	assignIfNotEmpty(credentials, "expires_at", payload["expired"])

	extra := buildCPAExtra(fileName, "claude", sourceKey, email)
	assignIfNotEmpty(extra, "org_uuid", payload["org_uuid"])
	assignIfNotEmpty(extra, "account_uuid", payload["account_uuid"])

	return &cpaImportCandidate{
		provider:    "claude",
		name:        buildCPAAccountName("Claude", email, ""),
		email:       email,
		platform:    PlatformAnthropic,
		accountType: AccountTypeOAuth,
		fileName:    fileName,
		sourceKey:   sourceKey,
		matchKeys:   []string{sourceKey},
		priority:    anyToInt(payload["priority"], defaultCPAImportPriority),
		credentials: credentials,
		extra:       extra,
		warnings:    warnings,
	}, nil
}

func parseCodexCPACandidate(fileName string, payload map[string]any) (*cpaImportCandidate, error) {
	accessToken := strings.TrimSpace(anyToString(payload["access_token"]))
	if accessToken == "" {
		return nil, errors.New("missing access_token")
	}
	email := strings.TrimSpace(anyToString(payload["email"]))
	accountID := strings.TrimSpace(anyToString(payload["account_id"]))
	warnings := make([]string, 0, 2)

	idToken := strings.TrimSpace(anyToString(payload["id_token"]))
	if email == "" && idToken != "" {
		if claims, err := openai.DecodeIDToken(idToken); err == nil {
			if userInfo := claims.GetUserInfo(); userInfo != nil && strings.TrimSpace(userInfo.Email) != "" {
				email = strings.TrimSpace(userInfo.Email)
			}
		}
	}
	primaryID := chooseFirstNonEmpty(joinNonEmpty(":", email, accountID), email, accountID, fallbackCPAIdentity(fileName))
	sourceKey, warning := buildCPASourceKey("codex", primaryID)
	if warning != "" {
		warnings = append(warnings, warning)
	}
	matchKeys := uniqueNonEmptyStrings(
		sourceKey,
		mustCPASourceKey("codex", email),
		mustCPASourceKey("codex", accountID),
	)
	if strings.TrimSpace(anyToString(payload["refresh_token"])) == "" {
		warnings = append(warnings, "refresh_token missing; account may stop working after access_token expires")
	}

	credentials := map[string]any{
		"access_token": accessToken,
	}
	assignIfNotEmpty(credentials, "refresh_token", payload["refresh_token"])
	assignIfNotEmpty(credentials, "id_token", idToken)
	assignIfNotEmpty(credentials, "email", email)
	assignIfNotEmpty(credentials, "expires_at", payload["expired"])
	if accountID != "" {
		credentials["chatgpt_account_id"] = accountID
	}

	extra := buildCPAExtra(fileName, "codex", sourceKey, email)
	assignIfNotEmpty(extra, "cpa_account_id", accountID)

	return &cpaImportCandidate{
		provider:    "codex",
		name:        buildCPAAccountName("Codex", email, accountID),
		email:       email,
		platform:    PlatformOpenAI,
		accountType: AccountTypeOAuth,
		fileName:    fileName,
		sourceKey:   sourceKey,
		matchKeys:   matchKeys,
		priority:    anyToInt(payload["priority"], defaultCPAImportPriority),
		credentials: credentials,
		extra:       extra,
		warnings:    warnings,
	}, nil
}

func parseGeminiCPACandidate(fileName string, payload map[string]any) (*cpaImportCandidate, error) {
	tokenMap := anyToMap(payload["token"])
	if len(tokenMap) == 0 {
		return nil, errors.New("missing token")
	}

	accessToken := strings.TrimSpace(anyToString(tokenMap["access_token"]))
	if accessToken == "" {
		return nil, errors.New("missing token.access_token")
	}

	email := strings.TrimSpace(anyToString(payload["email"]))
	projectID := strings.TrimSpace(anyToString(payload["project_id"]))
	sourceKey, warning := buildCPASourceKey("gemini", chooseFirstNonEmpty(joinNonEmpty(":", email, normalizeMultiProjectID(projectID)), joinNonEmpty(":", email, fallbackCPAIdentity(fileName)), fallbackCPAIdentity(fileName)))
	warnings := make([]string, 0, 2)
	if warning != "" {
		warnings = append(warnings, warning)
	}
	if strings.TrimSpace(anyToString(tokenMap["refresh_token"])) == "" {
		warnings = append(warnings, "refresh_token missing; account may stop working after access_token expires")
	}

	credentials := map[string]any{
		"access_token": accessToken,
	}
	assignIfNotEmpty(credentials, "refresh_token", tokenMap["refresh_token"])
	assignIfNotEmpty(credentials, "token_type", tokenMap["token_type"])
	assignIfNotEmpty(credentials, "scope", tokenMap["scope"])
	assignIfNotEmpty(credentials, "expires_at", tokenMap["expiry"])
	assignIfNotEmpty(credentials, "email", email)
	assignIfNotEmpty(credentials, "project_id", projectID)
	if projectID != "" {
		credentials["oauth_type"] = "code_assist"
	}

	extra := buildCPAExtra(fileName, "gemini", sourceKey, email)
	assignIfNotEmpty(extra, "cpa_project_id", projectID)

	return &cpaImportCandidate{
		provider:    "gemini",
		name:        buildCPAAccountName("Gemini", email, projectID),
		email:       email,
		platform:    PlatformGemini,
		accountType: AccountTypeOAuth,
		fileName:    fileName,
		sourceKey:   sourceKey,
		matchKeys:   []string{sourceKey},
		priority:    anyToInt(payload["priority"], defaultCPAImportPriority),
		credentials: credentials,
		extra:       extra,
		warnings:    warnings,
	}, nil
}

func parseAntigravityCPACandidate(fileName string, payload map[string]any) (*cpaImportCandidate, error) {
	accessToken := strings.TrimSpace(anyToString(payload["access_token"]))
	if accessToken == "" {
		return nil, errors.New("missing access_token")
	}
	email := strings.TrimSpace(anyToString(payload["email"]))
	projectID := strings.TrimSpace(anyToString(payload["project_id"]))
	sourceKey, warning := buildCPASourceKey("antigravity", chooseFirstNonEmpty(joinNonEmpty(":", email, projectID), email, fallbackCPAIdentity(fileName)))
	warnings := make([]string, 0, 2)
	if warning != "" {
		warnings = append(warnings, warning)
	}
	if strings.TrimSpace(anyToString(payload["refresh_token"])) == "" {
		warnings = append(warnings, "refresh_token missing; account may stop working after access_token expires")
	}

	expiresAt := strings.TrimSpace(anyToString(payload["expired"]))
	if expiresAt == "" {
		expiresAt = deriveAntigravityExpiresAt(payload)
	}

	credentials := map[string]any{
		"access_token": accessToken,
	}
	assignIfNotEmpty(credentials, "refresh_token", payload["refresh_token"])
	assignIfNotEmpty(credentials, "expires_at", expiresAt)
	assignIfNotEmpty(credentials, "email", email)
	assignIfNotEmpty(credentials, "project_id", projectID)

	extra := buildCPAExtra(fileName, "antigravity", sourceKey, email)
	assignIfNotEmpty(extra, "cpa_project_id", projectID)

	return &cpaImportCandidate{
		provider:    "antigravity",
		name:        buildCPAAccountName("Antigravity", email, projectID),
		email:       email,
		platform:    PlatformAntigravity,
		accountType: AccountTypeOAuth,
		fileName:    fileName,
		sourceKey:   sourceKey,
		matchKeys:   []string{sourceKey},
		priority:    anyToInt(payload["priority"], defaultCPAImportPriority),
		credentials: credentials,
		extra:       extra,
		warnings:    warnings,
	}, nil
}

func buildCPAExtra(fileName, provider, sourceKey, email string) map[string]any {
	extra := map[string]any{
		"cpa_source":      true,
		"cpa_source_key":  sourceKey,
		"cpa_provider":    provider,
		"cpa_file_name":   fileName,
		"cpa_imported_at": time.Now().UTC().Format(time.RFC3339),
	}
	if email != "" {
		extra["cpa_email"] = email
	}
	return extra
}

func buildCPAAccountName(providerLabel, email, suffix string) string {
	base := strings.TrimSpace(email)
	if base == "" {
		base = strings.TrimSpace(suffix)
	}
	if base == "" {
		base = strings.TrimSpace(providerLabel)
	}
	return strings.TrimSpace("CPA " + providerLabel + " " + base)
}

func buildCPASourceKey(provider, id string) (string, string) {
	id = strings.TrimSpace(id)
	if id == "" {
		return provider + ":unknown", "stable identity fields missing; duplicate detection may not work as expected"
	}
	return provider + ":" + strings.ToLower(id), ""
}

func mustCPASourceKey(provider, id string) string {
	key, _ := buildCPASourceKey(provider, id)
	if strings.HasSuffix(key, ":unknown") {
		return ""
	}
	return key
}

func fallbackCPAIdentity(fileName string) string {
	fileName = strings.TrimSpace(fileName)
	if fileName == "" {
		return ""
	}
	return strings.TrimSpace(strings.TrimSuffix(fileName, filepath.Ext(fileName)))
}

func normalizeMultiProjectID(projectID string) string {
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return ""
	}
	parts := strings.Split(projectID, ",")
	normalized := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			normalized = append(normalized, part)
		}
	}
	if len(normalized) == 0 {
		return ""
	}
	return strings.Join(normalized, ",")
}

func deriveAntigravityExpiresAt(payload map[string]any) string {
	expiresIn := anyToInt64(payload["expires_in"], 0)
	if expiresIn <= 0 {
		return ""
	}
	timestamp := anyToInt64(payload["timestamp"], 0)
	if timestamp <= 0 {
		return ""
	}
	expiresAt := time.UnixMilli(timestamp).Add(time.Duration(expiresIn) * time.Second)
	return expiresAt.UTC().Format(time.RFC3339)
}

func decodeCPAImportPayload(rawJSON string) (map[string]any, error) {
	decoder := json.NewDecoder(strings.NewReader(rawJSON))
	decoder.UseNumber()
	var payload map[string]any
	if err := decoder.Decode(&payload); err != nil {
		return nil, err
	}
	if len(payload) == 0 {
		return nil, errors.New("empty auth object")
	}
	return payload, nil
}

func mergeExtraMaps(existing, incoming map[string]any) map[string]any {
	out := make(map[string]any)
	for k, v := range existing {
		out[k] = v
	}
	for k, v := range incoming {
		out[k] = v
	}
	return out
}

func assignIfNotEmpty(target map[string]any, key string, value any) {
	v := strings.TrimSpace(anyToString(value))
	if v == "" {
		return
	}
	target[key] = v
}

func anyToMap(value any) map[string]any {
	out, _ := value.(map[string]any)
	return out
}

func anyToString(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return v
	case json.Number:
		return v.String()
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case float32:
		return strconv.FormatFloat(float64(v), 'f', -1, 32)
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	case int32:
		return strconv.FormatInt(int64(v), 10)
	case uint:
		return strconv.FormatUint(uint64(v), 10)
	case uint64:
		return strconv.FormatUint(v, 10)
	case bool:
		if v {
			return "true"
		}
		return "false"
	default:
		return ""
	}
}

func anyToInt(value any, fallback int) int {
	switch v := value.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	case json.Number:
		if parsed, err := v.Int64(); err == nil {
			return int(parsed)
		}
	case string:
		if parsed, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			return parsed
		}
	}
	return fallback
}

func anyToInt64(value any, fallback int64) int64 {
	switch v := value.(type) {
	case int:
		return int64(v)
	case int64:
		return v
	case float64:
		return int64(v)
	case json.Number:
		if parsed, err := v.Int64(); err == nil {
			return parsed
		}
	case string:
		if parsed, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64); err == nil {
			return parsed
		}
	}
	return fallback
}

func normalizeInt64Slice(values []int64) []int64 {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[int64]struct{}, len(values))
	out := make([]int64, 0, len(values))
	for _, value := range values {
		if value <= 0 {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func extraString(extra map[string]any, key string) string {
	if extra == nil {
		return ""
	}
	return strings.TrimSpace(anyToString(extra[key]))
}

func chooseFirstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func joinNonEmpty(sep string, values ...string) string {
	filtered := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			filtered = append(filtered, value)
		}
	}
	return strings.Join(filtered, sep)
}

func uniqueNonEmptyStrings(values ...string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
