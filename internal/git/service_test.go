package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"treeshift/internal/model"
)

// TestParseWorktreePorcelain 验证 porcelain 解析会保留主工作区与缺失目录状态。
func TestParseWorktreePorcelain(t *testing.T) {
	output := `worktree D:\Code\demo
HEAD abcdef1234567890
branch refs/heads/main

worktree D:\Code\_worktrees\feature-login
HEAD 0123456789abcdef
branch refs/heads/feature/login

worktree D:\Code\_worktrees\missing
HEAD fedcba9876543210
branch refs/heads/feature/missing
prunable gitdir file points to non-existent location
`

	worktrees := ParseWorktreePorcelain(output)
	if len(worktrees) != 3 {
		t.Fatalf("解析数量不正确，want=3 got=%d", len(worktrees))
	}
	if !worktrees[0].IsMain {
		t.Fatal("首个 worktree 应被标记为主工作区")
	}
	if worktrees[1].Branch != "feature/login" {
		t.Fatalf("分支名未被正确缩短，got=%s", worktrees[1].Branch)
	}
	if worktrees[2].Status != model.WorktreeStatusMissing {
		t.Fatalf("缺失目录状态不正确，got=%s", worktrees[2].Status)
	}
}

// TestParseWorktreePorcelainMarksDetached 验证 porcelain 解析会正确识别 detached HEAD。
func TestParseWorktreePorcelainMarksDetached(t *testing.T) {
	output := `worktree D:\Code\demo
HEAD abcdef1234567890
branch refs/heads/main

worktree D:\Code\_worktrees\detached
HEAD 0123456789abcdef
detached
`

	worktrees := ParseWorktreePorcelain(output)
	if len(worktrees) != 2 {
		t.Fatalf("解析数量不正确，want=2 got=%d", len(worktrees))
	}
	if !worktrees[1].IsDetached {
		t.Fatal("detached worktree 未被标记为游离 HEAD")
	}
	if worktrees[1].Branch != "detached" {
		t.Fatalf("游离 HEAD worktree 展示分支不正确，got=%s", worktrees[1].Branch)
	}
}

// TestParseWorktreePorcelainMarksLocked 验证 porcelain 解析会保留 locked 标记和锁定原因。
func TestParseWorktreePorcelainMarksLocked(t *testing.T) {
	rootPath := filepath.Join(t.TempDir(), "repo")
	lockedPath := filepath.Join(rootPath, "_worktrees", "feature-locked")
	if err := os.MkdirAll(lockedPath, 0o755); err != nil {
		t.Fatalf("创建测试 worktree 目录失败：%v", err)
	}

	output := "worktree " + rootPath + "\n" +
		"HEAD abcdef1234567890\n" +
		"branch refs/heads/main\n\n" +
		"worktree " + lockedPath + "\n" +
		"HEAD 0123456789abcdef\n" +
		"branch refs/heads/feature/locked\n" +
		"locked usb offline\n"

	worktrees := ParseWorktreePorcelain(output)
	if len(worktrees) != 2 {
		t.Fatalf("解析数量不正确，want=2 got=%d", len(worktrees))
	}
	if !worktrees[1].IsLocked {
		t.Fatal("locked worktree 未被标记为已锁定")
	}
	if worktrees[1].LockReason != "usb offline" {
		t.Fatalf("锁定原因解析不正确，got=%s", worktrees[1].LockReason)
	}
	if worktrees[1].Status != model.WorktreeStatusNormal {
		t.Fatalf("已锁定且目录存在的 worktree 状态不正确，got=%s", worktrees[1].Status)
	}
}

// TestParseWorktreePorcelainKeepsLockedMissing 验证目录缺失时仍会保留锁定标记。
func TestParseWorktreePorcelainKeepsLockedMissing(t *testing.T) {
	rootPath := filepath.Join(t.TempDir(), "repo")
	missingPath := filepath.Join(rootPath, "_worktrees", "feature-missing")
	if err := os.MkdirAll(rootPath, 0o755); err != nil {
		t.Fatalf("创建测试仓库目录失败：%v", err)
	}

	output := "worktree " + rootPath + "\n" +
		"HEAD abcdef1234567890\n" +
		"branch refs/heads/main\n\n" +
		"worktree " + missingPath + "\n" +
		"HEAD 0123456789abcdef\n" +
		"branch refs/heads/feature/missing\n" +
		"locked usb offline\n"

	worktrees := ParseWorktreePorcelain(output)
	if len(worktrees) != 2 {
		t.Fatalf("解析数量不正确，want=2 got=%d", len(worktrees))
	}
	if !worktrees[1].IsLocked {
		t.Fatal("目录缺失的 locked worktree 未保留锁定标记")
	}
	if worktrees[1].Status != model.WorktreeStatusMissing {
		t.Fatalf("目录缺失的 locked worktree 状态不正确，got=%s", worktrees[1].Status)
	}
}

// TestIsDirtyRemoveError 验证脏目录错误会被正确识别。
func TestIsDirtyRemoveError(t *testing.T) {
	if !IsDirtyRemoveError(simpleError("fatal: 'D:/Code/demo' contains modified or untracked files, use --force to delete it")) {
		t.Fatal("脏目录错误未被识别")
	}
	if IsDirtyRemoveError(simpleError("fatal: not a git repository")) {
		t.Fatal("普通 Git 错误不应被误判为脏目录")
	}
}

// TestParseWorktreeChangeSummary 验证文件改动摘要会被正确归类。
func TestParseWorktreeChangeSummary(t *testing.T) {
	output := ` M internal/git/service.go
M  frontend/src/App.tsx
?? frontend/src/components/NewCard.tsx
D  docs/old-plan.md
RD legacy/config.json
`

	summary := ParseWorktreeChangeSummary(output)
	if summary.ChangedCount != 3 {
		t.Fatalf("正向改动数量不正确，want=3 got=%d", summary.ChangedCount)
	}
	if summary.DeletedCount != 2 {
		t.Fatalf("删除改动数量不正确，want=2 got=%d", summary.DeletedCount)
	}
}

// TestCreateWorktreeDetached 验证 detached 模式会创建游离 HEAD worktree。
func TestCreateWorktreeDetached(t *testing.T) {
	service := NewService()
	repository := createTestRepository(t)
	targetPath := filepath.Join(t.TempDir(), "detached-worktree")

	if err := service.CreateWorktree(repository, model.CreateWorktreeRequest{
		Mode:         "detached",
		SourceBranch: currentBranchName(t, repository.MainWorktreePath),
		TargetPath:   targetPath,
	}); err != nil {
		t.Fatalf("创建 detached worktree 失败：%v", err)
	}

	worktrees, err := service.ListWorktrees(repository)
	if err != nil {
		t.Fatalf("读取 worktree 列表失败：%v", err)
	}

	targetWorktree, found := findWorktreeByPath(worktrees, targetPath)
	if !found {
		t.Fatalf("未找到新创建的 detached worktree：%s", targetPath)
	}
	if !targetWorktree.IsDetached {
		t.Fatal("新创建的 worktree 应处于 detached 状态")
	}
}

// TestAttachDetachedWorktreeCreatesBranch 验证游离 HEAD 可以创建并切换到新分支。
func TestAttachDetachedWorktreeCreatesBranch(t *testing.T) {
	service := NewService()
	repository := createTestRepository(t)
	targetPath := filepath.Join(t.TempDir(), "detached-worktree")

	if err := service.CreateWorktree(repository, model.CreateWorktreeRequest{
		Mode:         "detached",
		SourceBranch: currentBranchName(t, repository.MainWorktreePath),
		TargetPath:   targetPath,
	}); err != nil {
		t.Fatalf("创建 detached worktree 失败：%v", err)
	}

	if err := service.AttachDetachedWorktree(repository, model.AttachDetachedWorktreeRequest{
		Path:       targetPath,
		Mode:       "new",
		BranchName: "feature-detached",
	}); err != nil {
		t.Fatalf("游离 HEAD 创建新分支失败：%v", err)
	}

	worktrees, err := service.ListWorktrees(repository)
	if err != nil {
		t.Fatalf("读取 worktree 列表失败：%v", err)
	}

	targetWorktree, found := findWorktreeByPath(worktrees, targetPath)
	if !found {
		t.Fatalf("未找到目标 worktree：%s", targetPath)
	}
	if targetWorktree.IsDetached {
		t.Fatal("附着新分支后不应仍处于 detached 状态")
	}
	if targetWorktree.Branch != "feature-detached" {
		t.Fatalf("附着后的分支不正确，got=%s", targetWorktree.Branch)
	}
}

// TestAttachDetachedWorktreeSwitchesToFreeBranch 验证游离 HEAD 可以切换到未占用的现有分支。
func TestAttachDetachedWorktreeSwitchesToFreeBranch(t *testing.T) {
	service := NewService()
	repository := createTestRepository(t)
	targetPath := filepath.Join(t.TempDir(), "detached-worktree")
	runGitCommand(t, repository.MainWorktreePath, "branch", "feature-free")

	if err := service.CreateWorktree(repository, model.CreateWorktreeRequest{
		Mode:         "detached",
		SourceBranch: currentBranchName(t, repository.MainWorktreePath),
		TargetPath:   targetPath,
	}); err != nil {
		t.Fatalf("创建 detached worktree 失败：%v", err)
	}

	if err := service.AttachDetachedWorktree(repository, model.AttachDetachedWorktreeRequest{
		Path:       targetPath,
		Mode:       "existing",
		BranchName: "feature-free",
	}); err != nil {
		t.Fatalf("切换到空闲现有分支失败：%v", err)
	}

	worktrees, err := service.ListWorktrees(repository)
	if err != nil {
		t.Fatalf("读取 worktree 列表失败：%v", err)
	}

	targetWorktree, found := findWorktreeByPath(worktrees, targetPath)
	if !found {
		t.Fatalf("未找到目标 worktree：%s", targetPath)
	}
	if targetWorktree.IsDetached {
		t.Fatal("切换到现有分支后不应仍处于 detached 状态")
	}
	if targetWorktree.Branch != "feature-free" {
		t.Fatalf("切换后的分支不正确，got=%s", targetWorktree.Branch)
	}
}

// TestAttachDetachedWorktreeRejectsOccupiedBranch 验证游离 HEAD 切换到已占用分支时会被 Git 拒绝。
func TestAttachDetachedWorktreeRejectsOccupiedBranch(t *testing.T) {
	service := NewService()
	repository := createTestRepository(t)
	detachedPath := filepath.Join(t.TempDir(), "detached-worktree")
	busyPath := filepath.Join(t.TempDir(), "busy-worktree")
	runGitCommand(t, repository.MainWorktreePath, "branch", "feature-busy")
	runGitCommand(t, repository.MainWorktreePath, "worktree", "add", busyPath, "feature-busy")

	if err := service.CreateWorktree(repository, model.CreateWorktreeRequest{
		Mode:         "detached",
		SourceBranch: currentBranchName(t, repository.MainWorktreePath),
		TargetPath:   detachedPath,
	}); err != nil {
		t.Fatalf("创建 detached worktree 失败：%v", err)
	}

	err := service.AttachDetachedWorktree(repository, model.AttachDetachedWorktreeRequest{
		Path:       detachedPath,
		Mode:       "existing",
		BranchName: "feature-busy",
	})
	if err == nil {
		t.Fatal("切换到已占用分支时应返回错误")
	}
	if !strings.Contains(err.Error(), "already used by worktree") {
		t.Fatalf("错误内容未体现分支已被占用，got=%v", err)
	}
}

// TestSetWorktreeLock 验证 linked worktree 可以在锁定和解锁之间切换。
func TestSetWorktreeLock(t *testing.T) {
	service := NewService()
	repository := createTestRepository(t)
	targetPath := filepath.Join(t.TempDir(), "locked-worktree")

	runGitCommand(t, repository.MainWorktreePath, "branch", "feature-locked")
	runGitCommand(t, repository.MainWorktreePath, "worktree", "add", targetPath, "feature-locked")

	if err := service.SetWorktreeLock(repository, targetPath, true); err != nil {
		t.Fatalf("锁定 linked worktree 失败：%v", err)
	}

	worktrees, err := service.ListWorktrees(repository)
	if err != nil {
		t.Fatalf("读取锁定后的 worktree 列表失败：%v", err)
	}

	lockedWorktree, found := findWorktreeByPath(worktrees, targetPath)
	if !found {
		t.Fatalf("未找到被锁定的 worktree：%s", targetPath)
	}
	if !lockedWorktree.IsLocked {
		t.Fatal("锁定后 worktree 未被标记为已锁定")
	}

	if err := service.SetWorktreeLock(repository, targetPath, false); err != nil {
		t.Fatalf("解锁 linked worktree 失败：%v", err)
	}

	worktrees, err = service.ListWorktrees(repository)
	if err != nil {
		t.Fatalf("读取解锁后的 worktree 列表失败：%v", err)
	}

	unlockedWorktree, found := findWorktreeByPath(worktrees, targetPath)
	if !found {
		t.Fatalf("未找到被解锁的 worktree：%s", targetPath)
	}
	if unlockedWorktree.IsLocked {
		t.Fatal("解锁后 worktree 仍被标记为已锁定")
	}
}

// simpleError 是测试内使用的极简错误类型。
type simpleError string

// Error 返回错误文本。
func (e simpleError) Error() string {
	return string(e)
}

// createTestRepository 创建一个最小可用的 Git 仓库绑定。
//
// 测试仓库会完成一次初始提交，确保后续 worktree 与 switch 命令可正常执行。
func createTestRepository(t *testing.T) model.RepositoryBinding {
	t.Helper()

	repositoryPath := t.TempDir()
	runGitCommand(t, repositoryPath, "init")
	if err := os.WriteFile(filepath.Join(repositoryPath, "README.md"), []byte("demo"), 0o644); err != nil {
		t.Fatalf("写入测试文件失败：%v", err)
	}
	runGitCommand(t, repositoryPath, "add", "README.md")
	runGitCommand(t, repositoryPath, "-c", "user.name=codex", "-c", "user.email=codex@example.com", "commit", "-m", "init")

	return model.RepositoryBinding{
		ID:               "repo-" + filepath.Base(repositoryPath),
		DisplayName:      filepath.Base(repositoryPath),
		SelectedPath:     repositoryPath,
		MainWorktreePath: repositoryPath,
		CommonDir:        filepath.Join(repositoryPath, ".git"),
	}
}

// currentBranchName 返回测试仓库当前主工作区检出的分支名。
func currentBranchName(t *testing.T, repositoryPath string) string {
	t.Helper()

	return strings.TrimSpace(runGitCommand(t, repositoryPath, "branch", "--show-current"))
}

// runGitCommand 执行测试用 Git 命令，并在失败时输出原始错误。
//
// 该辅助方法返回标准输出文本，便于测试直接读取当前分支等单值结果。
func runGitCommand(t *testing.T, path string, args ...string) string {
	t.Helper()

	commandArgs := append([]string{"-C", path}, args...)
	command := exec.Command("git", commandArgs...)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("Git 命令失败：git %v\n%s", args, string(output))
	}

	return strings.TrimSpace(string(output))
}
