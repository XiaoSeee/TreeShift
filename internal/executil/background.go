package executil

import "os/exec"

// PrepareBackgroundCommand 配置后台命令执行方式。
//
// 该方法用于 Git 查询、环境检查等不应打断用户的后台命令。
// 在支持的平台上，它会尽量阻止系统弹出额外控制台窗口。
func PrepareBackgroundCommand(command *exec.Cmd) {
	if command == nil {
		return
	}

	prepareBackgroundCommand(command)
}
