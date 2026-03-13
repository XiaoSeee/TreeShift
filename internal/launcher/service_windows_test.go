//go:build windows

package launcher

import (
	"strings"
	"testing"
)

// TestBuildWindowsCommandLine 会验证带空格的路径参数仍会被正确加引号。
//
// 该测试不直接启动外部进程，只校验 ShellExecuteW 所需的参数拼接逻辑，
// 以避免管理员提权场景难以在自动化测试里稳定执行的问题。
func TestBuildWindowsCommandLine(t *testing.T) {
	commandLine := buildWindowsCommandLine([]string{
		"-w",
		"0",
		"nt",
		"-d",
		`D:\Code\My Repo\feature branch`,
	})

	if !strings.Contains(commandLine, `-w 0 nt -d`) {
		t.Fatalf("命令行前缀不符合预期：%s", commandLine)
	}

	if !strings.Contains(commandLine, `"D:\Code\My Repo\feature branch"`) {
		t.Fatalf("路径参数未被正确包裹：%s", commandLine)
	}
}

// TestBuildElevatedExternalToolTerminalArgs 会验证 AI CLI 会通过 Terminal + PowerShell 启动。
//
// 该测试的目标是锁定最关键的用户体验约束：
// 必须优先走当前 Terminal 窗口的新标签页，并且明确使用 PowerShell 作为承载 shell。
func TestBuildElevatedExternalToolTerminalArgs(t *testing.T) {
	args := buildElevatedExternalToolTerminalArgs(
		`D:\Code\Repo A`,
		"codex",
		[]string{"--model", "gpt-5", "hello world"},
	)

	commandLine := buildWindowsCommandLine(args)
	if !strings.Contains(commandLine, `-w 0 nt -d "D:\Code\Repo A"`) {
		t.Fatalf("Terminal 标签页参数不符合预期：%s", commandLine)
	}

	if !strings.Contains(commandLine, `powershell.exe -NoExit "codex '--model' 'gpt-5' 'hello world'"`) {
		t.Fatalf("未按 PowerShell 模式启动：%s", commandLine)
	}
}

// TestBuildPowerShellCommandInvocation 会验证简单命令名不会被错误地改写成路径调用。
//
// 这能避免 `codex` 这类依赖 PowerShell 自身 PATH/PATHEXT 解析的命令
// 被我们提前展开成用户不希望看到的 `.cmd` 路径。
func TestBuildPowerShellCommandInvocation(t *testing.T) {
	if actual := buildPowerShellCommandInvocation("codex"); actual != "codex" {
		t.Fatalf("简单命令名应保持原样：%s", actual)
	}

	if actual := buildPowerShellCommandInvocation(`C:\Program Files\Tool\tool.exe`); actual != `& 'C:\Program Files\Tool\tool.exe'` {
		t.Fatalf("带路径命令应使用调用运算符：%s", actual)
	}
}

// TestBuildPowerShellLaunchCommand 会验证最终传给 PowerShell 的命令字符串不含分号。
//
// 这是为了避免 Windows Terminal 把分号误判成自己的命令分隔符，
// 从而把一次 AI CLI 启动拆成多个错误的标签页动作。
func TestBuildPowerShellLaunchCommand(t *testing.T) {
	commandLine := buildPowerShellLaunchCommand("codex", []string{"--model", "gpt-5"})
	if commandLine != "codex '--model' 'gpt-5'" {
		t.Fatalf("PowerShell 命令字符串不符合预期：%s", commandLine)
	}

	if strings.Contains(commandLine, ";") {
		t.Fatalf("PowerShell 命令字符串不应包含分号：%s", commandLine)
	}
}
