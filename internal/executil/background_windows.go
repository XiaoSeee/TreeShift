//go:build windows

package executil

import (
	"os/exec"
	"syscall"
)

const createNoWindow = 0x08000000

// prepareBackgroundCommand 在 Windows 下隐藏后台命令的控制台窗口。
func prepareBackgroundCommand(command *exec.Cmd) {
	if command.SysProcAttr == nil {
		command.SysProcAttr = &syscall.SysProcAttr{}
	}

	command.SysProcAttr.HideWindow = true
	command.SysProcAttr.CreationFlags |= createNoWindow
}
