//go:build !windows

package launcher

import (
	"os/exec"

	"treeshift/internal/model"
)

// configureCommandForNewConsole 在非 Windows 平台不做额外处理。
func configureCommandForNewConsole(command *exec.Cmd) {}

// openTerminalWithPreferredPrivileges 在非 Windows 平台退回到普通终端启动。
//
// 当前应用主要面向 Windows，该实现只用于保持跨平台编译通过，
// 因此不做管理员提权，仅按原始方式启动 wt.exe。
func openTerminalWithPreferredPrivileges(path string, launchScript model.LaunchScriptSettings) error {
	_ = launchScript
	command := exec.Command("wt.exe", "-d", path)
	return command.Start()
}

// launchExternalToolWithPreferredPrivileges 在非 Windows 平台退回到普通外部 CLI 启动。
//
// 当前应用的管理员启动需求只在 Windows 平台生效，因此这里沿用原有行为：
// 在目标 worktree 目录下创建独立控制台并启动外部工具。
func launchExternalToolWithPreferredPrivileges(command string, args []string, workingDirectory string, launchScript model.LaunchScriptSettings) error {
	_ = launchScript
	process := exec.Command(command, args...)
	process.Dir = workingDirectory
	configureCommandForNewConsole(process)

	return process.Start()
}
