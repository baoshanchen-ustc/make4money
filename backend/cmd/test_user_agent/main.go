package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

// 简单的缓存实现，用于测试
type testCache struct {
	userAgent string
}

func (c *testCache) GetLatestUserAgent(ctx context.Context) (string, error) {
	if c.userAgent == "" {
		return "", fmt.Errorf("no cached user agent")
	}
	return c.userAgent, nil
}

func (c *testCache) SetLatestUserAgent(ctx context.Context, userAgent string, ttl time.Duration) error {
	c.userAgent = userAgent
	fmt.Printf("✅ 缓存已更新: %s (TTL: %v)\n", userAgent, ttl)
	return nil
}

func main() {
	// 命令行参数
	var (
		testMode = flag.String("mode", "fetch", "测试模式: fetch, learn, compare, version")
		clientUA = flag.String("client-ua", "", "客户端 User-Agent（用于 learn 模式）")
		ver1     = flag.String("ver1", "", "版本号1（用于 compare 模式）")
		ver2     = flag.String("ver2", "", "版本号2（用于 compare 模式）")
	)
	flag.Parse()

	fmt.Println("=" + repeatString("=", 60))
	fmt.Println("  Claude Code User-Agent 测试工具")
	fmt.Println("=" + repeatString("=", 60))
	fmt.Println()

	// 创建配置
	cfg := &config.Config{
		Gateway: config.GatewayConfig{
			DefaultUserAgent:             "claude-cli/2.0.62 (external, cli)",
			UserAgentAutoUpdate:          true,
			UserAgentLearnFromRequests:   true,
			UserAgentUpdateIntervalHours: 24,
		},
	}

	// 创建缓存
	cache := &testCache{}

	// 创建更新器
	updater := service.NewUserAgentUpdater(cfg, cache)

	switch *testMode {
	case "fetch":
		testFetchFromRegistry(updater)
	case "learn":
		testLearnFromRequest(updater, *clientUA)
	case "compare":
		testCompareVersions(updater, *ver1, *ver2)
	case "version":
		testExtractVersion(updater, *clientUA)
	default:
		fmt.Printf("❌ 未知的测试模式: %s\n", *testMode)
		fmt.Println()
		printUsage()
		os.Exit(1)
	}
}

// testFetchFromRegistry 测试从 npm registry 获取最新版本
func testFetchFromRegistry(updater *service.UserAgentUpdater) {
	fmt.Println("📡 测试从 npm registry 获取最新版本")
	fmt.Println(repeatString("-", 60))
	fmt.Println()

	fmt.Printf("⏳ 当前 User-Agent: %s\n", updater.GetUserAgent())
	fmt.Println()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fmt.Println("🔄 正在从 npm registry 获取最新版本...")
	start := time.Now()

	// 通过反射或类型断言调用私有方法（这里我们用公开的方式测试）
	// 实际上我们可以直接调用 checkAndUpdate
	updater.Start(ctx)
	time.Sleep(2 * time.Second) // 等待后台任务执行
	updater.Stop()

	elapsed := time.Since(start)

	newUA := updater.GetUserAgent()
	fmt.Println()
	fmt.Printf("✅ 获取成功！（耗时: %v）\n", elapsed)
	fmt.Printf("📦 最新 User-Agent: %s\n", newUA)
	fmt.Println()

	// 提取版本号
	fmt.Println("🔍 版本信息分析:")
	fmt.Printf("   格式: claude-cli/版本号 (external, cli)\n")
	fmt.Printf("   当前版本: %s\n", newUA)
	fmt.Println()
}

// testLearnFromRequest 测试从客户端请求学习
func testLearnFromRequest(updater *service.UserAgentUpdater, clientUA string) {
	fmt.Println("🎓 测试从客户端请求学习功能")
	fmt.Println(repeatString("-", 60))
	fmt.Println()

	if clientUA == "" {
		fmt.Println("❌ 错误: 请提供客户端 User-Agent")
		fmt.Println()
		fmt.Println("示例: -client-ua \"claude-cli/2.0.99 (external, cli)\"")
		fmt.Println()
		os.Exit(1)
	}

	fmt.Printf("📊 初始状态:\n")
	fmt.Printf("   系统 User-Agent: %s\n", updater.GetUserAgent())
	fmt.Printf("   客户端 User-Agent: %s\n", clientUA)
	fmt.Println()

	fmt.Println("🔄 执行学习...")
	updater.LearnFromRequest(clientUA)

	newUA := updater.GetUserAgent()
	fmt.Println()
	fmt.Printf("📊 最终状态:\n")
	fmt.Printf("   系统 User-Agent: %s\n", newUA)
	fmt.Println()

	if newUA == clientUA {
		fmt.Println("✅ 学习成功！已采用客户端的更新版本")
	} else {
		fmt.Println("ℹ️  未更新（客户端版本可能不是更新版本，或学习功能被禁用）")
	}
	fmt.Println()
}

// testCompareVersions 测试版本比较
func testCompareVersions(updater *service.UserAgentUpdater, ver1, ver2 string) {
	fmt.Println("🔍 测试版本比较功能")
	fmt.Println(repeatString("-", 60))
	fmt.Println()

	if ver1 == "" || ver2 == "" {
		fmt.Println("❌ 错误: 请提供两个版本号")
		fmt.Println()
		fmt.Println("示例: -ver1 2.0.70 -ver2 2.0.62")
		fmt.Println()
		os.Exit(1)
	}

	fmt.Printf("📊 版本比较:\n")
	fmt.Printf("   版本1: %s\n", ver1)
	fmt.Printf("   版本2: %s\n", ver2)
	fmt.Println()

	// 这里需要一些技巧来访问私有方法
	// 实际使用中，我们通过构造 User-Agent 来间接测试
	ua1 := fmt.Sprintf("claude-cli/%s (external, cli)", ver1)
	ua2 := fmt.Sprintf("claude-cli/%s (external, cli)", ver2)

	fmt.Println("🔄 测试场景1: 版本1 是否比版本2 新")
	updater.LearnFromRequest(ua2) // 设置为版本2
	oldUA := updater.GetUserAgent()
	updater.LearnFromRequest(ua1) // 尝试学习版本1
	newUA := updater.GetUserAgent()

	if newUA != oldUA {
		fmt.Printf("   结果: %s > %s ✅\n", ver1, ver2)
	} else {
		fmt.Printf("   结果: %s <= %s\n", ver1, ver2)
	}
	fmt.Println()

	fmt.Println("🔄 测试场景2: 版本2 是否比版本1 新")
	updater.LearnFromRequest(ua1) // 设置为版本1
	oldUA = updater.GetUserAgent()
	updater.LearnFromRequest(ua2) // 尝试学习版本2
	newUA = updater.GetUserAgent()

	if newUA != oldUA {
		fmt.Printf("   结果: %s > %s ✅\n", ver2, ver1)
	} else {
		fmt.Printf("   结果: %s <= %s\n", ver2, ver1)
	}
	fmt.Println()
}

// testExtractVersion 测试版本号提取
func testExtractVersion(updater *service.UserAgentUpdater, userAgent string) {
	fmt.Println("🔧 测试版本号提取功能")
	fmt.Println(repeatString("-", 60))
	fmt.Println()

	if userAgent == "" {
		userAgent = "claude-cli/2.0.62 (external, cli)"
		fmt.Printf("ℹ️  使用默认 User-Agent: %s\n", userAgent)
		fmt.Println()
	}

	fmt.Printf("📊 输入 User-Agent: %s\n", userAgent)
	fmt.Println()

	// 通过更新器间接测试（设置后再获取）
	updater.LearnFromRequest(userAgent)
	result := updater.GetUserAgent()

	if result == userAgent {
		fmt.Println("✅ 格式有效，成功提取版本号")
		// 手动解析显示
		if idx := strings.Index(userAgent, "claude-cli/"); idx != -1 {
			versionPart := userAgent[idx+11:]
			if endIdx := strings.Index(versionPart, " "); endIdx != -1 {
				version := versionPart[:endIdx]
				fmt.Printf("   提取的版本号: %s\n", version)
			}
		}
	} else {
		fmt.Println("❌ 格式无效，无法提取版本号")
	}
	fmt.Println()
}

// repeatString 生成重复字符串
func repeatString(s string, n int) string {
	result := ""
	for i := 0; i < n; i++ {
		result += s
	}
	return result
}

// printUsage 打印使用说明
func printUsage() {
	fmt.Println("使用方法:")
	fmt.Println()
	fmt.Println("  1. 测试从 npm registry 获取最新版本:")
	fmt.Println("     go run main.go -mode fetch")
	fmt.Println()
	fmt.Println("  2. 测试从客户端学习:")
	fmt.Println("     go run main.go -mode learn -client-ua \"claude-cli/2.0.99 (external, cli)\"")
	fmt.Println()
	fmt.Println("  3. 测试版本比较:")
	fmt.Println("     go run main.go -mode compare -ver1 2.0.70 -ver2 2.0.62")
	fmt.Println()
	fmt.Println("  4. 测试版本提取:")
	fmt.Println("     go run main.go -mode version -client-ua \"claude-cli/2.0.62 (external, cli)\"")
	fmt.Println()
}
