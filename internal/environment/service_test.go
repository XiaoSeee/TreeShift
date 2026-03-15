package environment

import (
	"testing"

	"treeshift/internal/model"
)

// TestCheckDoesNotInspectExternalToolsOnStartup 验证启动环境检查不会扫描外部工具。
//
// 应用启动时只应关注 Git 和 Windows Terminal 这类核心依赖；
// 设置页中预置或用户自定义的可选 CLI 不应被纳入默认启动诊断。
func TestCheckDoesNotInspectExternalToolsOnStartup(t *testing.T) {
	service := NewService()

	status := service.Check(model.Settings{
		ExternalTools: []model.ExternalTool{
			{
				ID:      "tool-codex",
				Name:    "Codex CLI",
				Command: "codex",
				Enabled: true,
			},
			{
				ID:      "tool-custom",
				Name:    "My CLI",
				Command: "mycli",
				Enabled: true,
			},
		},
	}, nil)

	if len(status.ExternalTools) != 0 {
		t.Fatalf("启动环境检查不应返回外部工具状态：%v", status.ExternalTools)
	}
}
