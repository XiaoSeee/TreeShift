package config

import (
	"os"
	"path/filepath"
	"testing"

	"treeshift/internal/model"
)

// TestSaveAndLoadSettings 验证配置文件可以成功落盘并被完整回读。
func TestSaveAndLoadSettings(t *testing.T) {
	tempDir := t.TempDir()
	service := NewService("TreeShift", tempDir)

	settings := model.Settings{
		SchemaVersion:       model.SettingsSchemaVersion,
		DefaultWorktreeRoot: `D:\Worktrees`,
		Repositories: []model.RepositoryBinding{
			{
				ID:                  "repo-1",
				DisplayName:         "demo",
				SelectedPath:        `D:\Code\demo`,
				MainWorktreePath:    `D:\Code\demo`,
				CommonDir:           `D:\Code\demo\.git`,
				DefaultWorktreeRoot: `D:\Worktrees\demo`,
			},
		},
		ExternalTools: []model.ExternalTool{
			{
				ID:      "tool-codex",
				Name:    "Codex CLI",
				Command: "codex",
				Args:    []string{"--model", "gpt-5"},
				Enabled: true,
			},
		},
		UIPreferences: model.UIPreferences{
			LastSelectedRepositoryID: "repo-1",
		},
	}

	if err := service.Save(settings); err != nil {
		t.Fatalf("保存配置失败：%v", err)
	}

	loaded, err := service.Load()
	if err != nil {
		t.Fatalf("加载配置失败：%v", err)
	}

	if loaded.DefaultWorktreeRoot != settings.DefaultWorktreeRoot {
		t.Fatalf("默认路径不匹配，want=%s got=%s", settings.DefaultWorktreeRoot, loaded.DefaultWorktreeRoot)
	}

	if _, err := os.Stat(filepath.Join(tempDir, "config.json")); err != nil {
		t.Fatalf("配置文件未落盘：%v", err)
	}
}

// TestNormalizeSettings 验证归一化逻辑会自动补齐缺失字段。
func TestNormalizeSettings(t *testing.T) {
	settings := model.Settings{
		Repositories: []model.RepositoryBinding{
			{
				DisplayName:      "demo",
				MainWorktreePath: `D:\Code\demo`,
				CommonDir:        `D:\Code\demo\.git`,
			},
		},
		ExternalTools: []model.ExternalTool{
			{
				Name:    "Codex CLI",
				Command: "codex",
			},
		},
	}

	normalized := NormalizeSettings(settings)
	if normalized.SchemaVersion != model.SettingsSchemaVersion {
		t.Fatalf("配置版本未被补齐：%d", normalized.SchemaVersion)
	}

	if normalized.Repositories[0].ID == "" {
		t.Fatal("仓库 ID 未自动生成")
	}

	if normalized.ExternalTools[0].ID == "" {
		t.Fatal("工具 ID 未自动生成")
	}
}
