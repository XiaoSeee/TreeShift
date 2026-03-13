package config

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"treeshift/internal/model"
)

// Service 负责读取和写入便携式配置文件。
//
// 配置默认位于可执行文件同级目录；开发模式下如果当前工作目录存在 wails.json，
// 则优先使用当前工作目录，避免调试时写入临时目录。
type Service struct {
	appName string
	baseDir string
}

// NewService 创建配置服务实例。
//
// baseDir 为空时走自动解析逻辑；测试场景可传入固定目录，便于断言落盘位置。
func NewService(appName string, baseDir string) *Service {
	return &Service{
		appName: appName,
		baseDir: baseDir,
	}
}

// Load 从磁盘读取配置文件。
//
// 文件不存在时返回默认配置；JSON 解析失败时同样返回默认配置，并把错误交给上层展示。
func (s *Service) Load() (model.Settings, error) {
	path, err := s.configPath()
	if err != nil {
		return DefaultSettings(), err
	}

	content, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultSettings(), nil
		}

		return DefaultSettings(), err
	}

	settings := DefaultSettings()
	if err := json.Unmarshal(content, &settings); err != nil {
		return DefaultSettings(), fmt.Errorf("解析配置文件失败：%w", err)
	}

	return NormalizeSettings(settings), nil
}

// Save 把配置安全写入磁盘。
//
// 采用“临时文件 + 原子替换”方式，尽量避免异常退出导致最终配置文件损坏。
func (s *Service) Save(settings model.Settings) error {
	path, err := s.configPath()
	if err != nil {
		return err
	}

	normalized := NormalizeSettings(settings)
	content, err := json.MarshalIndent(normalized, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化配置文件失败：%w", err)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("创建配置目录失败：%w", err)
	}

	tempPath := path + ".tmp"
	if err := os.WriteFile(tempPath, content, 0o644); err != nil {
		return fmt.Errorf("写入临时配置文件失败：%w", err)
	}

	if _, err := os.Stat(path); err == nil {
		if err := os.Remove(path); err != nil {
			return fmt.Errorf("替换旧配置文件失败：%w", err)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("检查旧配置文件失败：%w", err)
	}

	if err := os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("替换配置文件失败：%w", err)
	}

	return nil
}

// DefaultSettings 返回全新的默认配置。
//
// 默认预置一个启用状态的 Codex CLI，方便用户首次启动后直接测试外部工具唤起。
func DefaultSettings() model.Settings {
	return model.Settings{
		SchemaVersion:       model.SettingsSchemaVersion,
		Repositories:        []model.RepositoryBinding{},
		DefaultWorktreeRoot: "",
		PendingCleanups:     []model.PendingCleanup{},
		ExternalTools: []model.ExternalTool{
			{
				ID:      "tool-codex",
				Name:    "Codex CLI",
				Command: "codex",
				Args:    []string{},
				Enabled: true,
			},
		},
		UIPreferences: model.UIPreferences{},
	}
}

// NormalizeSettings 对配置结构做归一化处理。
//
// 该方法会补齐 schema 版本、仓库 ID、工具 ID 和空展示名，保证旧配置也能被平滑读取。
func NormalizeSettings(settings model.Settings) model.Settings {
	normalized := settings
	normalized.SchemaVersion = model.SettingsSchemaVersion

	if normalized.Repositories == nil {
		normalized.Repositories = []model.RepositoryBinding{}
	}
	if normalized.PendingCleanups == nil {
		normalized.PendingCleanups = []model.PendingCleanup{}
	}
	if normalized.ExternalTools == nil {
		normalized.ExternalTools = []model.ExternalTool{}
	}

	for index := range normalized.Repositories {
		repository := normalized.Repositories[index]
		if strings.TrimSpace(repository.ID) == "" {
			repository.ID = stableID(repository.CommonDir + repository.MainWorktreePath)
		}
		if strings.TrimSpace(repository.DisplayName) == "" && strings.TrimSpace(repository.MainWorktreePath) != "" {
			repository.DisplayName = filepath.Base(repository.MainWorktreePath)
		}
		repository.SelectedPath = filepath.Clean(strings.TrimSpace(repository.SelectedPath))
		repository.MainWorktreePath = filepath.Clean(strings.TrimSpace(repository.MainWorktreePath))
		repository.CommonDir = filepath.Clean(strings.TrimSpace(repository.CommonDir))
		repository.DefaultWorktreeRoot = strings.TrimSpace(repository.DefaultWorktreeRoot)
		normalized.Repositories[index] = repository
	}

	for index := range normalized.ExternalTools {
		tool := normalized.ExternalTools[index]
		if strings.TrimSpace(tool.ID) == "" {
			tool.ID = stableID(tool.Name + tool.Command)
		}
		tool.Name = strings.TrimSpace(tool.Name)
		tool.Command = strings.TrimSpace(tool.Command)
		if tool.Args == nil {
			tool.Args = []string{}
		}
		normalized.ExternalTools[index] = tool
	}

	cleanups := make([]model.PendingCleanup, 0, len(normalized.PendingCleanups))
	for _, cleanup := range normalized.PendingCleanups {
		cleanPath := filepath.Clean(strings.TrimSpace(cleanup.Path))
		if strings.TrimSpace(cleanup.Path) == "" || cleanPath == "." {
			continue
		}

		cleanups = append(cleanups, model.PendingCleanup{
			RepositoryID: strings.TrimSpace(cleanup.RepositoryID),
			Path:         cleanPath,
			Branch:       strings.TrimSpace(cleanup.Branch),
			Head:         strings.TrimSpace(cleanup.Head),
			LastError:    strings.TrimSpace(cleanup.LastError),
		})
	}
	normalized.PendingCleanups = cleanups

	return normalized
}

// configPath 解析配置文件完整路径。
func (s *Service) configPath() (string, error) {
	baseDir, err := s.resolveBaseDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(baseDir, "config.json"), nil
}

// resolveBaseDir 解析配置基准目录。
func (s *Service) resolveBaseDir() (string, error) {
	if strings.TrimSpace(s.baseDir) != "" {
		return filepath.Clean(s.baseDir), nil
	}

	if cwd, err := os.Getwd(); err == nil {
		if _, statErr := os.Stat(filepath.Join(cwd, "wails.json")); statErr == nil {
			return cwd, nil
		}
	}

	executablePath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("获取可执行文件路径失败：%w", err)
	}

	return filepath.Dir(executablePath), nil
}

// stableID 根据种子值生成稳定短 ID。
//
// 前 12 位十六进制 SHA-1 足以满足本地仓库和工具配置的去重场景。
func stableID(seed string) string {
	sum := sha1.Sum([]byte(seed))
	return hex.EncodeToString(sum[:])[:12]
}
