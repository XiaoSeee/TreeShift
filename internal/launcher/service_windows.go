//go:build windows

package launcher

import (
	"encoding/base64"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"unicode/utf16"
	"unsafe"

	"treeshift/internal/model"
)

const (
	createNewConsole             = 0x00000010
	shellExecuteSuccessThreshold = 32
	showNormalWindow             = 1
)

var (
	shell32DLL       = syscall.NewLazyDLL("shell32.dll")
	shellExecuteProc = shell32DLL.NewProc("ShellExecuteW")
)

// configureCommandForNewConsole 让外部 CLI 以独立控制台窗口启动。
//
// 该方法仅用于“可见的新控制台”场景，例如显式启动 AI CLI。
// 它不会做提权处理，只负责为目标进程分配独立控制台窗口。
func configureCommandForNewConsole(command *exec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: createNewConsole,
	}
}

// openTerminalWithPreferredPrivileges 以管理员权限启动 Windows Terminal。
//
// 当未启用启动脚本时，仍沿用现有的默认 Profile 打开方式；
// 当启用了“打开终端前执行脚本”后，会改为显式使用 PowerShell，
// 并通过 EncodedCommand 安全承载用户脚本，避免分号被 wt.exe 误拆分。
func openTerminalWithPreferredPrivileges(path string, launchScript model.LaunchScriptSettings) error {
	cleanPath := filepath.Clean(path)
	if shouldRunTerminalLaunchScript(launchScript) {
		powerShellScript := buildTerminalStartupPowerShellScript(launchScript.PowerShellScript)
		err := shellExecuteRunAs("wt.exe", buildPowerShellTerminalArgs(cleanPath, powerShellScript, true), cleanPath)
		if err == nil {
			return nil
		}

		return shellExecuteRunAs("wt.exe", buildPowerShellTerminalArgs(cleanPath, powerShellScript, false), cleanPath)
	}

	err := shellExecuteRunAs("wt.exe", []string{"-w", "0", "nt", "-d", cleanPath}, cleanPath)
	if err == nil {
		return nil
	}

	return shellExecuteRunAs("wt.exe", []string{"-d", cleanPath}, cleanPath)
}

// launchExternalToolWithPreferredPrivileges 以管理员权限启动外部 AI CLI。
//
// 该方法不会直接提权启动 CLI 本体，而是通过管理员权限启动 Windows Terminal，
// 并在新标签页中显式使用 PowerShell 执行目标命令。这样可以同时满足：
// 1. 打开结果落在 Terminal 中，而不是回退到独立 conhost/cmd 窗口；
// 2. 新标签页的工作目录固定为目标 worktree；
// 3. CLI 退出后仍保留 PowerShell 会话，便于继续查看输出或补充命令。
//
// 当设置启用了“启动外部 CLI 前执行脚本”时，用户脚本会先于 CLI 执行；
// 若脚本执行失败或返回非零退出码，则当前终端会保留错误信息，但不会继续启动 CLI。
func launchExternalToolWithPreferredPrivileges(command string, args []string, workingDirectory string, launchScript model.LaunchScriptSettings) error {
	trimmedCommand := strings.TrimSpace(command)
	if shouldRunExternalToolLaunchScript(launchScript) {
		powerShellScript := buildExternalToolPowerShellScript(trimmedCommand, args, launchScript.PowerShellScript)
		return shellExecuteRunAs(
			"wt.exe",
			buildPowerShellTerminalArgs(workingDirectory, powerShellScript, true),
			workingDirectory,
		)
	}

	return shellExecuteRunAs(
		"wt.exe",
		buildElevatedExternalToolTerminalArgs(workingDirectory, trimmedCommand, args),
		workingDirectory,
	)
}

// shellExecuteRunAs 使用 Windows ShellExecuteW 以 runas 动词启动进程。
//
// file 表示要启动的可执行文件，args 会被按 Windows 命令行规则拼接，
// workingDirectory 用于指定目标进程的工作目录。返回值会保留系统层面的错误信息，
// 便于前端向用户反馈具体失败原因。
func shellExecuteRunAs(file string, args []string, workingDirectory string) error {
	verbPointer, err := syscall.UTF16PtrFromString("runas")
	if err != nil {
		return fmt.Errorf("构造提权动词失败：%w", err)
	}

	filePointer, err := syscall.UTF16PtrFromString(file)
	if err != nil {
		return fmt.Errorf("构造可执行文件路径失败：%w", err)
	}

	parametersPointer, err := utf16PointerOrNil(buildWindowsCommandLine(args))
	if err != nil {
		return fmt.Errorf("构造命令参数失败：%w", err)
	}

	directoryPointer, err := utf16PointerOrNil(workingDirectory)
	if err != nil {
		return fmt.Errorf("构造工作目录失败：%w", err)
	}

	result, _, callErr := shellExecuteProc.Call(
		0,
		uintptr(unsafe.Pointer(verbPointer)),
		uintptr(unsafe.Pointer(filePointer)),
		uintptr(unsafe.Pointer(parametersPointer)),
		uintptr(unsafe.Pointer(directoryPointer)),
		uintptr(showNormalWindow),
	)
	if result > shellExecuteSuccessThreshold {
		return nil
	}

	if callErr != syscall.Errno(0) {
		return fmt.Errorf("启动管理员进程失败：%w", callErr)
	}

	return fmt.Errorf("启动管理员进程失败：ShellExecuteW 返回码 %d", result)
}

// buildElevatedExternalToolTerminalArgs 构造管理员 Terminal 所需的命令参数。
//
// 参数会显式指定 `wt -w 0 nt`，优先把 AI CLI 放进最近使用的 Terminal 窗口的新标签页；
// 若当前不存在可复用窗口，Windows Terminal 会自行创建新窗口。
func buildElevatedExternalToolTerminalArgs(workingDirectory string, command string, args []string) []string {
	powerShellCommandLine := buildPowerShellLaunchCommand(command, args)
	return []string{
		"-w",
		"0",
		"nt",
		"-d",
		workingDirectory,
		"powershell.exe",
		"-NoExit",
		powerShellCommandLine,
	}
}

// buildPowerShellTerminalArgs 构造由 Windows Terminal 承载的 PowerShell 脚本启动参数。
//
// reuseWindow=true 时会优先复用最近使用的 Terminal 窗口；
// reuseWindow=false 时会强制新建窗口，用于复用失败后的回退路径。
func buildPowerShellTerminalArgs(workingDirectory string, powerShellScript string, reuseWindow bool) []string {
	args := make([]string, 0, 10)
	if reuseWindow {
		args = append(args, "-w", "0")
	}

	args = append(args,
		"nt",
		"-d",
		workingDirectory,
		"powershell.exe",
		"-NoExit",
		"-EncodedCommand",
		encodePowerShellScript(powerShellScript),
	)
	return args
}

// shouldRunTerminalLaunchScript 判断“打开终端”入口是否需要先执行启动脚本。
//
// 只有当脚本文本非空且启用了终端开关时，才会切换到 PowerShell 注入模式。
func shouldRunTerminalLaunchScript(launchScript model.LaunchScriptSettings) bool {
	return launchScript.ApplyToTerminal && strings.TrimSpace(launchScript.PowerShellScript) != ""
}

// shouldRunExternalToolLaunchScript 判断“启动外部 CLI”入口是否需要先执行启动脚本。
//
// 该判断会同时检查脚本文本与启用开关，避免空脚本误改变原有启动行为。
func shouldRunExternalToolLaunchScript(launchScript model.LaunchScriptSettings) bool {
	return launchScript.ApplyToExternalTools && strings.TrimSpace(launchScript.PowerShellScript) != ""
}

// buildTerminalStartupPowerShellScript 生成“打开终端”入口使用的 PowerShell 脚本。
//
// 这里会显式把 ErrorActionPreference 设为 Stop，确保脚本里的 PowerShell 错误
// 能在当前终端中直接暴露出来，而不是被静默吞掉后继续执行后续语句。
func buildTerminalStartupPowerShellScript(userScript string) string {
	scriptLines := []string{
		"$treeShiftOriginalErrorActionPreference = $ErrorActionPreference",
		"try {",
		"$ErrorActionPreference = 'Stop'",
		strings.TrimSpace(userScript),
		"} finally {",
		"  $ErrorActionPreference = $treeShiftOriginalErrorActionPreference",
		"}",
	}
	return strings.Join(scriptLines, "\n")
}

// buildExternalToolPowerShellScript 生成“外部 CLI”入口的完整 PowerShell 脚本。
//
// 脚本会先执行用户配置的启动脚本，再在同一终端中启动目标 CLI。
// 若前置脚本失败，则直接 return，保留当前窗口供用户查看错误输出。
func buildExternalToolPowerShellScript(command string, args []string, userScript string) string {
	scriptLines := []string{
		"$treeShiftOriginalErrorActionPreference = $ErrorActionPreference",
		"try {",
		"  $ErrorActionPreference = 'Stop'",
		"  $global:LASTEXITCODE = 0",
		indentPowerShellScript(strings.TrimSpace(userScript)),
		"  if (-not $?) {",
		"    Write-Error '启动前脚本执行失败，已取消后续 CLI 启动。'",
		"    return",
		"  }",
		"  if ($LASTEXITCODE -ne 0) {",
		"    Write-Error '启动前脚本返回非零退出码，已取消后续 CLI 启动。'",
		"    return",
		"  }",
		"} finally {",
		"  $ErrorActionPreference = $treeShiftOriginalErrorActionPreference",
		"}",
		buildPowerShellLaunchCommand(command, args),
	}
	return strings.Join(scriptLines, "\n")
}

// indentPowerShellScript 为多行 PowerShell 脚本统一补缩进。
//
// 该方法仅用于把用户脚本文本嵌入 try 代码块，避免多行脚本破坏生成结果的层级结构。
func indentPowerShellScript(script string) string {
	if script == "" {
		return ""
	}

	lines := strings.Split(script, "\n")
	for index := range lines {
		lines[index] = "  " + lines[index]
	}

	return strings.Join(lines, "\n")
}

// encodePowerShellScript 按 PowerShell -EncodedCommand 的要求编码脚本。
//
// PowerShell 要求传入的内容是 UTF-16LE 字节序列的 Base64 字符串，
// 这样可以避免多层命令行转义导致的引号、分号和换行问题。
func encodePowerShellScript(script string) string {
	utf16Values := utf16.Encode([]rune(script))
	rawBytes := make([]byte, 0, len(utf16Values)*2)
	for _, value := range utf16Values {
		rawBytes = append(rawBytes, byte(value), byte(value>>8))
	}

	return base64.StdEncoding.EncodeToString(rawBytes)
}

// buildPowerShellLaunchCommand 生成传给 PowerShell 的单条命令字符串。
//
// 这里故意不再使用分号拼接多条 PowerShell 语句，因为 Windows Terminal 会把分号
// 解释为自己的命令分隔符，进而错误拆出额外标签页。工作目录切换统一交给 `wt -d` 处理。
func buildPowerShellLaunchCommand(command string, args []string) string {
	commandParts := []string{buildPowerShellCommandInvocation(command)}
	for _, argument := range args {
		commandParts = append(commandParts, fmt.Sprintf("'%s'", escapePowerShellSingleQuotedString(argument)))
	}

	return strings.Join(commandParts, " ")
}

// buildPowerShellCommandInvocation 生成 PowerShell 可执行的命令头。
//
// 对于 `codex` 这类简单命令名，直接返回裸命令，让 PowerShell 自己按 PATH 和 PATHEXT 解析；
// 对于绝对路径、相对路径或包含空格的命令，则退回到调用运算符模式。
func buildPowerShellCommandInvocation(command string) string {
	trimmedCommand := strings.TrimSpace(command)
	if isSimplePowerShellCommandToken(trimmedCommand) {
		return trimmedCommand
	}

	return fmt.Sprintf("& '%s'", escapePowerShellSingleQuotedString(trimmedCommand))
}

// isSimplePowerShellCommandToken 判断命令是否可以作为 PowerShell 裸命令使用。
//
// 只要命令中出现路径分隔符、盘符、空白或引号，就说明它更适合走调用运算符模式；
// 否则直接交给 PowerShell 做标准命令解析即可。
func isSimplePowerShellCommandToken(command string) bool {
	if command == "" {
		return false
	}

	if strings.ContainsAny(command, `\/:'" `+"\t") {
		return false
	}

	return true
}

// escapePowerShellSingleQuotedString 转义 PowerShell 单引号字符串中的单引号。
//
// PowerShell 在单引号字符串中使用两个连续单引号表示字面量单引号，
// 因此这里只需要做这一项最小且稳定的转义。
func escapePowerShellSingleQuotedString(value string) string {
	return strings.ReplaceAll(value, "'", "''")
}

// buildWindowsCommandLine 按 Windows 的转义规则拼接参数列表。
//
// 该方法会对每个参数执行单独转义，确保包含空格、引号等字符的目录路径
// 在 ShellExecuteW 中仍能被 wt.exe 正确解析。
func buildWindowsCommandLine(args []string) string {
	if len(args) == 0 {
		return ""
	}

	escapedArgs := make([]string, 0, len(args))
	for _, argument := range args {
		escapedArgs = append(escapedArgs, syscall.EscapeArg(argument))
	}

	return strings.Join(escapedArgs, " ")
}

// utf16PointerOrNil 将字符串安全地转换为 UTF-16 指针。
//
// 当输入为空字符串时，该方法返回 nil，避免 ShellExecuteW 收到无意义的空参数指针。
func utf16PointerOrNil(value string) (*uint16, error) {
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}

	return syscall.UTF16PtrFromString(value)
}
