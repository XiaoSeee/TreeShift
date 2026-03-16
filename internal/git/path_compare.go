package git

import (
	"os"
	"path/filepath"
	"strings"
)

// PathKey 把 worktree 路径转换为可稳定比较的运行时键。
//
// 该方法会先清理空白和分隔符；如果目标路径当前存在，则会进一步尝试解析
// 符号链接、Windows 8.3 短路径等别名，避免同一目录因展示形式不同而匹配失败。
// 返回值适合用于路径比较和运行态索引，不直接用于界面展示。
func PathKey(path string) string {
	cleanPath, ok := cleanComparablePath(path)
	if !ok {
		return ""
	}

	if resolvedPath, resolved := resolveExistingPathForComparison(cleanPath); resolved {
		cleanPath = resolvedPath
	}

	return normalizeComparisonPath(cleanPath)
}

// PathsEqual 判断两个 worktree 路径是否指向同一物理目录。
//
// 该方法优先比较 PathKey；若两边都存在但归一化后仍不一致，则再退回到
// `os.SameFile` 做文件系统级确认，尽量兼容 Windows 短路径、大小写和链接差异。
func PathsEqual(left string, right string) bool {
	leftPath, leftOK := cleanComparablePath(left)
	rightPath, rightOK := cleanComparablePath(right)
	if !leftOK || !rightOK {
		return leftOK == rightOK
	}

	leftKey := PathKey(leftPath)
	rightKey := PathKey(rightPath)
	if leftKey == rightKey {
		return true
	}

	leftInfo, leftErr := os.Stat(leftPath)
	rightInfo, rightErr := os.Stat(rightPath)
	if leftErr == nil && rightErr == nil {
		return os.SameFile(leftInfo, rightInfo)
	}

	return false
}

// cleanComparablePath 清洗待比较路径并过滤空输入。
//
// path 为用户输入、测试路径或 Git 输出的原始字符串；返回值为去空白后的
// `filepath.Clean` 结果。若输入为空白字符串，则第二个返回值为 false。
func cleanComparablePath(path string) (string, bool) {
	trimmedPath := strings.TrimSpace(path)
	if trimmedPath == "" {
		return "", false
	}

	return filepath.Clean(trimmedPath), true
}
