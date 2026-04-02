package service

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
)

// =========================================================================
// Persona Profile — 每个底层账号对应的虚拟设备环境画像
// 所有经过 sub2api 网关的遥测数据，环境信息都会被统一涂抹为此画像。
// 目的：让官方看到的永远是「一个高级 Mac 开发者在多窗口高频使用」。
// =========================================================================

// PersonaProfile 描述一台虚拟设备的完整环境指纹
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
	Version               string `json:"version"`       // claude-code 版本号
	VersionBase           string `json:"version_base"`   // 版本基线
}

// defaultPersona 硬编码的默认虚拟设备画像：一台典型的 macOS 高级开发者工作站
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

// TelemetryService 处理独立于模型的打点遥测和审计机制拦截与清洗
type TelemetryService struct{}

func NewTelemetryService() *TelemetryService {
	return &TelemetryService{}
}

// GenerateShadowDeviceID 基于底层账号的 accountUUID（优先）或原始 device_id 生成稳定影子 ID。
// 同一个底层账号 → 同一个 ShadowDeviceID，无论背后有多少个真实终端。
func (s *TelemetryService) GenerateShadowDeviceID(seed string) string {
	if seed == "" {
		seed = "anonymous"
	}
	hash := sha256.Sum256([]byte(seed + "_sub2api_telemetry_salt"))
	hexHash := fmt.Sprintf("%x", hash)
	// 生成 UUIDv4 形态
	return fmt.Sprintf("%s-%s-4%s-%s-%s",
		hexHash[0:8],
		hexHash[8:12],
		hexHash[13:16],
		hexHash[16:20],
		hexHash[20:32],
	)
}

// DeepScrubPayload 核心消杀引擎。
// 对 /api/event_logging/batch 的 JSON 包体执行三层手术：
//  1. PII 剔除（email, githubActionsMetadata, apiBaseUrlHost, baseUrl, gateway）
//  2. 设备身份统一（device_id → 基于 accountUUID 的稳定影子 ID）
//  3. 环境指纹涂抹（platform, arch, nodeVersion 等 → 统一虚拟画像）
func (s *TelemetryService) DeepScrubPayload(bodyBytes []byte) ([]byte, error) {
	var payload map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &payload); err != nil {
		return nil, err
	}

	eventsData, ok := payload["events"]
	if !ok {
		return bodyBytes, nil
	}

	events, ok := eventsData.([]interface{})
	if !ok {
		return bodyBytes, nil
	}

	for i, ev := range events {
		eventMap, ok := ev.(map[string]interface{})
		if !ok {
			continue
		}

		eventType, _ := eventMap["event_type"].(string)
		eventData, ok := eventMap["event_data"].(map[string]interface{})
		if !ok {
			continue
		}

		// ——————————————————————————————————————————————
		// Step 1: 确定影子设备 ID 的种子
		// 优先用 accountUUID（保证同一底层号 → 同一台虚拟机器）
		// 否则回退到原始 device_id（保证单用户稳定）
		// ——————————————————————————————————————————————
		var idSeed string

		// 尝试从 Growthbook user_attributes 提取 accountUUID
		if eventType == "GrowthbookExperimentEvent" {
			if userAttrStr, ok := eventData["user_attributes"].(string); ok {
				var tmpAttrs map[string]interface{}
				if json.Unmarshal([]byte(userAttrStr), &tmpAttrs) == nil {
					if uuid, ok := tmpAttrs["accountUUID"].(string); ok && uuid != "" {
						idSeed = uuid
					}
				}
			}
		}

		// 尝试从 ClaudeCodeInternalEvent 顶层提取
		if idSeed == "" {
			if uuid, ok := eventData["accountUUID"].(string); ok && uuid != "" {
				idSeed = uuid
			}
		}

		// 最终兜底：用原始 device_id
		if idSeed == "" {
			if devID, ok := eventData["device_id"].(string); ok {
				idSeed = devID
			} else if devID, ok := eventMap["device_id"].(string); ok {
				idSeed = devID
			}
		}

		shadowDeviceID := s.GenerateShadowDeviceID(idSeed)

		// ——————————————————————————————————————————————
		// Step 2: 全局替换最外层 device_id
		// ——————————————————————————————————————————————
		if _, has := eventMap["device_id"]; has {
			eventMap["device_id"] = shadowDeviceID
		}

		// ——————————————————————————————————————————————
		// GrowthbookExperimentEvent 处理
		// ——————————————————————————————————————————————
		if eventType == "GrowthbookExperimentEvent" {
			if _, has := eventData["device_id"]; has {
				eventData["device_id"] = shadowDeviceID
			}

			// 拆解 user_attributes JSON 字符串
			if userAttrStr, ok := eventData["user_attributes"].(string); ok {
				var attrMap map[string]interface{}
				if err := json.Unmarshal([]byte(userAttrStr), &attrMap); err == nil {
					// 致命 PII 清除
					delete(attrMap, "apiBaseUrlHost")
					delete(attrMap, "email")
					delete(attrMap, "githubActionsMetadata")

					// 身份统一
					if _, has := attrMap["id"]; has {
						attrMap["id"] = shadowDeviceID
					}
					if _, has := attrMap["deviceID"]; has {
						attrMap["deviceID"] = shadowDeviceID
					}

					// 环境涂抹
					attrMap["platform"] = defaultPersona.Platform

					// 回写
					if newAttrBytes, err := json.Marshal(attrMap); err == nil {
						eventData["user_attributes"] = string(newAttrBytes)
					}
				}
			}

		// ——————————————————————————————————————————————
		// ClaudeCodeInternalEvent 处理
		// ——————————————————————————————————————————————
		} else if eventType == "ClaudeCodeInternalEvent" {
			// 顶层身份处理
			if _, has := eventData["device_id"]; has {
				eventData["device_id"] = shadowDeviceID
			}
			delete(eventData, "email") // git config user.email 致命泄露

			// 解析 additional_metadata (Base64 → JSON)
			if b64Meta, ok := eventData["additional_metadata"].(string); ok {
				decodedMeta, err := base64.StdEncoding.DecodeString(b64Meta)
				if err == nil {
					var metaMap map[string]interface{}
					if err := json.Unmarshal(decodedMeta, &metaMap); err == nil {
						// 剥离代理网关特征
						delete(metaMap, "baseUrl")
						delete(metaMap, "gateway")

						// ——————————————————————————————
						// 环境指纹涂抹：覆写 env 子对象
						// ——————————————————————————————
						s.overwriteEnvBlock(metaMap)

						// 回装 Base64
						if newMetaBytes, err := json.Marshal(metaMap); err == nil {
							eventData["additional_metadata"] = base64.StdEncoding.EncodeToString(newMetaBytes)
						}
					}
				}
			}
		}

		// 写回
		eventMap["event_data"] = eventData
		events[i] = eventMap
	}

	payload["events"] = events
	return json.Marshal(payload)
}

// overwriteEnvBlock 对 additional_metadata 解码后的 JSON 中的 env 子对象执行环境涂抹。
// 如果 env 不存在则创建，确保发出去的数据环境指纹 100% 一致。
func (s *TelemetryService) overwriteEnvBlock(metaMap map[string]interface{}) {
	envBlock, ok := metaMap["env"].(map[string]interface{})
	if !ok {
		envBlock = make(map[string]interface{})
	}

	// 强制覆写核心环境字段
	envBlock["platform"] = defaultPersona.Platform
	envBlock["platform_raw"] = defaultPersona.PlatformRaw
	envBlock["arch"] = defaultPersona.Arch
	envBlock["node_version"] = defaultPersona.NodeVersion
	envBlock["terminal"] = defaultPersona.Terminal
	envBlock["package_managers"] = defaultPersona.PackageManagers
	envBlock["runtimes"] = defaultPersona.Runtimes
	envBlock["is_running_with_bun"] = defaultPersona.IsRunningWithBun
	envBlock["deployment_environment"] = defaultPersona.DeploymentEnvironment

	// 版本号可选覆写（如果画像里配了的话）
	if defaultPersona.Version != "" {
		envBlock["version"] = defaultPersona.Version
	}
	if defaultPersona.VersionBase != "" {
		envBlock["version_base"] = defaultPersona.VersionBase
	}

	// 清除可能暴露真实物理环境的可选字段
	delete(envBlock, "wsl_version")
	delete(envBlock, "linux_distro_id")
	delete(envBlock, "linux_distro_version")
	delete(envBlock, "linux_kernel")
	delete(envBlock, "github_actions_metadata")
	delete(envBlock, "github_event_name")
	delete(envBlock, "github_actions_runner_environment")
	delete(envBlock, "github_actions_runner_os")
	delete(envBlock, "github_action_ref")

	// 统一标记为非 CI / 非远程 / 非容器
	envBlock["is_ci"] = false
	envBlock["is_github_action"] = false
	envBlock["is_claude_code_action"] = false
	envBlock["is_claude_code_remote"] = false
	envBlock["is_local_agent_mode"] = false
	envBlock["is_conductor"] = false
	envBlock["is_claubbit"] = false

	metaMap["env"] = envBlock
}

// ForwardBackground 异步转发清洗后的遥测数据至官方端点。
// 加入随机延迟（0~3秒），防止同一底层账号在同一毫秒内收到多个设备的并发包。
func (s *TelemetryService) ForwardBackground(cleanedBody []byte, originalAuthToken string) {
	go func() {
		// 随机延迟 0~3000ms，打散时间指纹
		jitter := time.Duration(rand.Intn(3000)) * time.Millisecond
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

		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			logger.LegacyPrintf("service.telemetry", "[Error] failed to send shadow telemetry: %v", err)
			return
		}
		defer resp.Body.Close()

		logger.LegacyPrintf("service.telemetry", "[Success] Shadow telemetry dispatched (jitter=%dms), Status: %d", jitter.Milliseconds(), resp.StatusCode)
	}()
}
