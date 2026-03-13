package launcher

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"treeshift/internal/model"
)

// Service 负责打开资源管理器、Terminal 和外部 CLI。
//
// 它只关心最终确定的路径和工具配置，不参与 Git 逻辑。
type Service struct{}

// NewService 创建启动服务实例。
func NewService() *Service {
	return &Service{}
}

// OpenExplorer 在资源管理器中打开指定目录。
func (s *Service) OpenExplorer(path string) error {
	command := exec.Command("explorer.exe", filepath.Clean(path))
	return command.Start()
}

// OpenTerminal 在 Windows Terminal 中打开指定目录。
//
// 在 Windows 平台上，该方法会默认通过管理员权限启动终端，
// 以匹配用户习惯的管理员工作流；其他平台则退回到普通启动。
func (s *Service) OpenTerminal(path string) error {
	cleanPath := filepath.Clean(path)
	return openTerminalWithPreferredPrivileges(cleanPath)
}

// LaunchExternalTool 在目标 worktree 目录下启动外部 CLI。
//
// 参数数组中的 {path} 和 {branch} 占位符会在真正启动前替换为实际值。
func (s *Service) LaunchExternalTool(tool model.ExternalTool, worktreePath string, branch string) error {
	if strings.TrimSpace(tool.Command) == "" {
		return fmt.Errorf("工具 %s 的命令路径为空", tool.Name)
	}

	resolvedArgs := buildExternalToolArgs(tool.Args, worktreePath, branch)
	return launchExternalToolWithPreferredPrivileges(tool.Command, resolvedArgs, filepath.Clean(worktreePath))
}

// buildExternalToolArgs 负责替换外部工具参数中的上下文占位符。
//
// 该方法会把设置页中保存的参数模板展开为最终参数列表，
// 以确保 {path} 与 {branch} 始终和当前 worktree 上下文一致。
func buildExternalToolArgs(arguments []string, worktreePath string, branch string) []string {
	resolvedArgs := make([]string, 0, len(arguments))
	for _, argument := range arguments {
		replaced := strings.ReplaceAll(argument, "{path}", worktreePath)
		replaced = strings.ReplaceAll(replaced, "{branch}", branch)
		resolvedArgs = append(resolvedArgs, replaced)
	}

	return resolvedArgs
}
