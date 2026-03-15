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

// Check 扫描启动阶段必须依赖的环境项。
//
// 当前启动诊断只检查 Git 与 Windows Terminal；外部 CLI 属于可选能力，
// 即使在设置中存在预置或用户自定义工具，也不会在软件启动时自动执行环境检查。
func (s *Service) Check(_ model.Settings, warnings []string) model.EnvironmentStatus {
	status := model.EnvironmentStatus{
		Git:           s.checkGit(),
		Terminal:      s.checkTerminal(),
		ExternalTools: []model.ToolStatus{},
		Warnings:      append([]string{}, warnings...),
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
