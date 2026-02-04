package handler

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/xml"
	"io"
	"log"
	"sort"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
)

// WeChatScanInitResponse 扫码登录初始化响应
type WeChatScanInitResponse struct {
	SceneID       string `json:"scene_id"`
	QRCodeURL     string `json:"qr_code_url"`
	ExpireSeconds int    `json:"expire_seconds"`
}

// WeChatScanPollResponse 扫码登录轮询响应
type WeChatScanPollResponse struct {
	Status      string    `json:"status"` // "waiting", "confirmed"
	AccessToken string    `json:"access_token,omitempty"`
	TokenType   string    `json:"token_type,omitempty"`
	User        *dto.User `json:"user,omitempty"`
}

// WeChatMPMessage 微信公众号消息结构
type WeChatMPMessage struct {
	XMLName      xml.Name `xml:"xml"`
	ToUserName   string   `xml:"ToUserName"`
	FromUserName string   `xml:"FromUserName"` // 发送方 OpenID
	CreateTime   int64    `xml:"CreateTime"`
	MsgType      string   `xml:"MsgType"`
	Event        string   `xml:"Event"`      // SCAN 或 subscribe
	EventKey     string   `xml:"EventKey"`   // 扫码场景值（subscribe 时带 qrscene_ 前缀）
	Ticket       string   `xml:"Ticket"`     // 二维码票据
	Content      string   `xml:"Content"`    // 文本消息内容
	MsgId        int64    `xml:"MsgId"`
}

// WeChatMPReply 微信公众号回复消息
type WeChatMPReply struct {
	XMLName      xml.Name `xml:"xml"`
	ToUserName   string   `xml:"ToUserName"`
	FromUserName string   `xml:"FromUserName"`
	CreateTime   int64    `xml:"CreateTime"`
	MsgType      string   `xml:"MsgType"`
	Content      string   `xml:"Content,omitempty"`
}

// WeChatScanInit 初始化扫码登录会话
// GET /api/v1/auth/wechat/scan/init
func (h *AuthHandler) WeChatScanInit(c *gin.Context) {
	// 检查是否启用微信登录
	if !h.settingSvc.IsWeChatAuthEnabled(c.Request.Context()) {
		response.ErrorFrom(c, infraerrors.NotFound("WECHAT_AUTH_DISABLED", "管理员未开启微信登录"))
		return
	}

	// 检查扫码登录服务是否可用
	if h.scanLoginSvc == nil {
		response.ErrorFrom(c, infraerrors.ServiceUnavailable("SCAN_LOGIN_NOT_AVAILABLE", "扫码登录服务不可用"))
		return
	}

	// 检查是否为订阅号类型
	isSubscription, err := h.scanLoginSvc.IsSubscriptionAccountType(c.Request.Context())
	if err != nil {
		log.Printf("[WeChatScanInit] Failed to check account type: %v", err)
		response.ErrorFrom(c, infraerrors.InternalServer("CONFIG_ERROR", "获取配置失败"))
		return
	}
	if !isSubscription {
		response.ErrorFrom(c, infraerrors.BadRequest("ACCOUNT_TYPE_MISMATCH", "当前公众号类型不支持扫码自动登录"))
		return
	}

	// 初始化扫码会话
	session, err := h.scanLoginSvc.InitScanSession(c.Request.Context())
	if err != nil {
		log.Printf("[WeChatScanInit] Failed to init scan session: %v", err)
		response.ErrorFrom(c, infraerrors.ServiceUnavailable("INIT_FAILED", "初始化扫码登录失败"))
		return
	}

	response.Success(c, WeChatScanInitResponse{
		SceneID:       session.SceneID,
		QRCodeURL:     session.QRCodeURL,
		ExpireSeconds: session.ExpireSeconds,
	})
}

// WeChatScanPoll 轮询扫码登录状态
// GET /api/v1/auth/wechat/scan/poll?scene_id=xxx
func (h *AuthHandler) WeChatScanPoll(c *gin.Context) {
	sceneID := c.Query("scene_id")
	if sceneID == "" {
		response.BadRequest(c, "scene_id 不能为空")
		return
	}

	// 检查扫码登录服务是否可用
	if h.scanLoginSvc == nil {
		response.ErrorFrom(c, infraerrors.ServiceUnavailable("SCAN_LOGIN_NOT_AVAILABLE", "扫码登录服务不可用"))
		return
	}

	// 检查扫码结果
	result, err := h.scanLoginSvc.CheckScanResult(c.Request.Context(), sceneID)
	if err != nil {
		log.Printf("[WeChatScanPoll] Failed to check scan result: %v", err)
		response.ErrorFrom(c, infraerrors.InternalServer("CHECK_FAILED", "查询扫码状态失败"))
		return
	}

	// 如果还在等待
	if result.Status == "waiting" {
		response.Success(c, WeChatScanPollResponse{
			Status: "waiting",
		})
		return
	}

	// 扫码成功，执行登录逻辑
	openID := result.OpenID

	// 消费扫码结果（防止重复使用）
	if err := h.scanLoginSvc.ConsumeScanResult(c.Request.Context(), sceneID); err != nil {
		log.Printf("[WeChatScanPoll] Failed to consume scan result: %v", err)
		// 继续处理，不影响登录
	}

	// 优先检查是否有用户已绑定该 WeChat OpenID
	boundUser, err := h.userService.GetByWeChatOpenID(c.Request.Context(), openID)
	if err == nil && boundUser != nil {
		token, err := h.authService.GenerateToken(boundUser)
		if err != nil {
			log.Printf("[WeChatScanPoll] Failed to generate token for bound user: %v", err)
			response.ErrorFrom(c, err)
			return
		}
		response.Success(c, WeChatScanPollResponse{
			Status:      "confirmed",
			AccessToken: token,
			TokenType:   "Bearer",
			User:        dto.UserFromService(boundUser),
		})
		return
	}

	// 未找到绑定用户，使用合成邮箱登录或注册
	email := wechatSyntheticEmail(openID)
	username := wechatShortUsername(openID)

	token, user, err := h.authService.LoginOrRegisterOAuth(c.Request.Context(), email, username)
	if err != nil {
		log.Printf("[WeChatScanPoll] Login or register failed: %v", err)
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, WeChatScanPollResponse{
		Status:      "confirmed",
		AccessToken: token,
		TokenType:   "Bearer",
		User:        dto.UserFromService(user),
	})
}

// WeChatMPCallback 微信公众号消息回调
// GET+POST /api/v1/webhook/wechat/mp
func (h *AuthHandler) WeChatMPCallback(c *gin.Context) {
	// GET 请求：微信服务器验证
	if c.Request.Method == "GET" {
		h.handleWeChatMPVerify(c)
		return
	}

	// POST 请求：接收消息和事件
	h.handleWeChatMPEvent(c)
}

// handleWeChatMPVerify 处理微信服务器验证请求
func (h *AuthHandler) handleWeChatMPVerify(c *gin.Context) {
	signature := c.Query("signature")
	timestamp := c.Query("timestamp")
	nonce := c.Query("nonce")
	echostr := c.Query("echostr")

	// 获取配置的 Token
	cfg, err := h.settingSvc.GetWeChatConfig(c.Request.Context())
	if err != nil {
		log.Printf("[WeChatMPVerify] Failed to get config: %v", err)
		c.String(400, "配置错误")
		return
	}

	token := cfg.ServerToken
	if token == "" {
		log.Printf("[WeChatMPVerify] Server token not configured")
		c.String(400, "Token 未配置")
		return
	}

	// 验证签名
	if !verifyWeChatSignature(signature, timestamp, nonce, token) {
		log.Printf("[WeChatMPVerify] Invalid signature")
		c.String(400, "签名验证失败")
		return
	}

	// 返回 echostr 表示验证成功
	c.String(200, echostr)
}

// handleWeChatMPEvent 处理微信公众号事件
func (h *AuthHandler) handleWeChatMPEvent(c *gin.Context) {
	// 检查是否为内部转发请求（来自 wechat-server 的转发不带签名参数）
	forwardedBy := c.GetHeader("X-Forwarded-By")
	isInternalForward := forwardedBy == "wechat-server"

	if !isInternalForward {
		// 直接来自微信的请求，验证签名
		signature := c.Query("signature")
		timestamp := c.Query("timestamp")
		nonce := c.Query("nonce")

		cfg, err := h.settingSvc.GetWeChatConfig(c.Request.Context())
		if err != nil {
			log.Printf("[WeChatMPEvent] Failed to get config: %v", err)
			c.String(200, "success")
			return
		}

		if !verifyWeChatSignature(signature, timestamp, nonce, cfg.ServerToken) {
			log.Printf("[WeChatMPEvent] Invalid signature")
			c.String(200, "success")
			return
		}
	} else {
		log.Printf("[WeChatMPEvent] Accepted internal forward from wechat-server")
	}

	// 读取并解析消息
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		log.Printf("[WeChatMPEvent] Failed to read body: %v", err)
		c.String(200, "success")
		return
	}

	var msg WeChatMPMessage
	if err := xml.Unmarshal(body, &msg); err != nil {
		log.Printf("[WeChatMPEvent] Failed to parse message: %v", err)
		c.String(200, "success")
		return
	}

	log.Printf("[WeChatMPEvent] Received: MsgType=%s, Event=%s, EventKey=%s, FromUser=%s",
		msg.MsgType, msg.Event, msg.EventKey, msg.FromUserName)

	// 处理扫码事件
	if msg.MsgType == "event" && (msg.Event == "SCAN" || msg.Event == "subscribe") {
		h.handleWeChatScanEvent(c, &msg)
		return
	}

	// 其他消息返回 success
	c.String(200, "success")
}

// handleWeChatScanEvent 处理扫码事件
func (h *AuthHandler) handleWeChatScanEvent(c *gin.Context, msg *WeChatMPMessage) {
	if h.scanLoginSvc == nil {
		log.Printf("[WeChatScanEvent] Scan login service not available")
		c.String(200, "success")
		return
	}

	// 提取场景值
	sceneStr := msg.EventKey
	if msg.Event == "subscribe" && strings.HasPrefix(sceneStr, "qrscene_") {
		// subscribe 事件的 EventKey 带有 qrscene_ 前缀
		sceneStr = strings.TrimPrefix(sceneStr, "qrscene_")
	}

	if sceneStr == "" {
		log.Printf("[WeChatScanEvent] Empty scene_str")
		c.String(200, "success")
		return
	}

	openID := msg.FromUserName

	// 将扫码结果写入 Redis
	if err := h.scanLoginSvc.HandleScanEvent(c.Request.Context(), sceneStr, openID); err != nil {
		log.Printf("[WeChatScanEvent] Failed to handle scan event: %v", err)
	} else {
		log.Printf("[WeChatScanEvent] Scan event handled: sceneStr=%s, openID=%s", sceneStr, openID)
	}

	// 返回空字符串表示不回复消息
	c.String(200, "success")
}

// WeChatScanBindPoll 轮询扫码绑定状态（已登录用户）
// GET /api/v1/auth/wechat/scan/bind/poll?scene_id=xxx
func (h *AuthHandler) WeChatScanBindPoll(c *gin.Context) {
	sceneID := c.Query("scene_id")
	if sceneID == "" {
		response.BadRequest(c, "scene_id 不能为空")
		return
	}

	// 获取当前登录用户
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "用户未登录")
		return
	}

	// 检查扫码登录服务是否可用
	if h.scanLoginSvc == nil {
		response.ErrorFrom(c, infraerrors.ServiceUnavailable("SCAN_LOGIN_NOT_AVAILABLE", "扫码登录服务不可用"))
		return
	}

	// 检查扫码结果
	result, err := h.scanLoginSvc.CheckScanResult(c.Request.Context(), sceneID)
	if err != nil {
		log.Printf("[WeChatScanBindPoll] Failed to check scan result: %v", err)
		response.ErrorFrom(c, infraerrors.InternalServer("CHECK_FAILED", "查询扫码状态失败"))
		return
	}

	// 如果还在等待
	if result.Status == "waiting" {
		response.Success(c, gin.H{
			"status": "waiting",
		})
		return
	}

	// 扫码成功，执行绑定逻辑
	openID := result.OpenID

	// 检查该 OpenID 是否已被其他用户绑定
	exists, err := h.userService.ExistsByWeChatOpenID(c.Request.Context(), openID)
	if err != nil {
		log.Printf("[WeChatScanBindPoll] Failed to check wechat openid exists: %v", err)
		response.ErrorFrom(c, infraerrors.InternalServer("INTERNAL_ERROR", "检查绑定状态失败"))
		return
	}
	if exists {
		response.ErrorFrom(c, infraerrors.Conflict("WECHAT_ALREADY_BOUND", "该微信账号已被其他用户绑定"))
		return
	}

	// 保存绑定关系
	if err := h.userService.BindWeChatOpenID(c.Request.Context(), subject.UserID, openID); err != nil {
		log.Printf("[WeChatScanBindPoll] Failed to bind wechat openid: %v", err)
		response.ErrorFrom(c, infraerrors.InternalServer("BIND_FAILED", "绑定失败"))
		return
	}

	// 绑定成功后再消费扫码结果（冲突时不消费，用户可重试）
	if err := h.scanLoginSvc.ConsumeScanResult(c.Request.Context(), sceneID); err != nil {
		log.Printf("[WeChatScanBindPoll] Failed to consume scan result: %v", err)
	}

	log.Printf("[WeChatScanBindPoll] Bind success: userID=%d, openID=%s", subject.UserID, openID)

	response.Success(c, gin.H{
		"status":        "confirmed",
		"wechat_openid": openID,
		"message":       "绑定成功",
	})
}

// verifyWeChatSignature 验证微信签名
func verifyWeChatSignature(signature, timestamp, nonce, token string) bool {
	// 将 token、timestamp、nonce 三个参数进行字典序排序
	strs := []string{token, timestamp, nonce}
	sort.Strings(strs)

	// 拼接成一个字符串进行 sha1 加密
	str := strings.Join(strs, "")
	hash := sha1.New()
	hash.Write([]byte(str))
	hashStr := hex.EncodeToString(hash.Sum(nil))

	// 与 signature 对比
	return hashStr == signature
}
