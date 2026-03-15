package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"treeshift/internal/config"
	"treeshift/internal/git"
	"treeshift/internal/model"
)

// TestSaveSettingsPreservesPendingCleanups 验证设置保存不会覆盖待清理记录。
func TestSaveSettingsPreservesPendingCleanups(t *testing.T) {
	configDir := t.TempDir()
	repository := model.RepositoryBinding{
		ID:               "repo-1",
		DisplayName:      "demo",
		SelectedPath:     `D:\Code\demo`,
		MainWorktreePath: `D:\Code\demo`,
		CommonDir:        `D:\Code\demo\.git`,
	}

	app := newTestApp(t, configDir, model.Settings{
		Repositories: []model.RepositoryBinding{repository},
		PendingCleanups: []model.PendingCleanup{
			{
				RepositoryID: repository.ID,
				Path:         `D:\Code\demo\_worktrees\feature-a`,
				Branch:       "feature-a",
				Head:         "abcdef12",
				LastError:    "目录仍被占用",
			},
		},
	})

	savedSettings, err := app.SaveSettings(model.Settings{
		Repositories: []model.RepositoryBinding{repository},
		ExternalTools: []model.ExternalTool{
			{
				ID:      "tool-codex",
				Name:    "Codex CLI",
				Command: "codex",
				Enabled: true,
			},
		},
	})
	if err != nil {
		t.Fatalf("保存设置失败：%v", err)
	}

	if len(savedSettings.PendingCleanups) != 1 {
		t.Fatalf("待清理记录被意外覆盖：%v", savedSettings.PendingCleanups)
	}
}

// TestReconcilePendingCleanupsLockedRemovesMissingEntries 验证刷新时会清理已不存在的残留目录记录。
func TestReconcilePendingCleanupsLockedRemovesMissingEntries(t *testing.T) {
	configDir := t.TempDir()
	existingPath := filepath.Join(configDir, "existing-cleanup")
	if err := os.MkdirAll(existingPath, 0o755); err != nil {
		t.Fatalf("创建测试目录失败：%v", err)
	}

	app := newTestApp(t, configDir, model.Settings{
		PendingCleanups: []model.PendingCleanup{
			{
				RepositoryID: "repo-1",
				Path:         existingPath,
				Branch:       "feature-a",
			},
			{
				RepositoryID: "repo-1",
				Path:         filepath.Join(configDir, "missing-cleanup"),
				Branch:       "feature-b",
			},
		},
	})

	if err := app.reconcilePendingCleanupsLocked(); err != nil {
		t.Fatalf("清理待处理记录失败：%v", err)
	}

	if len(app.pendingCleanup) != 1 {
		t.Fatalf("待处理记录数量不正确：%d", len(app.pendingCleanup))
	}

	loadedSettings, err := app.configService.Load()
	if err != nil {
		t.Fatalf("重新加载配置失败：%v", err)
	}

	if len(loadedSettings.PendingCleanups) != 1 || loadedSettings.PendingCleanups[0].Path != existingPath {
		t.Fatalf("配置中的待处理记录未被正确清理：%v", loadedSettings.PendingCleanups)
	}
}

// TestRemovePendingCleanupFolderLocked 验证待清理卡片会删除残留目录并清空持久化记录。
func TestRemovePendingCleanupFolderLocked(t *testing.T) {
	configDir := t.TempDir()
	repository := createTestRepository(t)
	leftoverPath := filepath.Join(configDir, "leftover-folder")
	if err := os.MkdirAll(leftoverPath, 0o755); err != nil {
		t.Fatalf("创建残留目录失败：%v", err)
	}

	app := newTestApp(t, configDir, model.Settings{
		Repositories: []model.RepositoryBinding{repository},
		PendingCleanups: []model.PendingCleanup{
			{
				RepositoryID: repository.ID,
				Path:         leftoverPath,
				Branch:       "feature-a",
				Head:         "abcdef12",
				LastError:    "目录仍被占用",
			},
		},
	})

	result, err := app.removePendingCleanupFolderLocked(repository, model.WorktreeInfo{
		Path:   leftoverPath,
		Branch: "feature-a",
		Head:   "abcdef12",
		Status: model.WorktreeStatusPendingCleanup,
	})
	if err != nil {
		t.Fatalf("删除残留目录失败：%v", err)
	}

	if !result.Success {
		t.Fatalf("删除残留目录应成功，实际结果：%+v", result)
	}

	if _, err := os.Stat(leftoverPath); !os.IsNotExist(err) {
		t.Fatalf("残留目录未被删除：%v", err)
	}

	if len(app.pendingCleanup) != 0 {
		t.Fatalf("待清理映射未被清空：%v", app.pendingCleanup)
	}

	loadedSettings, err := app.configService.Load()
	if err != nil {
		t.Fatalf("重新加载配置失败：%v", err)
	}

	if len(loadedSettings.PendingCleanups) != 0 {
		t.Fatalf("配置中的待清理记录未被清空：%v", loadedSettings.PendingCleanups)
	}
}

// TestRemoveMissingWorktreeLocked 验证缺失目录卡片只会移除 Git Worktree 记录。
func TestRemoveMissingWorktreeLocked(t *testing.T) {
	configDir := t.TempDir()
	repository := createTestRepository(t)
	missingWorktreePath := filepath.Join(configDir, "missing-worktree")
	runGitCommand(t, repository.MainWorktreePath, "worktree", "add", "-b", "feature-missing", missingWorktreePath, "HEAD")
	if err := os.RemoveAll(missingWorktreePath); err != nil {
		t.Fatalf("删除测试 worktree 目录失败：%v", err)
	}

	app := newTestApp(t, configDir, model.Settings{
		Repositories: []model.RepositoryBinding{repository},
	})

	result, err := app.removeMissingWorktreeLocked(repository, model.WorktreeInfo{
		Path:   missingWorktreePath,
		Branch: "feature-missing",
		Status: model.WorktreeStatusMissing,
	})
	if err != nil {
		t.Fatalf("移除缺失目录 Worktree 失败：%v", err)
	}

	if !result.Success {
		t.Fatalf("缺失目录 Worktree 应成功移除：%+v", result)
	}

	for _, worktree := range result.View.Worktrees {
		if filepath.Clean(worktree.Path) == filepath.Clean(missingWorktreePath) {
			t.Fatalf("缺失目录的 Worktree 记录仍然存在：%s", worktree.Path)
		}
	}
}

// TestHandleRemoveWorktreeErrorLockedTreatsDeregisteredPathAsRemoved 验证 Git 记录已移除时不会误判为普通 Git 失败。
func TestHandleRemoveWorktreeErrorLockedTreatsDeregisteredPathAsRemoved(t *testing.T) {
	configDir := t.TempDir()
	repository := createTestRepository(t)
	targetPath := filepath.Join(configDir, "partial-remove-worktree")
	runGitCommand(t, repository.MainWorktreePath, "worktree", "add", "-b", "feature-partial", targetPath, "HEAD")
	runGitCommand(t, repository.MainWorktreePath, "worktree", "remove", "--force", targetPath)
	if err := os.MkdirAll(targetPath, 0o755); err != nil {
		t.Fatalf("重建残留目录失败：%v", err)
	}

	app := newTestApp(t, configDir, model.Settings{
		Repositories: []model.RepositoryBinding{repository},
	})

	result, err := app.handleRemoveWorktreeErrorLocked(
		repository,
		model.WorktreeInfo{
			Path:   targetPath,
			Branch: "feature-partial",
			Head:   "abcdef12",
			Status: model.WorktreeStatusNormal,
		},
		targetPath,
		simpleError("error: failed to delete worktree folder"),
	)
	if err != nil {
		t.Fatalf("处理部分成功删除失败：%v", err)
	}

	if !result.Success {
		t.Fatalf("Git 记录已移除时应继续走目录收尾，而不是 git_failed：%+v", result)
	}

	if _, err := os.Stat(targetPath); !os.IsNotExist(err) {
		t.Fatalf("残留目录未被清理：%v", err)
	}
}

// TestAttachDetachedWorktree 验证应用层会把游离 HEAD worktree 附着到新分支并返回最新视图。
func TestAttachDetachedWorktree(t *testing.T) {
	configDir := t.TempDir()
	repository := createTestRepository(t)
	detachedPath := filepath.Join(configDir, "detached-worktree")

	app := newTestApp(t, configDir, model.Settings{
		Repositories: []model.RepositoryBinding{repository},
	})

	createView, err := app.CreateWorktree(model.CreateWorktreeRequest{
		RepositoryID: repository.ID,
		Mode:         "detached",
		SourceBranch: currentBranchName(t, repository.MainWorktreePath),
		TargetPath:   detachedPath,
	})
	if err != nil {
		t.Fatalf("创建 detached worktree 失败：%v", err)
	}

	createdWorktree, found := worktreeByPath(createView.Worktrees, detachedPath)
	if !found {
		t.Fatalf("未找到新创建的 detached worktree：%s", detachedPath)
	}
	if !createdWorktree.IsDetached {
		t.Fatal("新创建的 worktree 应处于 detached 状态")
	}

	attachedView, err := app.AttachDetachedWorktree(model.AttachDetachedWorktreeRequest{
		RepositoryID: repository.ID,
		Path:         detachedPath,
		Mode:         "new",
		BranchName:   "feature-detached-ui",
	})
	if err != nil {
		t.Fatalf("附着 detached worktree 失败：%v", err)
	}

	attachedWorktree, found := worktreeByPath(attachedView.Worktrees, detachedPath)
	if !found {
		t.Fatalf("未找到附着后的 worktree：%s", detachedPath)
	}
	if attachedWorktree.IsDetached {
		t.Fatal("附着后不应仍然是 detached 状态")
	}
	if attachedWorktree.Branch != "feature-detached-ui" {
		t.Fatalf("附着后的分支不正确，got=%s", attachedWorktree.Branch)
	}
}

// newTestApp 创建带临时配置目录的测试应用实例。
func newTestApp(t *testing.T, configDir string, settings model.Settings) *App {
	t.Helper()

	normalizedSettings := config.NormalizeSettings(settings)
	return &App{
		configService: config.NewService("TreeShift", configDir),
		gitService:    git.NewService(),
		settings:      normalizedSettings,
		pendingCleanup: pendingCleanupMapFromSlice(
			normalizedSettings.PendingCleanups,
		),
	}
}

// createTestRepository 创建一个最小可用的 Git 仓库绑定。
func createTestRepository(t *testing.T) model.RepositoryBinding {
	t.Helper()

	repositoryPath := t.TempDir()
	runGitCommand(t, repositoryPath, "init")
	if err := os.WriteFile(filepath.Join(repositoryPath, "README.md"), []byte("demo"), 0o644); err != nil {
		t.Fatalf("写入仓库文件失败：%v", err)
	}
	runGitCommand(t, repositoryPath, "add", "README.md")
	runGitCommandWithOptions(t, repositoryPath, []string{"-c", "user.name=codex", "-c", "user.email=codex@example.com", "commit", "-m", "init"})

	return model.RepositoryBinding{
		ID:               "repo-" + filepath.Base(repositoryPath),
		DisplayName:      filepath.Base(repositoryPath),
		SelectedPath:     repositoryPath,
		MainWorktreePath: repositoryPath,
		CommonDir:        filepath.Join(repositoryPath, ".git"),
	}
}

// currentBranchName 返回测试仓库当前检出的分支名。
func currentBranchName(t *testing.T, repositoryPath string) string {
	t.Helper()

	return strings.TrimSpace(runGitCommandOutput(t, repositoryPath, "branch", "--show-current"))
}

// runGitCommand 执行测试用 Git 命令。
func runGitCommand(t *testing.T, path string, args ...string) {
	t.Helper()
	runGitCommandWithOptions(t, path, args)
}

// runGitCommandOutput 执行测试用 Git 命令并返回标准输出文本。
func runGitCommandOutput(t *testing.T, path string, args ...string) string {
	t.Helper()

	commandArgs := append([]string{"-C", path}, args...)
	command := exec.Command("git", commandArgs...)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("Git 命令失败：git %v\n%s", args, string(output))
	}

	return string(output)
}

// runGitCommandWithOptions 执行测试用 Git 命令并在失败时输出原始错误。
func runGitCommandWithOptions(t *testing.T, path string, args []string) {
	t.Helper()

	commandArgs := append([]string{"-C", path}, args...)
	command := exec.Command("git", commandArgs...)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("Git 命令失败：git %v\n%s", args, string(output))
	}
}

// simpleError 是测试使用的极简错误类型。
type simpleError string

// Error 返回错误文本。
func (e simpleError) Error() string {
	return string(e)
}
