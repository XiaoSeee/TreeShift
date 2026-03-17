//go:build windows

package launcher

import (
	"encoding/base64"
	"strings"
	"testing"
	"unicode/utf16"
)

// TestBuildWindowsCommandLine 验证带空格的路径参数仍会被正确加引号。
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

// TestBuildElevatedExternalToolTerminalArgs 验证默认 AI CLI 仍通过 Terminal + PowerShell 启动。
//
// 在未启用启动脚本时，外部 CLI 应继续走现有的 PowerShell 命令行承载模式，
// 避免无关设置改变已有工具模板的行为和输出形态。
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

// TestBuildPowerShellTerminalArgs 验证注入脚本时会改用 EncodedCommand 承载脚本内容。
//
// 该模式用于承载包含分号、换行和引号的前置脚本，避免这些字符被 wt.exe
// 误识别为自己的命令分隔符。
func TestBuildPowerShellTerminalArgs(t *testing.T) {
	script := "$treeShiftOriginalErrorActionPreference = $ErrorActionPreference\ntry {\n$ErrorActionPreference = 'Stop'\n$env:HTTP_PROXY='http://127.0.0.1:6789'\n} finally {\n  $ErrorActionPreference = $treeShiftOriginalErrorActionPreference\n}"
	args := buildPowerShellTerminalArgs(`D:\Code\Repo A`, script, true)

	commandLine := buildWindowsCommandLine(args)
	if !strings.Contains(commandLine, `powershell.exe -NoExit -EncodedCommand`) {
		t.Fatalf("未切换到 EncodedCommand 模式：%s", commandLine)
	}

	encoded := argumentValueAfter(args, "-EncodedCommand")
	if encoded == "" {
		t.Fatal("缺少 EncodedCommand 参数值")
	}

	decoded := decodePowerShellEncodedCommand(t, encoded)
	if decoded != script {
		t.Fatalf("EncodedCommand 解码结果不符合预期：got=%q want=%q", decoded, script)
	}
}

// TestBuildExternalToolPowerShellScript 验证前置脚本会先于 CLI 执行，并带有失败门禁。
//
// 该测试会锁定“先执行用户脚本，再执行外部 CLI”的顺序要求，
// 并确保脚本失败时不会静默继续启动后续 CLI。
func TestBuildExternalToolPowerShellScript(t *testing.T) {
	powerShellScript := buildExternalToolPowerShellScript(
		"codex",
		[]string{"--model", "gpt-5"},
		"$env:HTTP_PROXY='http://127.0.0.1:6789';\n$env:HTTPS_PROXY='http://127.0.0.1:6789'",
	)

	if !strings.Contains(powerShellScript, "$global:LASTEXITCODE = 0") {
		t.Fatalf("缺少启动脚本退出码重置：%s", powerShellScript)
	}
	if !strings.Contains(powerShellScript, "$treeShiftOriginalErrorActionPreference = $ErrorActionPreference") {
		t.Fatalf("缺少 ErrorActionPreference 恢复保护：%s", powerShellScript)
	}
	if !strings.Contains(powerShellScript, "$ErrorActionPreference = $treeShiftOriginalErrorActionPreference") {
		t.Fatalf("缺少 ErrorActionPreference 恢复语句：%s", powerShellScript)
	}

	proxyIndex := strings.Index(powerShellScript, "$env:HTTP_PROXY='http://127.0.0.1:6789'")
	cliIndex := strings.Index(powerShellScript, "codex '--model' 'gpt-5'")
	if proxyIndex < 0 || cliIndex < 0 || proxyIndex > cliIndex {
		t.Fatalf("前置脚本与 CLI 顺序不符合预期：%s", powerShellScript)
	}

	if !strings.Contains(powerShellScript, "启动前脚本执行失败") {
		t.Fatalf("缺少前置脚本失败保护：%s", powerShellScript)
	}
	if !strings.Contains(powerShellScript, "启动前脚本返回非零退出码") {
		t.Fatalf("缺少非零退出码保护：%s", powerShellScript)
	}
}

// TestBuildTerminalStartupPowerShellScript 验证终端启动脚本会恢复内部错误策略修改。
//
// 启动脚本只应把 ErrorActionPreference 调整用于预启动阶段，
// 不应把这一内部保护设置泄漏到用户随后继续交互的终端会话。
func TestBuildTerminalStartupPowerShellScript(t *testing.T) {
	powerShellScript := buildTerminalStartupPowerShellScript("$env:HTTP_PROXY='http://127.0.0.1:6789'")

	if !strings.Contains(powerShellScript, "$treeShiftOriginalErrorActionPreference = $ErrorActionPreference") {
		t.Fatalf("缺少 ErrorActionPreference 保存语句：%s", powerShellScript)
	}
	if !strings.Contains(powerShellScript, "$ErrorActionPreference = 'Stop'") {
		t.Fatalf("缺少 ErrorActionPreference 设置语句：%s", powerShellScript)
	}
	if !strings.Contains(powerShellScript, "$ErrorActionPreference = $treeShiftOriginalErrorActionPreference") {
		t.Fatalf("缺少 ErrorActionPreference 恢复语句：%s", powerShellScript)
	}
}

// TestBuildPowerShellCommandInvocation 验证简单命令名不会被错误地改写成路径调用。
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

// TestBuildPowerShellLaunchCommand 验证传给 PowerShell 的命令字符串仍保持单条命令模式。
//
// 这里不应主动拼接分号，否则 Windows Terminal 可能把分号误判成自己的命令分隔符。
func TestBuildPowerShellLaunchCommand(t *testing.T) {
	commandLine := buildPowerShellLaunchCommand("codex", []string{"--model", "gpt-5"})
	if commandLine != "codex '--model' 'gpt-5'" {
		t.Fatalf("PowerShell 命令字符串不符合预期：%s", commandLine)
	}

	if strings.Contains(commandLine, ";") {
		t.Fatalf("PowerShell 命令字符串不应包含分号：%s", commandLine)
	}
}

// argumentValueAfter 返回参数列表中指定标记后的第一个值。
//
// 该辅助方法仅用于测试，方便从 wt.exe 参数列表中提取 EncodedCommand 的内容。
func argumentValueAfter(args []string, flag string) string {
	for index := 0; index < len(args)-1; index++ {
		if args[index] == flag {
			return args[index+1]
		}
	}

	return ""
}

// decodePowerShellEncodedCommand 按 PowerShell EncodedCommand 规则解码脚本文本。
//
// PowerShell 使用 UTF-16LE Base64 编码脚本，该测试辅助方法会把编码内容还原为
// 原始脚本文本，便于断言脚本内容和顺序。
func decodePowerShellEncodedCommand(t *testing.T, encoded string) string {
	t.Helper()

	rawBytes, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatalf("EncodedCommand Base64 解码失败：%v", err)
	}

	if len(rawBytes)%2 != 0 {
		t.Fatalf("EncodedCommand 字节长度不合法：%d", len(rawBytes))
	}

	utf16Values := make([]uint16, 0, len(rawBytes)/2)
	for index := 0; index < len(rawBytes); index += 2 {
		utf16Values = append(utf16Values, uint16(rawBytes[index])|uint16(rawBytes[index+1])<<8)
	}

	return string(utf16.Decode(utf16Values))
}
