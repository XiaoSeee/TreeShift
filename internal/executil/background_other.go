//go:build !windows

package executil

import "os/exec"

// prepareBackgroundCommand 在非 Windows 平台不做额外处理。
func prepareBackgroundCommand(command *exec.Cmd) {}
