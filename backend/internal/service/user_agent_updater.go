package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/claude"
)

// UserAgentCache 定义 User-Agent 缓存接口
type UserAgentCache interface {
	GetLatestUserAgent(ctx context.Context) (string, error)
	SetLatestUserAgent(ctx context.Context, userAgent string, ttl time.Duration) error
}

// UserAgentUpdater 自动更新 User-Agent 服务
type UserAgentUpdater struct {
	cfg           *config.Config
	cache         UserAgentCache
	mu            sync.RWMutex
	currentUA     string
	stopCh        chan struct{}
	httpClient    *http.Client
	learningRegex *regexp.Regexp
}

// NewUserAgentUpdater 创建 User-Agent 更新服务
func NewUserAgentUpdater(cfg *config.Config, cache UserAgentCache) *UserAgentUpdater {
	return &UserAgentUpdater{
		cfg:   cfg,
		cache: cache,
		// 初始化为配置的默认值
		currentUA: getConfiguredUserAgent(cfg),
		stopCh:    make(chan struct{}),
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		// 匹配 claude-cli/x.y.z 格式
		learningRegex: regexp.MustCompile(`^claude-cli/(\d+\.\d+\.\d+)`),
	}
}

// Start 启动自动更新服务
func (u *UserAgentUpdater) Start(ctx context.Context) {
	if !u.isAutoUpdateEnabled() {
		log.Println("[UserAgentUpdater] Auto-update disabled, using configured value")
		return
	}

	log.Println("[UserAgentUpdater] Starting auto-update service")

	// 立即尝试从缓存加载
	go u.loadFromCache(ctx)

	// 启动后台更新任务
	go u.updateLoop(ctx)
}

// Stop 停止更新服务
func (u *UserAgentUpdater) Stop() {
	select {
	case <-u.stopCh:
		return
	default:
		close(u.stopCh)
	}
}

// GetUserAgent 获取当前的 User-Agent
func (u *UserAgentUpdater) GetUserAgent() string {
	u.mu.RLock()
	defer u.mu.RUnlock()
	return u.currentUA
}

// LearnFromRequest 从客户端请求中学习 User-Agent
// 当检测到更新的 claude-cli 版本时，自动更新
func (u *UserAgentUpdater) LearnFromRequest(userAgent string) {
	if !u.isLearningEnabled() {
		return
	}

	userAgent = strings.TrimSpace(userAgent)
	if userAgent == "" || !strings.HasPrefix(userAgent, "claude-cli/") {
		return
	}

	// 解析版本号
	matches := u.learningRegex.FindStringSubmatch(userAgent)
	if len(matches) < 2 {
		return
	}

	// 如果客户端版本更新，则更新（原子比较 + 更新）
	if updated, previous := u.updateIfNewer(userAgent); updated {
		log.Printf("[UserAgentUpdater] Learned newer version from request: %s (previous: %s)", userAgent, previous)
	}
}

// updateLoop 后台更新循环
func (u *UserAgentUpdater) updateLoop(ctx context.Context) {
	interval := u.getUpdateInterval()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// 启动后立即检查一次
	u.checkAndUpdate(ctx)

	for {
		select {
		case <-u.stopCh:
			log.Println("[UserAgentUpdater] Update loop stopped")
			return
		case <-ctx.Done():
			return
		case <-ticker.C:
			u.checkAndUpdate(ctx)
		}
	}
}

// checkAndUpdate 检查并更新 User-Agent
func (u *UserAgentUpdater) checkAndUpdate(ctx context.Context) {
	if !u.isAutoUpdateEnabled() {
		return
	}

	log.Println("[UserAgentUpdater] Checking for updates...")

	// 从 npm registry 获取最新版本
	latestUA, err := u.fetchLatestFromRegistry(ctx)
	if err != nil {
		log.Printf("[UserAgentUpdater] Failed to fetch from npm registry: %v", err)
		return
	}

	if latestUA == "" {
		return
	}

	if updated, previous := u.updateIfNewer(latestUA); updated {
		log.Printf("[UserAgentUpdater] Updating User-Agent: %s -> %s", previous, latestUA)
	} else if latestUA == u.GetUserAgent() {
		log.Println("[UserAgentUpdater] Already using latest version")
	}
}

// fetchLatestFromRegistry 从 npm registry 获取最新版本
func (u *UserAgentUpdater) fetchLatestFromRegistry(ctx context.Context) (string, error) {
	// 从 npm registry 获取 Claude Code CLI 的最新版本
	// npm 包：@anthropic-ai/claude-code
	// API 文档：https://github.com/npm/registry/blob/master/docs/REGISTRY-API.md
	url := "https://registry.npmjs.org/@anthropic-ai/claude-code/latest"

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}

	// 设置 User-Agent
	req.Header.Set("User-Agent", "sub2api-updater")
	req.Header.Set("Accept", "application/json")

	resp, err := u.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			log.Printf("[UserAgentUpdater] Failed to close response body: %v", closeErr)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("npm registry API returned %d: %s", resp.StatusCode, string(body))
	}

	// 解析 npm 包信息
	var pkgInfo struct {
		Version string `json:"version"`
		Name    string `json:"name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&pkgInfo); err != nil {
		return "", err
	}

	if pkgInfo.Version == "" {
		return "", fmt.Errorf("npm 包信息中没有版本号")
	}

	// 构建 User-Agent
	// Claude Code 的标准格式：claude-cli/x.y.z (external, cli)
	userAgent := fmt.Sprintf("claude-cli/%s (external, cli)", pkgInfo.Version)
	return userAgent, nil
}

// updateUserAgent 更新 User-Agent 并缓存
func (u *UserAgentUpdater) updateUserAgent(newUA string) {
	newUA = strings.TrimSpace(newUA)
	if newUA == "" {
		return
	}
	u.mu.Lock()
	u.currentUA = newUA
	u.mu.Unlock()

	u.cacheUserAgentIfCurrent(newUA)
}

// loadFromCache 从缓存加载 User-Agent
func (u *UserAgentUpdater) loadFromCache(ctx context.Context) {
	if u.cache == nil {
		return
	}

	cachedUA, err := u.cache.GetLatestUserAgent(ctx)
	if err != nil {
		// 缓存未命中不算错误
		return
	}

	if cachedUA != "" {
		if updated, _ := u.updateIfNewer(cachedUA); updated {
			log.Printf("[UserAgentUpdater] Loaded from cache: %s", cachedUA)
		}
	}
}

// updateIfNewer 在同一把锁内完成比较与更新，避免并发覆盖
// 返回是否更新，以及更新前的 User-Agent
func (u *UserAgentUpdater) updateIfNewer(newUA string) (bool, string) {
	newUA = strings.TrimSpace(newUA)
	if newUA == "" {
		return false, ""
	}

	newVersion := u.extractVersion(newUA)
	if newVersion == "" {
		return false, ""
	}

	u.mu.Lock()
	previous := u.currentUA
	currentVersion := u.extractVersion(previous)
	if !u.isNewerVersion(newVersion, currentVersion) {
		u.mu.Unlock()
		return false, previous
	}
	u.currentUA = newUA
	u.mu.Unlock()

	u.cacheUserAgentIfCurrent(newUA)
	return true, previous
}

// cacheUserAgentIfCurrent 仅在值仍为最新时写入缓存，避免并发回写旧值
func (u *UserAgentUpdater) cacheUserAgentIfCurrent(userAgent string) {
	if u.cache == nil {
		return
	}
	if u.GetUserAgent() != userAgent {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := u.cache.SetLatestUserAgent(ctx, userAgent, 24*time.Hour); err != nil {
		log.Printf("[UserAgentUpdater] Failed to cache User-Agent: %v", err)
	}
}

// extractVersion 从 User-Agent 中提取版本号
func (u *UserAgentUpdater) extractVersion(userAgent string) string {
	matches := u.learningRegex.FindStringSubmatch(userAgent)
	if len(matches) < 2 {
		return ""
	}
	return matches[1]
}

// isNewerVersion 比较版本号（简单的字符串比较）
// 格式：x.y.z
func (u *UserAgentUpdater) isNewerVersion(newVer, oldVer string) bool {
	if newVer == "" || oldVer == "" {
		return false
	}

	newParts := strings.Split(newVer, ".")
	oldParts := strings.Split(oldVer, ".")

	// 补齐到相同长度
	maxLen := len(newParts)
	if len(oldParts) > maxLen {
		maxLen = len(oldParts)
	}

	for len(newParts) < maxLen {
		newParts = append(newParts, "0")
	}
	for len(oldParts) < maxLen {
		oldParts = append(oldParts, "0")
	}

	// 逐段比较
	for i := 0; i < maxLen; i++ {
		newNum, newErr := strconv.Atoi(newParts[i])
		oldNum, oldErr := strconv.Atoi(oldParts[i])
		if newErr != nil || oldErr != nil {
			return false
		}

		if newNum > oldNum {
			return true
		} else if newNum < oldNum {
			return false
		}
	}

	return false
}

// 配置相关方法

func (u *UserAgentUpdater) isAutoUpdateEnabled() bool {
	if u.cfg == nil {
		return false
	}
	return u.cfg.Gateway.UserAgentAutoUpdate
}

func (u *UserAgentUpdater) isLearningEnabled() bool {
	if u.cfg == nil {
		return false
	}
	return u.cfg.Gateway.UserAgentLearnFromRequests
}

func (u *UserAgentUpdater) getUpdateInterval() time.Duration {
	if u.cfg == nil || u.cfg.Gateway.UserAgentUpdateIntervalHours <= 0 {
		return 24 * time.Hour // 默认每天检查一次
	}
	return time.Duration(u.cfg.Gateway.UserAgentUpdateIntervalHours) * time.Hour
}

// getConfiguredUserAgent 获取配置的 User-Agent
func getConfiguredUserAgent(cfg *config.Config) string {
	// 1. 配置文件
	if cfg != nil && strings.TrimSpace(cfg.Gateway.DefaultUserAgent) != "" {
		return cfg.Gateway.DefaultUserAgent
	}
	// 2. 环境变量
	if envUA := strings.TrimSpace(os.Getenv("DEFAULT_USER_AGENT")); envUA != "" {
		return envUA
	}
	// 3. 内置默认值
	return claude.DefaultHeaders["User-Agent"]
}
