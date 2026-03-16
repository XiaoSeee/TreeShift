//go:build windows

package git

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"unsafe"

	"treeshift/internal/model"
)

var getShortPathNameProc = kernel32DLL.NewProc("GetShortPathNameW")

// TestPathKeyMatchesWindowsShortAndLongPath 验证路径键会把短路径和长路径归一到同一结果。
//
// 该测试直接命中 GitHub Actions 上的 Windows 场景：测试框架可能传入 8.3 短路径，
// 但 `git worktree list --porcelain` 返回长路径。如果两者无法归一到同一键，
// 后续 worktree 查找、锁定和删除前校验都会误报“未找到指定 Worktree”。
func TestPathKeyMatchesWindowsShortAndLongPath(t *testing.T) {
	longPath := filepath.Join(t.TempDir(), "Windows Short Path Comparison Directory")
	if err := os.MkdirAll(longPath, 0o755); err != nil {
		t.Fatalf("创建测试目录失败：%v", err)
	}

	shortPath, ok := existingShortPath(longPath)
	if !ok {
		t.Skip("当前文件系统未提供可区分的 8.3 短路径")
	}

	if PathKey(shortPath) != PathKey(longPath) {
		t.Fatalf("短路径和长路径未归一到同一键：short=%s long=%s", shortPath, longPath)
	}
	if !PathsEqual(shortPath, longPath) {
		t.Fatalf("短路径和长路径未被识别为同一目录：short=%s long=%s", shortPath, longPath)
	}
}

// TestFindWorktreeByPathMatchesWindowsShortPathAlias 验证 worktree 查找支持 Windows 路径别名。
//
// worktree 列表通常来自 Git 的长路径输出，而应用层请求路径可能来自测试框架、
// 文件拖拽或系统环境变量的短路径形式。该测试确保 `findWorktreeByPath` 能稳定命中。
func TestFindWorktreeByPathMatchesWindowsShortPathAlias(t *testing.T) {
	longPath := filepath.Join(t.TempDir(), "Windows Worktree Alias Directory")
	if err := os.MkdirAll(longPath, 0o755); err != nil {
		t.Fatalf("创建测试目录失败：%v", err)
	}

	shortPath, ok := existingShortPath(longPath)
	if !ok {
		t.Skip("当前文件系统未提供可区分的 8.3 短路径")
	}

	worktrees := []model.WorktreeInfo{
		{
			Path:   longPath,
			Status: model.WorktreeStatusNormal,
		},
	}

	worktree, found := findWorktreeByPath(worktrees, shortPath)
	if !found {
		t.Fatalf("未通过短路径匹配到 worktree：short=%s long=%s", shortPath, longPath)
	}
	if worktree.Path != longPath {
		t.Fatalf("匹配到的 worktree 路径不正确，got=%s want=%s", worktree.Path, longPath)
	}
}

// existingShortPath 返回已存在目录的 Windows 8.3 短路径。
//
// path 必须已经存在。若当前卷未启用 8.3 名称，或短路径与长路径没有可区分差异，
// 则返回 false，让上层测试以 `Skip` 退出，避免在不支持该特性的环境中误报失败。
func existingShortPath(path string) (string, bool) {
	pathPointer, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return "", false
	}

	bufferSize := len(path) + 1
	if bufferSize < 260 {
		bufferSize = 260
	}

	for {
		buffer := make([]uint16, bufferSize)
		result, _, callErr := getShortPathNameProc.Call(
			uintptr(unsafe.Pointer(pathPointer)),
			uintptr(unsafe.Pointer(&buffer[0])),
			uintptr(len(buffer)),
		)
		if result == 0 {
			if callErr != syscall.Errno(0) {
				return "", false
			}

			return "", false
		}
		if int(result) >= len(buffer) {
			bufferSize = int(result) + 1
			continue
		}

		shortPath := syscall.UTF16ToString(buffer[:result])
		if shortPath == "" || strings.EqualFold(shortPath, path) {
			return "", false
		}

		return shortPath, true
	}
}
