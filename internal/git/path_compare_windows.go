//go:build windows

package git

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"
)

var (
	kernel32DLL         = syscall.NewLazyDLL("kernel32.dll")
	getLongPathNameProc = kernel32DLL.NewProc("GetLongPathNameW")
)

// resolveExistingPathForComparison 解析 Windows 已存在路径的稳定比较形式。
//
// 该方法只处理当前确实存在的路径：先解析可能的链接，再尝试把 8.3 短路径
// 展开成长路径，并移除扩展路径前缀。若路径不存在，则返回 false，让上层
// 退回到纯字符串比较。
func resolveExistingPathForComparison(path string) (string, bool) {
	cleanPath := filepath.Clean(path)
	if _, err := os.Stat(cleanPath); err != nil {
		return "", false
	}

	if resolvedPath, err := filepath.EvalSymlinks(cleanPath); err == nil {
		cleanPath = resolvedPath
	}
	if longPath, err := getLongPathName(cleanPath); err == nil {
		cleanPath = longPath
	}

	return stripExtendedPathPrefix(filepath.Clean(cleanPath)), true
}

// normalizeComparisonPath 统一 Windows 路径比较时的大小写与前缀形式。
//
// path 应为已清理的文件系统路径；返回值会统一为小写，并去掉 `\\?\` 这类
// 仅用于 Win32 API 的扩展前缀，确保来自 Git、测试和界面的路径能共用同一键。
func normalizeComparisonPath(path string) string {
	return strings.ToLower(stripExtendedPathPrefix(filepath.Clean(path)))
}

// getLongPathName 调用 Win32 API 把已存在路径展开为长路径。
//
// path 必须是当前存在的目录或文件；返回值保留系统给出的规范化结果，
// 供上层继续做 `filepath.Clean` 和大小写归一化处理。
func getLongPathName(path string) (string, error) {
	pathPointer, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return "", err
	}

	bufferSize := len(path) + 1
	if bufferSize < 260 {
		bufferSize = 260
	}

	for {
		buffer := make([]uint16, bufferSize)
		result, _, callErr := getLongPathNameProc.Call(
			uintptr(unsafe.Pointer(pathPointer)),
			uintptr(unsafe.Pointer(&buffer[0])),
			uintptr(len(buffer)),
		)
		if result == 0 {
			if callErr != syscall.Errno(0) {
				return "", callErr
			}

			return "", fmt.Errorf("GetLongPathNameW 返回空结果：%s", path)
		}
		if int(result) >= len(buffer) {
			bufferSize = int(result) + 1
			continue
		}

		return syscall.UTF16ToString(buffer[:result]), nil
	}
}

// stripExtendedPathPrefix 移除 Windows 扩展路径前缀。
//
// Win32 的部分 API 会返回 `\\?\C:\...` 或 `\\?\UNC\server\share` 形式。
// 该方法会把它们还原为常规路径表示，避免这些前缀影响普通路径比较与展示。
func stripExtendedPathPrefix(path string) string {
	if strings.HasPrefix(path, `\\?\UNC\`) {
		return `\\` + strings.TrimPrefix(path, `\\?\UNC\`)
	}
	if strings.HasPrefix(path, `\\?\`) {
		return strings.TrimPrefix(path, `\\?\`)
	}

	return path
}
