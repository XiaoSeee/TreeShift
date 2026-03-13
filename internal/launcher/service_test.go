package launcher

import (
	"reflect"
	"testing"
)

// TestBuildExternalToolArgs 会验证外部工具参数模板中的占位符替换逻辑。
//
// 该测试覆盖 {path} 与 {branch} 两个核心占位符，
// 以确保 AI CLI 总是在当前 worktree 上下文中启动。
func TestBuildExternalToolArgs(t *testing.T) {
	arguments := []string{
		"--cwd",
		"{path}",
		"--branch",
		"{branch}",
		"literal",
	}

	resolvedArgs := buildExternalToolArgs(arguments, `D:\Code\Repo\feature-a`, "feature-a")
	expectedArgs := []string{
		"--cwd",
		`D:\Code\Repo\feature-a`,
		"--branch",
		"feature-a",
		"literal",
	}

	if !reflect.DeepEqual(resolvedArgs, expectedArgs) {
		t.Fatalf("参数替换结果不符合预期：got=%v want=%v", resolvedArgs, expectedArgs)
	}
}
