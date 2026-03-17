package config

import (
	"os"
	"path/filepath"
	"testing"

	"treeshift/internal/model"
)

// TestDefaultSettingsPresetsCodexTool 验证默认配置仍会预置 Codex CLI 模板。
//
// 该模板只用于帮助用户快速配置或调整外部工具，不代表启动时一定会执行环境检查。
func TestDefaultSettingsPresetsCodexTool(t *testing.T) {
	settings := DefaultSettings()

	if len(settings.ExternalTools) != 1 {
		t.Fatalf("默认外部工具数量不正确：%v", settings.ExternalTools)
	}
	if settings.ExternalTools[0].ID != "tool-codex" {
		t.Fatalf("默认工具未保留 Codex 预设：%v", settings.ExternalTools)
	}
	if settings.LaunchScript.PowerShellScript != "" {
		t.Fatalf("默认启动脚本应为空：%q", settings.LaunchScript.PowerShellScript)
	}
}

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
		LaunchScript: model.LaunchScriptSettings{
			PowerShellScript:     `$env:HTTP_PROXY="http://127.0.0.1:6789"`,
			ApplyToTerminal:      true,
			ApplyToExternalTools: true,
		},
		PendingCleanups: []model.PendingCleanup{
			{
				RepositoryID: "repo-1",
				Path:         `D:\Worktrees\demo\feature-a`,
				Branch:       "feature-a",
				Head:         "abcdef12",
				LastError:    "目录仍被占用",
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

	if len(loaded.PendingCleanups) != 1 || loaded.PendingCleanups[0].Path != settings.PendingCleanups[0].Path {
		t.Fatalf("待清理项目未被完整回读：%v", loaded.PendingCleanups)
	}
	if loaded.LaunchScript != settings.LaunchScript {
		t.Fatalf("启动脚本配置未被完整回读：got=%+v want=%+v", loaded.LaunchScript, settings.LaunchScript)
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
		LaunchScript: model.LaunchScriptSettings{
			PowerShellScript:     "  $env:HTTP_PROXY='http://127.0.0.1:6789'  ",
			ApplyToTerminal:      true,
			ApplyToExternalTools: true,
		},
		PendingCleanups: []model.PendingCleanup{
			{
				RepositoryID: "repo-1",
				Path:         ` D:\Code\demo\_worktrees\feature-a\ `,
				Branch:       " feature-a ",
				Head:         " abcdef12 ",
				LastError:    " 被占用 ",
			},
			{
				RepositoryID: "repo-1",
				Path:         "   ",
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

	if len(normalized.PendingCleanups) != 1 {
		t.Fatalf("待清理项目数量不正确：%d", len(normalized.PendingCleanups))
	}

	if normalized.PendingCleanups[0].Path != `D:\Code\demo\_worktrees\feature-a` {
		t.Fatalf("待清理路径未被清洗：%s", normalized.PendingCleanups[0].Path)
	}

	if normalized.LaunchScript.PowerShellScript != `$env:HTTP_PROXY='http://127.0.0.1:6789'` {
		t.Fatalf("启动脚本文本未被清洗：%q", normalized.LaunchScript.PowerShellScript)
	}
}
