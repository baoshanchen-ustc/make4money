package service

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
)

// TestDeepScrubPayload_FullPersona 综合测试：
// 模拟 10 个不同用户（不同 OS、不同 email、不同 device_id）使用同一个底层账号 accountUUID，
// 验证经过清洗后：
//  1. 所有人的 device_id 都收敛为同一个影子 ID（因为 accountUUID 相同）
//  2. 所有人的环境指纹都被统一涂抹为 defaultPersona（darwin/arm64）
//  3. 所有致命 PII 被彻底切除
//  4. accountUUID 被安全保留
func TestDeepScrubPayload_FullPersona(t *testing.T) {
	// 构造深层 Base64 嵌套：模拟 Linux 用户的 additional_metadata
	rawMeta := `{
		"baseUrl":"http://sub2api.local:8080/v1/messages",
		"gateway":"sub2api",
		"safe_info":"keep_this",
		"env":{
			"platform":"linux",
			"platform_raw":"linux",
			"arch":"x64",
			"node_version":"v18.20.0",
			"terminal":"gnome-terminal",
			"package_managers":"npm,yarn",
			"runtimes":"node",
			"is_running_with_bun":false,
			"deployment_environment":"unknown-linux",
			"wsl_version":"WSL2",
			"linux_distro_id":"ubuntu",
			"linux_distro_version":"22.04",
			"linux_kernel":"5.15.0",
			"is_ci":true,
			"is_github_action":true,
			"github_actions_metadata":{"actor_id":"12345","repository_id":"67890"}
		}
	}`
	encodedMeta := base64.StdEncoding.EncodeToString([]byte(rawMeta))

	// 模拟两种事件类型，来自不同 OS 的用户，但共用同一个 accountUUID
	mockPayload := `{
		"events": [
			{
				"event_type": "GrowthbookExperimentEvent",
				"event_data": {
					"device_id": "windows_device_aaa",
					"user_attributes": "{\"id\":\"windows_device_aaa\",\"deviceID\":\"windows_device_aaa\",\"apiBaseUrlHost\":\"sub2api.local:8080\",\"email\":\"user1@gmail.com\",\"githubActionsMetadata\":{\"repo\":\"secret\"},\"accountUUID\":\"shared-account-uuid-001\",\"platform\":\"win32\",\"subscriptionType\":\"pro\"}"
				}
			},
			{
				"event_type": "GrowthbookExperimentEvent",
				"event_data": {
					"device_id": "linux_device_bbb",
					"user_attributes": "{\"id\":\"linux_device_bbb\",\"deviceID\":\"linux_device_bbb\",\"apiBaseUrlHost\":\"192.168.1.100:3000\",\"email\":\"user2@company.org\",\"accountUUID\":\"shared-account-uuid-001\",\"platform\":\"linux\"}"
				}
			},
			{
				"event_type": "ClaudeCodeInternalEvent",
				"event_data": {
					"device_id": "mac_device_ccc",
					"email": "user3@hack.local",
					"accountUUID": "shared-account-uuid-001",
					"additional_metadata": "` + encodedMeta + `"
				}
			}
		]
	}`

	service := NewTelemetryService()
	scrubbedBytes, err := service.DeepScrubPayload([]byte(mockPayload))
	if err != nil {
		t.Fatalf("DeepScrubPayload failed: %v", err)
	}

	result := string(scrubbedBytes)
	t.Logf("Scrubbed JSON:\n%s\n", result)

	// ====== 解析输出 ======
	var parsedPayload map[string]interface{}
	json.Unmarshal(scrubbedBytes, &parsedPayload)
	events := parsedPayload["events"].([]interface{})

	// 收集所有 shadow device IDs
	var allShadowIDs []string

	for idx, ev := range events {
		evMap := ev.(map[string]interface{})
		evData := evMap["event_data"].(map[string]interface{})
		evType := evMap["event_type"].(string)

		deviceID := evData["device_id"].(string)
		allShadowIDs = append(allShadowIDs, deviceID)
		t.Logf("[Event %d] type=%s shadow_device_id=%s", idx, evType, deviceID)

		// ====== 致命 PII 不得泄露 ======
		evJSON, _ := json.Marshal(evData)
		evStr := string(evJSON)

		if strings.Contains(evStr, "user1@gmail.com") || strings.Contains(evStr, "user2@company.org") || strings.Contains(evStr, "user3@hack.local") {
			t.Errorf("[Event %d] FATAL: email leaked!", idx)
		}
		if strings.Contains(evStr, "apiBaseUrlHost") {
			t.Errorf("[Event %d] FATAL: apiBaseUrlHost leaked!", idx)
		}
		if strings.Contains(evStr, "sub2api") {
			t.Errorf("[Event %d] FATAL: gateway/sub2api signature leaked!", idx)
		}

		// ====== accountUUID 必须保留 ======
		if evType == "GrowthbookExperimentEvent" {
			if !strings.Contains(evStr, "shared-account-uuid-001") {
				t.Errorf("[Event %d] ERROR: accountUUID was wrongly deleted from Growthbook!", idx)
			}

			// 检查 user_attributes 中的 platform 是否已被涂抹
			if userAttrStr, ok := evData["user_attributes"].(string); ok {
				var attrMap map[string]interface{}
				json.Unmarshal([]byte(userAttrStr), &attrMap)

				if platform, ok := attrMap["platform"].(string); ok {
					if platform != "darwin" {
						t.Errorf("[Event %d] ERROR: Growthbook platform not overwritten! got=%s want=darwin", idx, platform)
					}
				}

				// subscriptionType 应当保留（仅当原始数据有此字段时才检验）
				if idx == 0 {
					if _, ok := attrMap["subscriptionType"]; !ok {
						t.Errorf("[Event %d] ERROR: subscriptionType was wrongly deleted!", idx)
					}
				}
			}
		}

		if evType == "ClaudeCodeInternalEvent" {
			// 检查 email 已被删除
			if _, hasEmail := evData["email"]; hasEmail {
				t.Errorf("[Event %d] FATAL: top-level email not deleted from Internal event!", idx)
			}

			// 深入 additional_metadata 检查环境涂抹
			newB64 := evData["additional_metadata"].(string)
			decoded, _ := base64.StdEncoding.DecodeString(newB64)
			var metaMap map[string]interface{}
			json.Unmarshal(decoded, &metaMap)
			t.Logf("[Event %d] Decoded additional_metadata: %s", idx, string(decoded))

			// baseUrl / gateway 必须消失
			if _, has := metaMap["baseUrl"]; has {
				t.Errorf("[Event %d] FATAL: baseUrl not removed from metadata!", idx)
			}
			if _, has := metaMap["gateway"]; has {
				t.Errorf("[Event %d] FATAL: gateway not removed from metadata!", idx)
			}

			// safe_info 必须保留
			if _, has := metaMap["safe_info"]; !has {
				t.Errorf("[Event %d] ERROR: safe_info was wrongly deleted!", idx)
			}

			// 环境涂抹验证
			envBlock, ok := metaMap["env"].(map[string]interface{})
			if !ok {
				t.Fatalf("[Event %d] FATAL: env block missing from metadata!", idx)
			}

			// 必须是 darwin/arm64
			if envBlock["platform"] != "darwin" {
				t.Errorf("[Event %d] FAIL: env.platform not overwritten! got=%v", idx, envBlock["platform"])
			}
			if envBlock["arch"] != "arm64" {
				t.Errorf("[Event %d] FAIL: env.arch not overwritten! got=%v", idx, envBlock["arch"])
			}
			if envBlock["node_version"] != "v22.13.1" {
				t.Errorf("[Event %d] FAIL: env.node_version not overwritten! got=%v", idx, envBlock["node_version"])
			}
			if envBlock["terminal"] != "iTerm.app" {
				t.Errorf("[Event %d] FAIL: env.terminal not overwritten! got=%v", idx, envBlock["terminal"])
			}

			// Linux 痕迹必须被清除
			if _, has := envBlock["wsl_version"]; has {
				t.Errorf("[Event %d] FAIL: wsl_version not removed!", idx)
			}
			if _, has := envBlock["linux_distro_id"]; has {
				t.Errorf("[Event %d] FAIL: linux_distro_id not removed!", idx)
			}
			if _, has := envBlock["linux_kernel"]; has {
				t.Errorf("[Event %d] FAIL: linux_kernel not removed!", idx)
			}
			if _, has := envBlock["github_actions_metadata"]; has {
				t.Errorf("[Event %d] FAIL: github_actions_metadata not removed from env!", idx)
			}

			// CI 标记必须被强制关闭
			if envBlock["is_ci"] != false {
				t.Errorf("[Event %d] FAIL: is_ci not forced to false!", idx)
			}
			if envBlock["is_github_action"] != false {
				t.Errorf("[Event %d] FAIL: is_github_action not forced to false!", idx)
			}
		}
	}

	// ====== 核心验证：三个不同用户因共用同一个 accountUUID，device_id 必须完全相同 ======
	if len(allShadowIDs) != 3 {
		t.Fatalf("Expected 3 events, got %d", len(allShadowIDs))
	}
	if allShadowIDs[0] != allShadowIDs[1] || allShadowIDs[1] != allShadowIDs[2] {
		t.Errorf("CRITICAL FAILURE: Same accountUUID produced DIFFERENT shadow device IDs!\n  Growthbook1=%s\n  Growthbook2=%s\n  Internal=%s",
			allShadowIDs[0], allShadowIDs[1], allShadowIDs[2])
	} else {
		t.Logf("SUCCESS: All 3 events converged to unified shadow device ID: %s", allShadowIDs[0])
	}

	// ====== 验证原始 device_id 绝不残留 ======
	if strings.Contains(result, "windows_device_aaa") || strings.Contains(result, "linux_device_bbb") || strings.Contains(result, "mac_device_ccc") {
		t.Errorf("FATAL: Original device_id leaked in output!")
	}
}

// TestDeepScrubPayload_EmptyAndMalformed 边界情况测试
func TestDeepScrubPayload_EmptyAndMalformed(t *testing.T) {
	svc := NewTelemetryService()

	tests := []struct {
		name      string
		input     string
		wantErr   bool
		wantSame  bool // 期望返回原始 bytes（无 events 字段时）
	}{
		{
			name:    "malformed JSON",
			input:   `{not valid json`,
			wantErr: true,
		},
		{
			name:     "no events field",
			input:    `{"foo":"bar"}`,
			wantSame: true,
		},
		{
			name:     "events is not array",
			input:    `{"events":"not_an_array"}`,
			wantSame: true,
		},
		{
			name:  "empty events array",
			input: `{"events":[]}`,
		},
		{
			name:  "event without event_data",
			input: `{"events":[{"event_type":"GrowthbookExperimentEvent"}]}`,
		},
		{
			name:  "event with non-object event_data",
			input: `{"events":[{"event_type":"GrowthbookExperimentEvent","event_data":"string_not_map"}]}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := svc.DeepScrubPayload([]byte(tt.input))
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.wantSame {
				// 无 events 时应返回原始内容
				var orig, got map[string]interface{}
				json.Unmarshal([]byte(tt.input), &orig)
				json.Unmarshal(result, &got)
				origBytes, _ := json.Marshal(orig)
				gotBytes, _ := json.Marshal(got)
				if string(origBytes) != string(gotBytes) {
					t.Errorf("expected unchanged payload, got diff")
				}
			}
		})
	}
}

// TestDeepScrubPayload_InvalidBase64Metadata Base64 格式异常不应 panic
func TestDeepScrubPayload_InvalidBase64Metadata(t *testing.T) {
	svc := NewTelemetryService()
	payload := `{
		"events": [{
			"event_type": "ClaudeCodeInternalEvent",
			"event_data": {
				"device_id": "dev-123",
				"additional_metadata": "NOT_VALID_BASE64!!!"
			}
		}]
	}`

	result, err := svc.DeepScrubPayload([]byte(payload))
	if err != nil {
		t.Fatalf("should not error on invalid base64: %v", err)
	}

	// device_id 应该仍被替换
	if strings.Contains(string(result), "dev-123") {
		t.Errorf("original device_id leaked despite bad metadata")
	}
}

// TestDeepScrubPayload_InvalidUserAttributes user_attributes 非法 JSON 不应 panic
func TestDeepScrubPayload_InvalidUserAttributes(t *testing.T) {
	svc := NewTelemetryService()
	payload := `{
		"events": [{
			"event_type": "GrowthbookExperimentEvent",
			"event_data": {
				"device_id": "dev-456",
				"user_attributes": "this is {not} valid json"
			}
		}]
	}`

	result, err := svc.DeepScrubPayload([]byte(payload))
	if err != nil {
		t.Fatalf("should not error on invalid user_attributes JSON: %v", err)
	}

	if strings.Contains(string(result), "dev-456") {
		t.Errorf("original device_id leaked despite bad user_attributes")
	}
}

// TestGenerateShadowDeviceID_UUIDFormat 验证生成的 ID 是合法 UUIDv4
func TestGenerateShadowDeviceID_UUIDFormat(t *testing.T) {
	svc := NewTelemetryService()

	seeds := []string{"test-uuid-1", "another-seed", "", "shared-account-uuid-001"}
	for _, seed := range seeds {
		id := svc.GenerateShadowDeviceID(seed, "")

		// 格式: 8-4-4-4-12
		parts := strings.Split(id, "-")
		if len(parts) != 5 {
			t.Errorf("seed=%q: expected 5 parts, got %d: %s", seed, len(parts), id)
			continue
		}
		if len(parts[0]) != 8 || len(parts[1]) != 4 || len(parts[2]) != 4 || len(parts[3]) != 4 || len(parts[4]) != 12 {
			t.Errorf("seed=%q: wrong part lengths in %s", seed, id)
			continue
		}

		// version nibble = 4
		if parts[2][0] != '4' {
			t.Errorf("seed=%q: version nibble should be '4', got '%c' in %s", seed, parts[2][0], id)
		}

		// variant nibble ∈ {8,9,a,b}
		v := parts[3][0]
		if v != '8' && v != '9' && v != 'a' && v != 'b' {
			t.Errorf("seed=%q: variant nibble should be 8/9/a/b, got '%c' in %s", seed, v, id)
		}
	}

	// 幂等性：同 seed → 同 ID
	id1 := svc.GenerateShadowDeviceID("stable-seed", "")
	id2 := svc.GenerateShadowDeviceID("stable-seed", "")
	if id1 != id2 {
		t.Errorf("same seed produced different IDs: %s vs %s", id1, id2)
	}
}
