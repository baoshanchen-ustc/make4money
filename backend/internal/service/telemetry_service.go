package service

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"math"
	"math/rand"
	"net/http"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
)

// =========================================================================
// Persona Profile
// =========================================================================

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
	Version:               "2.2.17",
	VersionBase:           "2.2.17",
}

// 转发 goroutine 限流 channel：最多 64 个并发转发
var forwardSem = make(chan struct{}, 64)

var forwardClient *http.Client

func init() {
	dialer := tlsfingerprint.NewDialer(nil, nil)
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

func (s *TelemetryService) GenerateShadowDeviceID(accountUUID string, originalDeviceID string) string {
	seed := accountUUID
	if originalDeviceID != "" {
		seed += "|" + originalDeviceID
	}
	if seed == "" {
		seed = "anonymous"
	}
	hash := sha256.Sum256([]byte(seed + telemetrySalt))
	hexHash := fmt.Sprintf("%x", hash)
	variantByte := hexHash[16:20]
	variantRune := []byte(variantByte)
	switch variantRune[0] {
	case '0', '1', '2', '3', '4', '5', '6', '7':
		variantRune[0] = '8'
	case 'c', 'd', 'e', 'f':
		variantRune[0] = 'a'
	}
	return fmt.Sprintf("%s-%s-4%s-%s-%s",
		hexHash[0:8],
		hexHash[8:12],
		hexHash[13:16],
		string(variantRune),
		hexHash[20:32],
	)
}

func (s *TelemetryService) GenerateMappedUUID(shadowDeviceID, originalID string) string {
	hash := sha256.Sum256([]byte(shadowDeviceID + originalID + telemetrySalt))
	hexHash := fmt.Sprintf("%x", hash)
	variantByte := hexHash[16:20]
	variantRune := []byte(variantByte)
	switch variantRune[0] {
	case '0', '1', '2', '3', '4', '5', '6', '7':
		variantRune[0] = '8'
	case 'c', 'd', 'e', 'f':
		variantRune[0] = 'a'
	}
	return fmt.Sprintf("%s-%s-4%s-%s-%s",
		hexHash[0:8],
		hexHash[8:12],
		hexHash[13:16],
		string(variantRune),
		hexHash[20:32],
	)
}

// GenerateDynamicPersona 从 deviceID 派生稳定的噪音，增加指纹混乱度
func (s *TelemetryService) GenerateDynamicPersona(shadowDeviceID string) PersonaProfile {
	persona := defaultPersona
	// 使用 hash 保证单个设备稳定
	hash := sha256.Sum256([]byte(shadowDeviceID + "persona"))
	
	val := int(hash[0])
	
	// 偶尔变更终端
	if val%10 < 3 {
		persona.Terminal = "Terminal.app"
	} else if val%10 < 5 {
		persona.Terminal = "vscode"
	} else if val%10 < 7 {
		persona.Terminal = "tmux"
	}
	
	// 偶尔变更 Node 修正版本
	minor := val % 5
	persona.NodeVersion = fmt.Sprintf("v22.13.%d", minor)

	// OS 系统可以有 x64 和 aarch64
	if (val/10)%10 < 2 {
		persona.Arch = "x64"
	}
	
	return persona
}

func (s *TelemetryService) DeepScrubPayload(bodyBytes []byte) ([]byte, error) {
	if !gjson.ValidBytes(bodyBytes) {
		return bodyBytes, nil
	}

	eventsRes := gjson.GetBytes(bodyBytes, "events")
	if !eventsRes.Exists() || !eventsRes.IsArray() {
		return bodyBytes, nil
	}

	resultBytes := bodyBytes

	for i, ev := range eventsRes.Array() {
		basePath := fmt.Sprintf("events.%d", i)
		eventType := ev.Get("event_type").String()

		var accountUUID string
		var origDevID string

		if eventType == "GrowthbookExperimentEvent" {
			userAttrStr := ev.Get("event_data.user_attributes").String()
			if userAttrStr != "" {
				accountUUID = gjson.Get(userAttrStr, "accountUUID").String()
			}
		}
		if accountUUID == "" {
			accountUUID = ev.Get("event_data.accountUUID").String()
		}

		origDevID = ev.Get("event_data.device_id").String()
		if origDevID == "" {
			origDevID = ev.Get("device_id").String()
		}

		shadowDeviceID := s.GenerateShadowDeviceID(accountUUID, origDevID)
		
		if ev.Get("device_id").Exists() {
			resultBytes, _ = sjson.SetBytes(resultBytes, basePath+".device_id", shadowDeviceID)
		}
		if ev.Get("event_data.device_id").Exists() {
			resultBytes, _ = sjson.SetBytes(resultBytes, basePath+".event_data.device_id", shadowDeviceID)
		}

		// Rewrite Session & Event IDs to prevent correlation leakage
		origSessionID := ev.Get("event_data.session_id").String()
		if origSessionID != "" {
			newSessionID := s.GenerateMappedUUID(shadowDeviceID, origSessionID)
			resultBytes, _ = sjson.SetBytes(resultBytes, basePath+".event_data.session_id", newSessionID)
		}

		origParentSessionID := ev.Get("event_data.parent_session_id").String()
		if origParentSessionID != "" {
			newParentSessionID := s.GenerateMappedUUID(shadowDeviceID, origParentSessionID)
			resultBytes, _ = sjson.SetBytes(resultBytes, basePath+".event_data.parent_session_id", newParentSessionID)
		}

		newEventID := uuid.New().String()
		if ev.Get("event_data.event_id").Exists() {
			resultBytes, _ = sjson.SetBytes(resultBytes, basePath+".event_data.event_id", newEventID)
		}
		if ev.Get("event_id").Exists() {
			resultBytes, _ = sjson.SetBytes(resultBytes, basePath+".event_id", newEventID)
		}
		
		persona := s.GenerateDynamicPersona(shadowDeviceID)

		if eventType == "GrowthbookExperimentEvent" {
			userAttrStr := ev.Get("event_data.user_attributes").String()
			if userAttrStr != "" {
				userAttrRaw := []byte(userAttrStr)
				userAttrRaw, _ = sjson.DeleteBytes(userAttrRaw, "apiBaseUrlHost")
				userAttrRaw, _ = sjson.DeleteBytes(userAttrRaw, "email")
				userAttrRaw, _ = sjson.DeleteBytes(userAttrRaw, "githubActionsMetadata")
				if gjson.GetBytes(userAttrRaw, "id").Exists() {
					userAttrRaw, _ = sjson.SetBytes(userAttrRaw, "id", shadowDeviceID)
				}
				if gjson.GetBytes(userAttrRaw, "deviceID").Exists() {
					userAttrRaw, _ = sjson.SetBytes(userAttrRaw, "deviceID", shadowDeviceID)
				}
				userAttrRaw, _ = sjson.SetBytes(userAttrRaw, "platform", persona.Platform)
				
				resultBytes, _ = sjson.SetBytes(resultBytes, basePath+".event_data.user_attributes", string(userAttrRaw))
			}
		} else if eventType == "ClaudeCodeInternalEvent" {
			resultBytes, _ = sjson.DeleteBytes(resultBytes, basePath+".event_data.email")

			b64Meta := ev.Get("event_data.additional_metadata").String()
			if b64Meta != "" {
				decodedMeta, err := base64.StdEncoding.DecodeString(b64Meta)
				if err == nil {
					decodedMeta, _ = sjson.DeleteBytes(decodedMeta, "baseUrl")
					decodedMeta, _ = sjson.DeleteBytes(decodedMeta, "gateway")

					decodedMeta = s.overwriteEnvBlockSJSON(decodedMeta, persona)

					newB64Meta := base64.StdEncoding.EncodeToString(decodedMeta)
					resultBytes, _ = sjson.SetBytes(resultBytes, basePath+".event_data.additional_metadata", newB64Meta)
				}
			}
		}
	}

	return resultBytes, nil
}

func (s *TelemetryService) overwriteEnvBlockSJSON(metaBytes []byte, persona PersonaProfile) []byte {
	
	metaBytes, _ = sjson.SetBytes(metaBytes, "env.platform", persona.Platform)
	metaBytes, _ = sjson.SetBytes(metaBytes, "env.platform_raw", persona.PlatformRaw)
	metaBytes, _ = sjson.SetBytes(metaBytes, "env.arch", persona.Arch)
	metaBytes, _ = sjson.SetBytes(metaBytes, "env.node_version", persona.NodeVersion)
	metaBytes, _ = sjson.SetBytes(metaBytes, "env.terminal", persona.Terminal)
	metaBytes, _ = sjson.SetBytes(metaBytes, "env.package_managers", persona.PackageManagers)
	metaBytes, _ = sjson.SetBytes(metaBytes, "env.runtimes", persona.Runtimes)
	metaBytes, _ = sjson.SetBytes(metaBytes, "env.is_running_with_bun", persona.IsRunningWithBun)
	metaBytes, _ = sjson.SetBytes(metaBytes, "env.deployment_environment", persona.DeploymentEnvironment)

	if persona.Version != "" {
		metaBytes, _ = sjson.SetBytes(metaBytes, "env.version", persona.Version)
	}
	if persona.VersionBase != "" {
		metaBytes, _ = sjson.SetBytes(metaBytes, "env.version_base", persona.VersionBase)
	}

	metaBytes, _ = sjson.DeleteBytes(metaBytes, "env.wsl_version")
	metaBytes, _ = sjson.DeleteBytes(metaBytes, "env.linux_distro_id")
	metaBytes, _ = sjson.DeleteBytes(metaBytes, "env.linux_distro_version")
	metaBytes, _ = sjson.DeleteBytes(metaBytes, "env.linux_kernel")
	metaBytes, _ = sjson.DeleteBytes(metaBytes, "env.github_actions_metadata")
	metaBytes, _ = sjson.DeleteBytes(metaBytes, "env.github_event_name")
	metaBytes, _ = sjson.DeleteBytes(metaBytes, "env.github_actions_runner_environment")
	metaBytes, _ = sjson.DeleteBytes(metaBytes, "env.github_actions_runner_os")
	metaBytes, _ = sjson.DeleteBytes(metaBytes, "env.github_action_ref")

	metaBytes, _ = sjson.SetBytes(metaBytes, "env.is_ci", false)
	metaBytes, _ = sjson.SetBytes(metaBytes, "env.is_github_action", false)
	metaBytes, _ = sjson.SetBytes(metaBytes, "env.is_claude_code_action", false)
	metaBytes, _ = sjson.SetBytes(metaBytes, "env.is_claude_code_remote", false)
	metaBytes, _ = sjson.SetBytes(metaBytes, "env.is_local_agent_mode", false)
	metaBytes, _ = sjson.SetBytes(metaBytes, "env.is_conductor", false)
	metaBytes, _ = sjson.SetBytes(metaBytes, "env.is_claubbit", false)

	return metaBytes
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

		// 泊松/指数分布延迟 (Mean 1500ms)
		u := rand.Float64()
		if u == 0 {
			u = 0.0001
		}
		jitterMs := int(-1500 * math.Log(u))
		if jitterMs > 10000 {
			jitterMs = 10000
		}
		jitter := time.Duration(jitterMs) * time.Millisecond
		time.Sleep(jitter)

		endpoint := "https://api.anthropic.com/api/event_logging/batch"

		req, err := http.NewRequest("POST", endpoint, bytes.NewBuffer(cleanedBody))
		if err != nil {
			logger.LegacyPrintf("service.telemetry", "[Error] failed to create telemetry request: %v", err)
			return
		}

		req.Header.Set("Content-Type", "application/json")
		if originalAuthToken != "" {
			req.Header.Set("x-api-key", originalAuthToken)
		}
		req.Header.Set("User-Agent", "claude-cli")

		resp, err := forwardClient.Do(req)
		if err != nil {
			logger.LegacyPrintf("service.telemetry", "[Error] failed to send shadow telemetry: %v", err)
			return
		}
		defer resp.Body.Close()

		logger.LegacyPrintf("service.telemetry", "[Success] Shadow telemetry dispatched (jitter=%dms), Status: %d", jitter.Milliseconds(), resp.StatusCode)
	}()
}
