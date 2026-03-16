//go:build !windows

package git

import "path/filepath"

// resolveExistingPathForComparison 解析非 Windows 平台上已存在路径的稳定比较形式。
//
// 该方法会尝试解析符号链接，以便让不同入口路径在比较时落到同一物理目录。
// 若解析失败，通常意味着路径不存在或平台无法进一步规范化，此时返回 false。
func resolveExistingPathForComparison(path string) (string, bool) {
	resolvedPath, err := filepath.EvalSymlinks(filepath.Clean(path))
	if err != nil {
		return "", false
	}

	return filepath.Clean(resolvedPath), true
}

// normalizeComparisonPath 返回非 Windows 平台的标准比较路径。
//
// 由于这些平台通常区分大小写，因此这里只做 `filepath.Clean`，不额外变更大小写。
func normalizeComparisonPath(path string) string {
	return filepath.Clean(path)
}
