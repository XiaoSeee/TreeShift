package environment

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"treeshift/internal/executil"
	"treeshift/internal/model"
)

// Service 负责检查外部依赖是否可用。
//
// 它不会修改任何配置，只返回诊断结果供前端展示。
type Service struct{}

// NewService 创建环境检查服务。
func NewService() *Service {
	return &Service{}
}

// Check 扫描 Git、Windows Terminal 与用户配置的外部工具。
//
// Git 会直接执行版本命令；Windows Terminal 仅解析可执行文件路径，避免检查时误开窗口。
func (s *Service) Check(settings model.Settings, warnings []string) model.EnvironmentStatus {
	status := model.EnvironmentStatus{
		Git:           s.checkGit(),
		Terminal:      s.checkTerminal(),
		ExternalTools: make([]model.ToolStatus, 0, len(settings.ExternalTools)),
		Warnings:      append([]string{}, warnings...),
	}

	for _, tool := range settings.ExternalTools {
		status.ExternalTools = append(status.ExternalTools, s.checkExternalTool(tool))
	}

	return status
}

// checkGit 检查 Git 是否可被执行。
func (s *Service) checkGit() model.ToolStatus {
	commandPath, err := exec.LookPath("git")
	if err != nil {
		return model.ToolStatus{
			Name:      "Git",
			Available: false,
			Message:   "未在 PATH 中找到 git.exe。",
		}
	}

	command := exec.Command(commandPath, "--version")
	executil.PrepareBackgroundCommand(command)

	output, versionErr := command.CombinedOutput()
	if versionErr != nil {
		return model.ToolStatus{
			Name:       "Git",
			Executable: commandPath,
			Available:  false,
			Message:    fmt.Sprintf("Git 已找到，但无法执行版本检查：%v", versionErr),
		}
	}

	return model.ToolStatus{
		Name:       "Git",
		Executable: commandPath,
		Available:  true,
		Message:    strings.TrimSpace(string(output)),
	}
}

// checkTerminal 检查 Windows Terminal 是否能被系统解析。
func (s *Service) checkTerminal() model.ToolStatus {
	commandPath, err := exec.LookPath("wt.exe")
	if err != nil {
		return model.ToolStatus{
			Name:      "Windows Terminal",
			Available: false,
			Message:   "未在 PATH 中找到 wt.exe。",
		}
	}

	return model.ToolStatus{
		Name:       "Windows Terminal",
		Executable: commandPath,
		Available:  true,
		Message:    "已解析到 wt.exe，运行时会优先复用现有窗口。",
	}
}

// checkExternalTool 检查用户自定义工具。
func (s *Service) checkExternalTool(tool model.ExternalTool) model.ToolStatus {
	name := tool.Name
	if strings.TrimSpace(name) == "" {
		name = "未命名工具"
	}

	command := strings.TrimSpace(tool.Command)
	if command == "" {
		return model.ToolStatus{
			Name:      name,
			Available: false,
			Message:   "命令路径为空。",
		}
	}

	if strings.Contains(command, `\`) || strings.Contains(command, `/`) {
		cleanPath := filepath.Clean(command)
		if _, err := os.Stat(cleanPath); err != nil {
			return model.ToolStatus{
				Name:       name,
				Executable: cleanPath,
				Available:  false,
				Message:    fmt.Sprintf("指定路径不存在：%v", err),
			}
		}

		return model.ToolStatus{
			Name:       name,
			Executable: cleanPath,
			Available:  true,
			Message:    "已找到指定可执行文件。",
		}
	}

	commandPath, err := exec.LookPath(command)
	if err != nil {
		return model.ToolStatus{
			Name:      name,
			Available: false,
			Message:   fmt.Sprintf("未在 PATH 中找到 %s。", command),
		}
	}

	return model.ToolStatus{
		Name:       name,
		Executable: commandPath,
		Available:  true,
		Message:    "已在 PATH 中找到可执行文件。",
	}
}
