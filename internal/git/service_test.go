package git

import (
	"testing"

	"treeshift/internal/model"
)

// TestParseWorktreePorcelain 验证 porcelain 解析会保留主工作区和缺失目录状态。
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

// simpleError 是测试内使用的极简错误类型。
type simpleError string

// Error 返回错误文本。
func (e simpleError) Error() string {
	return string(e)
}
