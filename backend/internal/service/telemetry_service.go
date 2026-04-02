package service

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
)

var errInvalidTelemetryPayload = errors.New("invalid telemetry payload")

type PersonaProfile struct {
	Platform              string `json:"platform"`
	PlatformRaw           string `json:"platform_raw"`
	Arch                  string `json:"arch"`
	NodeVersion           string `json:"node_version"`
	Terminal              string `json:"terminal"`
	PackageManagers       string `json:"package_managers"`
	Runtimes              string `json:"runtimes"`
	IsRunningWithBun      bool   `json:"is_running_with_bun"`
	DeploymentEnvironment string `json:"deployment_environment"`
	Version               string `json:"version"`
	VersionBase           string `json:"version_base"`
	BuildTime             string `json:"build_time"`
}

var personaTerminalPool = []string{
	"iTerm.app",
	"Terminal.app",
	"vscode",
	"tmux",
	"WezTerm",
	"WarpTerminal",
	"Alacritty",
	"kitty",
}

var personaNodeVersionPool = []string{
	"v22.13.0",
	"v22.13.1",
	"v22.13.2",
	"v22.13.3",
	"v22.14.0",
	"v22.14.1",
	"v22.15.0",
}

var personaPackageManagersPool = []string{
	"npm,pnpm",
	"npm",
	"pnpm",
	"npm,yarn",
	"pnpm,yarn",
}

var personaRuntimesPool = []string{
	"bun,node",
	"node",
	"node,bun",
	"node,deno",
	"bun,node,deno",
}

var personaDeploymentEnvPool = []string{
	"unknown-darwin",
	"desktop-darwin",
	"local-darwin",
	"developer-macos",
}

var syntheticAgentNames = []string{
	"planner", "researcher", "executor", "critic", "architect", "analyst", "reviewer", "writer",
}

var syntheticTeamNames = []string{
	"alpha", "beta", "delta", "ops", "prod", "assist", "swarm", "studio",
}

var defaultPersona = PersonaProfile{
	Platform:              "darwin",
	PlatformRaw:           "darwin",
	Arch:                  "arm64",
	NodeVersion:           "v22.13.1",
	Terminal:              "iTerm.app",
	PackageManagers:       "npm,pnpm",
	Runtimes:              "bun,node",
	IsRunningWithBun:      true,
	DeploymentEnvironment: "unknown-darwin",
	Version:               "2.2.19",
	VersionBase:           "2.2.19",
	BuildTime:             "2026-03-28T10:30:00Z",
}

var defaultTelemetryVersionPool = []string{"2.2.17", "2.2.18", "2.2.19", "2.3.0"}
var forwardSem = make(chan struct{}, 64)
var forwardClient *http.Client

func init() {
	profile := &tlsfingerprint.Profile{
		ALPNProtocols: []string{"http/1.1"},
	}
	dialer := tlsfingerprint.NewDialer(profile, nil)
	tr := &http.Transport{
		DialTLSContext:    dialer.DialTLSContext,
		ForceAttemptHTTP2: false,
		MaxIdleConns:      100,
		IdleConnTimeout:   90 * time.Second,
	}
	forwardClient = &http.Client{
		Transport: tr,
		Timeout:   10 * time.Second,
	}
}

var telemetrySalt = func() string {
	if salt := os.Getenv("TELEMETRY_SALT"); salt != "" {
		return salt
	}
	return "_sub2api_telemetry_salt_v1"
}()

type TelemetryService struct{}

func NewTelemetryService() *TelemetryService {
	return &TelemetryService{}
}

func (s *TelemetryService) GenerateShadowDeviceID(accountOrOrgUUID string, originalDeviceID string) string {
	seed := strings.TrimSpace(accountOrOrgUUID)
	if seed == "" {
		seed = strings.TrimSpace(originalDeviceID)
	}
	if seed == "" {
		seed = "anonymous"
	}
	return stableUUID(seed + telemetrySalt)
}

func (s *TelemetryService) GenerateMappedUUID(shadowDeviceID, originalID string) string {
	seed := shadowDeviceID + "|" + originalID + "|" + telemetrySalt
	return stableUUID(seed)
}

func (s *TelemetryService) GenerateOpaqueID(prefix, shadowDeviceID, originalID string) string {
	hash := sha256.Sum256([]byte(prefix + "|" + shadowDeviceID + "|" + originalID + "|" + telemetrySalt))
	return fmt.Sprintf("%s-%x", prefix, hash[:6])
}

func (s *TelemetryService) GenerateOpaqueAgentID(shadowDeviceID, originalID string) string {
	hash := sha256.Sum256([]byte("agent|" + shadowDeviceID + "|" + originalID + "|" + telemetrySalt))
	if strings.Contains(originalID, "@") {
		agentName := syntheticAgentNames[int(hash[0])%len(syntheticAgentNames)]
		teamName := syntheticTeamNames[int(hash[1])%len(syntheticTeamNames)]
		return fmt.Sprintf("%s@%s", agentName, teamName)
	}
	return stableUUID("agent|" + shadowDeviceID + "|" + originalID + "|" + telemetrySalt)
}

func (s *TelemetryService) GenerateDynamicPersona(shadowDeviceID string) PersonaProfile {
	persona := defaultPersona
	hash := sha256.Sum256([]byte(shadowDeviceID + "persona"))
	persona.Terminal = personaTerminalPool[int(hash[0])%len(personaTerminalPool)]
	persona.NodeVersion = personaNodeVersionPool[int(hash[1])%len(personaNodeVersionPool)]
	persona.PackageManagers = personaPackageManagersPool[int(hash[2])%len(personaPackageManagersPool)]
	persona.Runtimes = personaRuntimesPool[int(hash[3])%len(personaRuntimesPool)]
	persona.DeploymentEnvironment = personaDeploymentEnvPool[int(hash[4])%len(personaDeploymentEnvPool)]
	if int(hash[5])%10 < 3 {
		persona.Arch = "x64"
	}
	if strings.Contains(persona.Runtimes, "bun") {
		persona.IsRunningWithBun = true
	} else {
		persona.IsRunningWithBun = false
	}
	persona.Version = selectTelemetryVersion(shadowDeviceID)
	persona.VersionBase = versionBase(persona.Version)
	persona.BuildTime = syntheticBuildTime(persona.Version)
	return persona
}

func (s *TelemetryService) DeepScrubPayload(bodyBytes []byte) ([]byte, error) {
	if !gjson.ValidBytes(bodyBytes) {
		return nil, fmt.Errorf("%w: malformed json", errInvalidTelemetryPayload)
	}

	eventsRes := gjson.GetBytes(bodyBytes, "events")
	if !eventsRes.Exists() || !eventsRes.IsArray() {
		return nil, fmt.Errorf("%w: missing events array", errInvalidTelemetryPayload)
	}

	resultBytes := bodyBytes
	now := time.Now().UTC()

	for i, ev := range eventsRes.Array() {
		basePath := fmt.Sprintf("events.%d", i)
		eventData := ev.Get("event_data")
		if !eventData.Exists() || !looksLikeJSONObject(eventData.Raw) {
			return nil, fmt.Errorf("%w: event %d missing object event_data", errInvalidTelemetryPayload, i)
		}

		accountSeed := extractAccountSeed(ev)
		origDevID := firstNonEmpty(
			ev.Get("event_data.device_id").String(),
			ev.Get("device_id").String(),
		)
		shadowDeviceID := s.GenerateShadowDeviceID(accountSeed, origDevID)
		persona := s.GenerateDynamicPersona(shadowDeviceID)
		syntheticTime := syntheticEventTime(now, shadowDeviceID, i)

		resultBytes = deletePaths(resultBytes,
			basePath+".event_data.auth",
			basePath+".event_data.accountUUID",
			basePath+".event_data.account_uuid",
			basePath+".event_data.organizationUUID",
			basePath+".event_data.organization_uuid",
			basePath+".event_data.server_timestamp",
		)

		if ev.Get("device_id").Exists() {
			resultBytes, _ = sjson.SetBytes(resultBytes, basePath+".device_id", shadowDeviceID)
		}
		if ev.Get("event_data.device_id").Exists() {
			resultBytes, _ = sjson.SetBytes(resultBytes, basePath+".event_data.device_id", shadowDeviceID)
		}
		if ev.Get("event_data.event_id").Exists() {
			resultBytes, _ = sjson.SetBytes(resultBytes, basePath+".event_data.event_id", uuid.NewString())
		}
		if ev.Get("event_id").Exists() {
			resultBytes, _ = sjson.SetBytes(resultBytes, basePath+".event_id", uuid.NewString())
		}
		if origSessionID := ev.Get("event_data.session_id").String(); origSessionID != "" {
			resultBytes, _ = sjson.SetBytes(resultBytes, basePath+".event_data.session_id", s.GenerateMappedUUID(shadowDeviceID, origSessionID))
		}
		if origParentSessionID := ev.Get("event_data.parent_session_id").String(); origParentSessionID != "" {
			resultBytes, _ = sjson.SetBytes(resultBytes, basePath+".event_data.parent_session_id", s.GenerateMappedUUID(shadowDeviceID, origParentSessionID))
		}
		if origAnonID := ev.Get("event_data.anonymous_id").String(); origAnonID != "" {
			resultBytes, _ = sjson.SetBytes(resultBytes, basePath+".event_data.anonymous_id", s.GenerateMappedUUID(shadowDeviceID, origAnonID))
		}
		if origAgentID := ev.Get("event_data.agent_id").String(); origAgentID != "" {
			resultBytes, _ = sjson.SetBytes(resultBytes, basePath+".event_data.agent_id", s.GenerateOpaqueAgentID(shadowDeviceID, origAgentID))
		}
		if ev.Get("event_data.timestamp").Exists() {
			resultBytes, _ = sjson.SetBytes(resultBytes, basePath+".event_data.timestamp", syntheticTime.Format(time.RFC3339Nano))
		}
		if ev.Get("event_data.client_timestamp").Exists() {
			resultBytes, _ = sjson.SetBytes(resultBytes, basePath+".event_data.client_timestamp", syntheticTime.Format(time.RFC3339Nano))
		}
		if ev.Get("event_data.event_metadata_vars").Exists() {
			resultBytes, _ = sjson.DeleteBytes(resultBytes, basePath+".event_data.event_metadata_vars")
		}

		sanitizedUserAttrs, hasUserAttrs := sanitizeUserAttributes(ev.Get("event_data.user_attributes").String(), shadowDeviceID, persona)
		if hasUserAttrs {
			resultBytes, _ = sjson.SetBytes(resultBytes, basePath+".event_data.user_attributes", sanitizedUserAttrs)
		}

		if ev.Get("event_type").String() == "ClaudeCodeInternalEvent" {
			resultBytes = deletePaths(resultBytes, basePath+".event_data.email")
			if ev.Get("event_data.process").Exists() {
				resultBytes, _ = sjson.SetBytes(resultBytes, basePath+".event_data.process", syntheticProcessMetrics(shadowDeviceID, syntheticTime))
			}
			resultBytes = overwriteEnvBlockSJSON(resultBytes, basePath+".event_data.env", persona)
			if sanitizedMeta, ok := sanitizeAdditionalMetadata(ev.Get("event_data.additional_metadata").String(), persona); ok {
				resultBytes, _ = sjson.SetBytes(resultBytes, basePath+".event_data.additional_metadata", sanitizedMeta)
			} else if ev.Get("event_data.additional_metadata").Exists() {
				resultBytes, _ = sjson.DeleteBytes(resultBytes, basePath+".event_data.additional_metadata")
			}
		}
	}

	return resultBytes, nil
}

func overwriteEnvBlockSJSON(payload []byte, prefix string, persona PersonaProfile) []byte {
	payload, _ = sjson.SetBytes(payload, prefix+".platform", persona.Platform)
	payload, _ = sjson.SetBytes(payload, prefix+".platform_raw", persona.PlatformRaw)
	payload, _ = sjson.SetBytes(payload, prefix+".arch", persona.Arch)
	payload, _ = sjson.SetBytes(payload, prefix+".node_version", persona.NodeVersion)
	payload, _ = sjson.SetBytes(payload, prefix+".terminal", persona.Terminal)
	payload, _ = sjson.SetBytes(payload, prefix+".package_managers", persona.PackageManagers)
	payload, _ = sjson.SetBytes(payload, prefix+".runtimes", persona.Runtimes)
	payload, _ = sjson.SetBytes(payload, prefix+".is_running_with_bun", persona.IsRunningWithBun)
	payload, _ = sjson.SetBytes(payload, prefix+".deployment_environment", persona.DeploymentEnvironment)
	payload, _ = sjson.SetBytes(payload, prefix+".version", persona.Version)
	payload, _ = sjson.SetBytes(payload, prefix+".version_base", persona.VersionBase)
	payload, _ = sjson.SetBytes(payload, prefix+".build_time", persona.BuildTime)

	payload = deletePaths(payload,
		prefix+".wsl_version",
		prefix+".linux_distro_id",
		prefix+".linux_distro_version",
		prefix+".linux_kernel",
		prefix+".github_actions_metadata",
		prefix+".github_event_name",
		prefix+".github_actions_runner_environment",
		prefix+".github_actions_runner_os",
		prefix+".github_action_ref",
		prefix+".remote_environment_type",
		prefix+".claude_code_container_id",
		prefix+".claude_code_remote_session_id",
		prefix+".vcs",
	)

	payload, _ = sjson.SetBytes(payload, prefix+".is_ci", false)
	payload, _ = sjson.SetBytes(payload, prefix+".is_github_action", false)
	payload, _ = sjson.SetBytes(payload, prefix+".is_claude_code_action", false)
	payload, _ = sjson.SetBytes(payload, prefix+".is_claude_code_remote", false)
	payload, _ = sjson.SetBytes(payload, prefix+".is_local_agent_mode", false)
	payload, _ = sjson.SetBytes(payload, prefix+".is_conductor", false)
	payload, _ = sjson.SetBytes(payload, prefix+".is_claubbit", false)
	return payload
}

func (s *TelemetryService) ForwardBackground(cleanedBody []byte, originalAuthToken string) {
	select {
	case forwardSem <- struct{}{}:
	default:
		logger.LegacyPrintf("service.telemetry", "[Warn] forward queue full, dropping telemetry batch")
		return
	}

	go func() {
		defer func() { <-forwardSem }()

		u := rand.Float64()
		if u == 0 {
			u = 0.0001
		}
		jitterMs := int(-1500 * math.Log(u))
		if jitterMs > 10000 {
			jitterMs = 10000
		}
		time.Sleep(time.Duration(jitterMs) * time.Millisecond)

		endpoint := "https://api.anthropic.com/api/event_logging/batch"
		req, err := http.NewRequest("POST", endpoint, bytes.NewBuffer(cleanedBody))
		if err != nil {
			logger.LegacyPrintf("service.telemetry", "[Error] failed to create telemetry request: %v", err)
			return
		}

		version := extractForwardVersion(cleanedBody)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", telemetryUserAgent(version))
		req.Header.Set("x-service-name", "claude-code")
		if originalAuthToken != "" {
			req.Header.Set("x-api-key", originalAuthToken)
		}

		resp, err := forwardClient.Do(req)
		if err != nil {
			logger.LegacyPrintf("service.telemetry", "[Error] failed to send shadow telemetry: %v", err)
			return
		}
		defer resp.Body.Close()

		logger.LegacyPrintf("service.telemetry", "[Success] Shadow telemetry dispatched (jitter=%dms), status=%d", jitterMs, resp.StatusCode)
	}()
}

func stableUUID(seed string) string {
	hash := sha256.Sum256([]byte(seed))
	hexHash := fmt.Sprintf("%x", hash)
	variantByte := []byte(hexHash[16:20])
	switch variantByte[0] {
	case '0', '1', '2', '3', '4', '5', '6', '7':
		variantByte[0] = '8'
	case 'c', 'd', 'e', 'f':
		variantByte[0] = 'a'
	}
	return fmt.Sprintf("%s-%s-4%s-%s-%s", hexHash[0:8], hexHash[8:12], hexHash[13:16], string(variantByte), hexHash[20:32])
}

func sanitizeUserAttributes(raw string, shadowDeviceID string, persona PersonaProfile) (string, bool) {
	if raw == "" {
		return "", false
	}
	payload := []byte(raw)
	if !gjson.ValidBytes(payload) {
		payload = []byte(`{}`)
	}
	payload = deletePaths(payload,
		"apiBaseUrlHost",
		"email",
		"githubActionsMetadata",
		"accountUUID",
		"account_uuid",
		"organizationUUID",
		"organization_uuid",
	)
	payload, _ = sjson.SetBytes(payload, "id", shadowDeviceID)
	payload, _ = sjson.SetBytes(payload, "deviceID", shadowDeviceID)
	payload, _ = sjson.SetBytes(payload, "platform", persona.Platform)
	return string(payload), true
}

func sanitizeAdditionalMetadata(raw string, persona PersonaProfile) (string, bool) {
	if raw == "" {
		return "", false
	}
	decodedMeta, err := base64.StdEncoding.DecodeString(raw)
	if err != nil || !gjson.ValidBytes(decodedMeta) {
		return "", false
	}
	decodedMeta = deletePaths(decodedMeta, "baseUrl", "gateway", "auth", "email", "organization_uuid", "organizationUUID")
	decodedMeta = overwriteEnvBlockSJSON(decodedMeta, "env", persona)
	return base64.StdEncoding.EncodeToString(decodedMeta), true
}

func extractAccountSeed(ev gjson.Result) string {
	userAttrs := ev.Get("event_data.user_attributes").String()
	return firstNonEmpty(
		ev.Get("event_data.auth.account_uuid").String(),
		ev.Get("event_data.auth.organization_uuid").String(),
		ev.Get("event_data.account_uuid").String(),
		ev.Get("event_data.accountUUID").String(),
		gjson.Get(userAttrs, "account_uuid").String(),
		gjson.Get(userAttrs, "accountUUID").String(),
		gjson.Get(userAttrs, "organization_uuid").String(),
		gjson.Get(userAttrs, "organizationUUID").String(),
	)
}

func extractForwardVersion(cleanedBody []byte) string {
	version := gjson.GetBytes(cleanedBody, "events.0.event_data.env.version").String()
	if version == "" {
		metaRaw := gjson.GetBytes(cleanedBody, "events.0.event_data.additional_metadata").String()
		if decoded, err := base64.StdEncoding.DecodeString(metaRaw); err == nil {
			version = gjson.GetBytes(decoded, "env.version").String()
		}
	}
	if version != "" && strings.HasPrefix(version, "2.") {
		return version
	}
	return defaultPersona.Version
}

func telemetryUserAgent(version string) string {
	if version == "" {
		version = defaultPersona.Version
	}
	return fmt.Sprintf("claude-code/%s", version)
}

func versionBase(version string) string {
	version = strings.TrimSpace(version)
	if version == "" {
		return defaultPersona.VersionBase
	}
	parts := strings.SplitN(version, "-", 2)
	return parts[0]
}

func selectTelemetryVersion(seed string) string {
	pool := telemetryVersionPool()
	if len(pool) == 0 {
		return defaultPersona.Version
	}
	if seed == "" {
		return pool[0]
	}
	hash := sha256.Sum256([]byte(seed + "version"))
	return pool[int(hash[0])%len(pool)]
}

func telemetryVersionPool() []string {
	poolRaw := strings.TrimSpace(os.Getenv("TELEMETRY_VERSION_POOL"))
	if poolRaw == "" {
		return append([]string(nil), defaultTelemetryVersionPool...)
	}
	parts := strings.Split(poolRaw, ",")
	pool := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			pool = append(pool, part)
		}
	}
	if len(pool) == 0 {
		return append([]string(nil), defaultTelemetryVersionPool...)
	}
	return pool
}

func syntheticEventTime(base time.Time, shadowDeviceID string, index int) time.Time {
	hash := sha256.Sum256([]byte(fmt.Sprintf("%s|%d|time", shadowDeviceID, index)))
	const maxOffset = 51 * time.Second
	fraction := float64(binary.BigEndian.Uint64(hash[:8])) / float64(^uint64(0))
	offset := time.Duration(fraction * float64(maxOffset))
	return base.Add(-offset)
}

func syntheticBuildTime(version string) string {
	known := map[string]string{
		"2.2.17": "2026-03-14T09:00:00Z",
		"2.2.18": "2026-03-20T09:00:00Z",
		"2.2.19": "2026-03-28T10:30:00Z",
		"2.3.0":  "2026-04-01T08:45:00Z",
	}
	if buildTime, ok := known[version]; ok {
		return buildTime
	}
	return defaultPersona.BuildTime
}

func syntheticProcessMetrics(shadowDeviceID string, syntheticTime time.Time) string {
	hash := sha256.Sum256([]byte(shadowDeviceID + "|process"))
	uptimeSeconds := 900 + int(syntheticTime.Unix()%86400) + int(hash[0])
	rss := 90_000_000 + int(hash[1])*350_000
	heapTotal := 28_000_000 + int(hash[2])*140_000
	heapUsed := 14_000_000 + int(hash[3])*95_000
	if heapUsed >= heapTotal {
		heapUsed = heapTotal - 512_000
	}
	external := 1_200_000 + int(hash[4])*14_000
	arrayBuffers := 180_000 + int(hash[5])*2_000
	constrainedMemory := 0
	cpuUser := 4_000_000 + int(hash[6])*18_000 + uptimeSeconds*800
	cpuSystem := 1_200_000 + int(hash[7])*9_000 + uptimeSeconds*300
	cpuPercent := 1.5 + float64(hash[8]%35)/10.0

	return fmt.Sprintf(`{"uptime":%d,"rss":%d,"heapTotal":%d,"heapUsed":%d,"external":%d,"arrayBuffers":%d,"constrainedMemory":%d,"cpuUsage":{"user":%d,"system":%d},"cpuPercent":%.1f}`,
		uptimeSeconds,
		rss,
		heapTotal,
		heapUsed,
		external,
		arrayBuffers,
		constrainedMemory,
		cpuUser,
		cpuSystem,
		cpuPercent,
	)
}

func deletePaths(payload []byte, paths ...string) []byte {
	for _, path := range paths {
		payload, _ = sjson.DeleteBytes(payload, path)
	}
	return payload
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func looksLikeJSONObject(raw string) bool {
	trimmed := strings.TrimSpace(raw)
	return strings.HasPrefix(trimmed, "{") && strings.HasSuffix(trimmed, "}")
}
