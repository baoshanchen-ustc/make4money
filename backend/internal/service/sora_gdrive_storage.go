package service

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/option"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
)

// SoraGDriveStorage 负责 Sora 媒体文件的 Google Drive 存储操作。
type SoraGDriveStorage struct {
	settingService *SettingService

	mu              sync.RWMutex
	srv             *drive.Service
	cfg             *SoraS3Profile // 缓存当前 GDrive 配置
	healthCheckedAt time.Time
	healthErr       error
	healthTTL       time.Duration
}

const defaultGDriveHealthTTL = 30 * time.Second

// NewSoraGDriveStorage 创建 Google Drive 存储服务实例。
func NewSoraGDriveStorage(settingService *SettingService) *SoraGDriveStorage {
	return &SoraGDriveStorage{
		settingService: settingService,
		healthTTL:      defaultGDriveHealthTTL,
	}
}

// StorageType 返回存储类型标识。
func (s *SoraGDriveStorage) StorageType() string {
	return SoraStorageTypeGDrive
}

// Enabled 返回 Google Drive 存储是否已启用。
func (s *SoraGDriveStorage) Enabled(ctx context.Context) bool {
	profile := s.getActiveGDriveProfile(ctx)
	if profile == nil {
		return false
	}
	return profile.Enabled && s.hasValidCredentials(profile)
}

// getActiveGDriveProfile 获取当前激活的 GDrive 配置。
func (s *SoraGDriveStorage) getActiveGDriveProfile(ctx context.Context) *SoraS3Profile {
	if s.settingService == nil {
		return nil
	}
	profile, err := s.settingService.GetActiveStorageProfile(ctx)
	if err != nil || profile == nil {
		return nil
	}
	if profile.GetProvider() != SoraStorageTypeGDrive {
		return nil
	}
	return profile
}

// hasValidCredentials 检查 GDrive 配置是否有有效凭证。
func (s *SoraGDriveStorage) hasValidCredentials(profile *SoraS3Profile) bool {
	switch profile.AuthType {
	case "oauth2":
		return profile.ClientID != "" && profile.ClientSecret != "" && profile.RefreshToken != ""
	case "service_account":
		return profile.ServiceAccountJSON != ""
	default:
		return false
	}
}

// getService 获取或初始化 Drive 服务（带缓存）。
func (s *SoraGDriveStorage) getService(ctx context.Context) (*drive.Service, *SoraS3Profile, error) {
	s.mu.RLock()
	if s.srv != nil && s.cfg != nil {
		srv, cfg := s.srv, s.cfg
		s.mu.RUnlock()
		return srv, cfg, nil
	}
	s.mu.RUnlock()

	return s.initService(ctx)
}

func (s *SoraGDriveStorage) initService(ctx context.Context) (*drive.Service, *SoraS3Profile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 双重检查
	if s.srv != nil && s.cfg != nil {
		return s.srv, s.cfg, nil
	}

	profile := s.getActiveGDriveProfile(ctx)
	if profile == nil {
		return nil, nil, fmt.Errorf("no active gdrive profile found")
	}
	if !profile.Enabled {
		return nil, nil, fmt.Errorf("gdrive storage is disabled")
	}

	srv, err := s.buildDriveService(ctx, profile)
	if err != nil {
		return nil, nil, fmt.Errorf("build gdrive service: %w", err)
	}

	s.srv = srv
	s.cfg = profile
	logger.LegacyPrintf("service.sora_gdrive", "[SoraGDrive] 客户端已初始化 auth_type=%s folder_id=%s", profile.AuthType, profile.FolderID)
	return srv, profile, nil
}

// buildDriveService 根据认证类型创建 Google Drive 服务。
func (s *SoraGDriveStorage) buildDriveService(ctx context.Context, profile *SoraS3Profile) (*drive.Service, error) {
	switch profile.AuthType {
	case "oauth2":
		return s.buildOAuth2Service(ctx, profile)
	case "service_account":
		return s.buildServiceAccountService(ctx, profile)
	default:
		return nil, fmt.Errorf("unsupported auth_type: %s", profile.AuthType)
	}
}

func (s *SoraGDriveStorage) buildOAuth2Service(ctx context.Context, profile *SoraS3Profile) (*drive.Service, error) {
	config := &oauth2.Config{
		ClientID:     profile.ClientID,
		ClientSecret: profile.ClientSecret,
		Endpoint:     google.Endpoint,
		Scopes:       []string{drive.DriveFileScope},
	}
	token := &oauth2.Token{
		RefreshToken: profile.RefreshToken,
	}
	tokenSource := config.TokenSource(ctx, token)
	srv, err := drive.NewService(ctx, option.WithTokenSource(tokenSource))
	if err != nil {
		return nil, fmt.Errorf("create gdrive oauth2 service: %w", err)
	}
	return srv, nil
}

func (s *SoraGDriveStorage) buildServiceAccountService(ctx context.Context, profile *SoraS3Profile) (*drive.Service, error) {
	creds, err := google.CredentialsFromJSON(ctx, []byte(profile.ServiceAccountJSON), drive.DriveFileScope)
	if err != nil {
		return nil, fmt.Errorf("parse service account json: %w", err)
	}
	srv, err := drive.NewService(ctx, option.WithCredentials(creds))
	if err != nil {
		return nil, fmt.Errorf("create gdrive service account service: %w", err)
	}
	return srv, nil
}

// RefreshClient 清除缓存的 Drive 客户端。
func (s *SoraGDriveStorage) RefreshClient() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.srv = nil
	s.cfg = nil
	s.healthCheckedAt = time.Time{}
	s.healthErr = nil
	logger.LegacyPrintf("service.sora_gdrive", "[SoraGDrive] 客户端缓存已清除")
}

// TestConnection 测试 Google Drive 连接。
func (s *SoraGDriveStorage) TestConnection(ctx context.Context) error {
	srv, _, err := s.getService(ctx)
	if err != nil {
		return err
	}
	_, err = srv.About.Get().Fields("storageQuota").Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("gdrive About.Get failed: %w", err)
	}
	return nil
}

// IsHealthy 返回 Google Drive 健康状态（带短缓存）。
func (s *SoraGDriveStorage) IsHealthy(ctx context.Context) bool {
	if s == nil {
		return false
	}
	now := time.Now()
	s.mu.RLock()
	lastCheck := s.healthCheckedAt
	lastErr := s.healthErr
	ttl := s.healthTTL
	s.mu.RUnlock()

	if ttl <= 0 {
		ttl = defaultGDriveHealthTTL
	}
	if !lastCheck.IsZero() && now.Sub(lastCheck) < ttl {
		return lastErr == nil
	}

	err := s.TestConnection(ctx)
	s.mu.Lock()
	s.healthCheckedAt = time.Now()
	s.healthErr = err
	s.mu.Unlock()
	return err == nil
}

// UploadFromURL 从上游 URL 下载并上传到 Google Drive。
// 返回 Google Drive 文件 ID 作为 objectKey。
func (s *SoraGDriveStorage) UploadFromURL(ctx context.Context, userID int64, sourceURL string) (string, int64, error) {
	srv, cfg, err := s.getService(ctx)
	if err != nil {
		return "", 0, err
	}

	// 下载源文件
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sourceURL, nil)
	if err != nil {
		return "", 0, fmt.Errorf("create download request: %w", err)
	}
	httpClient := &http.Client{Timeout: 5 * time.Minute}
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", 0, fmt.Errorf("download from upstream: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return "", 0, &UpstreamDownloadError{StatusCode: resp.StatusCode}
	}

	// 推断文件扩展名和 MIME
	ext := fileExtFromURL(sourceURL)
	if ext == "" {
		ext = fileExtFromContentType(resp.Header.Get("Content-Type"))
	}
	if ext == "" {
		ext = ".bin"
	}
	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	// 生成文件名
	datePath := time.Now().Format("2006-01-02")
	fileName := fmt.Sprintf("sora_%d_%s_%s%s", userID, datePath, uuid.NewString()[:8], ext)

	// 创建文件元数据
	fileMeta := &drive.File{
		Name:     fileName,
		MimeType: contentType,
	}
	if cfg.FolderID != "" {
		fileMeta.Parents = []string{cfg.FolderID}
	}

	// 使用 CountingReader 统计大小
	cr := &countingReader{Reader: resp.Body}

	// 上传到 Google Drive
	created, err := srv.Files.Create(fileMeta).
		Media(cr).
		Fields("id, size").
		Context(ctx).
		Do()
	if err != nil {
		return "", 0, fmt.Errorf("gdrive upload: %w", err)
	}

	fileSize := cr.BytesRead
	if created.Size > 0 {
		fileSize = created.Size
	}

	// 根据 access_mode 设置权限
	if cfg.AccessMode == "" || cfg.AccessMode == "direct" {
		// 设为任何人可读
		_, permErr := srv.Permissions.Create(created.Id, &drive.Permission{
			Type: "anyone",
			Role: "reader",
		}).Context(ctx).Do()
		if permErr != nil {
			logger.LegacyPrintf("service.sora_gdrive", "[SoraGDrive] 设置公开权限失败 fileID=%s err=%v", created.Id, permErr)
		}
	}

	logger.LegacyPrintf("service.sora_gdrive", "[SoraGDrive] 上传完成 fileID=%s size=%d", created.Id, fileSize)
	return created.Id, fileSize, nil
}

// DeleteObjects 删除一组 Google Drive 文件。
func (s *SoraGDriveStorage) DeleteObjects(ctx context.Context, objectKeys []string) error {
	if len(objectKeys) == 0 {
		return nil
	}

	srv, _, err := s.getService(ctx)
	if err != nil {
		return err
	}

	var lastErr error
	for _, fileID := range objectKeys {
		if err := srv.Files.Delete(fileID).Context(ctx).Do(); err != nil {
			logger.LegacyPrintf("service.sora_gdrive", "[SoraGDrive] 删除失败 fileID=%s err=%v", fileID, err)
			lastErr = err
		}
	}
	return lastErr
}

// GetAccessURL 获取 Google Drive 文件的访问 URL。
func (s *SoraGDriveStorage) GetAccessURL(ctx context.Context, objectKey string) (string, error) {
	_, cfg, err := s.getService(ctx)
	if err != nil {
		return "", err
	}

	// CDN URL 优先
	if cfg.CDNURL != "" {
		cdnBase := strings.TrimRight(cfg.CDNURL, "/")
		return cdnBase + "/" + objectKey, nil
	}

	// 默认使用 Google Drive 直链
	return fmt.Sprintf("https://drive.google.com/uc?export=download&id=%s", objectKey), nil
}

// countingReader 包装 io.Reader 以统计读取的字节数。
type countingReader struct {
	Reader    io.Reader
	BytesRead int64
}

func (r *countingReader) Read(p []byte) (int, error) {
	n, err := r.Reader.Read(p)
	r.BytesRead += int64(n)
	return n, err
}
