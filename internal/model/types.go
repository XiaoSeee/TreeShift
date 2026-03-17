package model

// SettingsSchemaVersion 定义配置文件结构版本。
const SettingsSchemaVersion = 3

// Worktree 状态常量用于前端渲染标签和删除重试逻辑。
const (
	WorktreeStatusNormal         = "normal"
	WorktreeStatusMissing        = "missing"
	WorktreeStatusPendingCleanup = "pending_cleanup"
)

// 删除流程阶段常量用于前端区分不同反馈。
const (
	RemoveStageRemoved      = "removed"
	RemoveStageDirtyBlocked = "dirty_blocked"
	RemoveStageGitFailed    = "git_failed"
	RemoveStageFolderBusy   = "folder_busy"
)

// ToolStatus 描述单个工具的环境检查结果。
//
// 前端会据此展示工具是否可用、对应可执行文件路径以及错误原因。
type ToolStatus struct {
	Name       string `json:"name"`
	Executable string `json:"executable"`
	Available  bool   `json:"available"`
	Message    string `json:"message"`
}

// EnvironmentStatus 描述应用整体环境状态。
//
// Git 和 Windows Terminal 属于核心依赖，ExternalTools 预留给可选 CLI 状态，
// 启动默认诊断不会主动填充该字段。
type EnvironmentStatus struct {
	Git           ToolStatus   `json:"git"`
	Terminal      ToolStatus   `json:"terminal"`
	ExternalTools []ToolStatus `json:"externalTools"`
	Warnings      []string     `json:"warnings"`
}

// ExternalTool 定义一个外部 CLI 工具模板。
//
// 参数采用字符串数组存储，避免拼接完整 shell 命令带来的转义问题。
type ExternalTool struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Command string   `json:"command"`
	Args    []string `json:"args"`
	Enabled bool     `json:"enabled"`
}

// LaunchScriptSettings 描述启动终端或外部 CLI 前要执行的 PowerShell 脚本配置。
//
// PowerShellScript 保存用户在设置页中录入的原始脚本文本；
// ApplyToTerminal 与 ApplyToExternalTools 分别控制该脚本是否作用于“打开终端”
// 和“启动外部 CLI”两个入口。
type LaunchScriptSettings struct {
	PowerShellScript     string `json:"powerShellScript"`
	ApplyToTerminal      bool   `json:"applyToTerminal"`
	ApplyToExternalTools bool   `json:"applyToExternalTools"`
}

// RepositoryBinding 描述一个已绑定 Git 仓库的持久化记录。
//
// CommonDir 用于仓库去重，MainWorktreePath 用于后续 Git 命令执行。
type RepositoryBinding struct {
	ID                  string `json:"id"`
	DisplayName         string `json:"displayName"`
	SelectedPath        string `json:"selectedPath"`
	MainWorktreePath    string `json:"mainWorktreePath"`
	CommonDir           string `json:"commonDir"`
	DefaultWorktreeRoot string `json:"defaultWorktreeRoot"`
}

// UIPreferences 保存界面层的轻量偏好。
type UIPreferences struct {
	LastSelectedRepositoryID string `json:"lastSelectedRepositoryId"`
}

// Settings 是应用完整配置模型。
//
// 该结构会以 JSON 形式持久化到可执行文件同级目录。
type Settings struct {
	SchemaVersion       int                  `json:"schemaVersion"`
	Repositories        []RepositoryBinding  `json:"repositories"`
	DefaultWorktreeRoot string               `json:"defaultWorktreeRoot"`
	ExternalTools       []ExternalTool       `json:"externalTools"`
	LaunchScript        LaunchScriptSettings `json:"launchScript"`
	PendingCleanups     []PendingCleanup     `json:"pendingCleanups"`
	UIPreferences       UIPreferences        `json:"uiPreferences"`
}

// RepositorySummary 是仓库切换器所需的摘要信息。
type RepositorySummary struct {
	ID                  string `json:"id"`
	DisplayName         string `json:"displayName"`
	MainWorktreePath    string `json:"mainWorktreePath"`
	DefaultWorktreeRoot string `json:"defaultWorktreeRoot"`
	WorktreeCount       int    `json:"worktreeCount"`
	PendingCleanupCount int    `json:"pendingCleanupCount"`
}

// WorktreeChangeSummary 描述单个 worktree 的轻量改动摘要。
//
// ChangedCount 统计新增、修改、重命名、未跟踪等“正向改动”文件数，
// DeletedCount 统计删除类文件数。该结构只用于界面提示，不替代完整 diff。
type WorktreeChangeSummary struct {
	ChangedCount int `json:"changedCount"`
	DeletedCount int `json:"deletedCount"`
}

// WorktreeInfo 描述单个 worktree 的展示状态。
//
// Status 和 StatusMessage 同时覆盖 Git 正常 worktree 与“待清理”虚拟项；
// IsLocked 和 LockReason 则单独表达 Git `worktree lock` 模式，
// 允许界面同时展示“锁定 + 目录缺失”这类组合状态。
type WorktreeInfo struct {
	Path          string                `json:"path"`
	Branch        string                `json:"branch"`
	Head          string                `json:"head"`
	IsMain        bool                  `json:"isMain"`
	IsDetached    bool                  `json:"isDetached"`
	IsLocked      bool                  `json:"isLocked"`
	LockReason    string                `json:"lockReason"`
	Status        string                `json:"status"`
	StatusMessage string                `json:"statusMessage"`
	ChangeSummary WorktreeChangeSummary `json:"changeSummary"`
}

// RepositoryView 是主界面渲染当前仓库所需的完整数据。
type RepositoryView struct {
	Repository        RepositoryBinding `json:"repository"`
	Worktrees         []WorktreeInfo    `json:"worktrees"`
	AvailableBranches []string          `json:"availableBranches"`
	SuggestedRoot     string            `json:"suggestedRoot"`
}

// CreateWorktreeRequest 描述创建 worktree 所需的入参。
//
// Mode 允许 detached、existing 或 new。
// detached 与 existing 模式要求提供 SourceBranch，
// new 模式要求同时提供 SourceBranch 与 BranchName。
type CreateWorktreeRequest struct {
	RepositoryID string `json:"repositoryId"`
	Mode         string `json:"mode"`
	SourceBranch string `json:"sourceBranch"`
	BranchName   string `json:"branchName"`
	TargetPath   string `json:"targetPath"`
}

// AttachDetachedWorktreeRequest 描述把游离 HEAD worktree 附着到分支的入参。
//
// Mode 允许 existing 或 new。
// existing 模式会切换到一个已有本地分支，
// new 模式会基于当前游离 HEAD 创建新分支并切换过去。
type AttachDetachedWorktreeRequest struct {
	RepositoryID string `json:"repositoryId"`
	Path         string `json:"path"`
	Mode         string `json:"mode"`
	BranchName   string `json:"branchName"`
}

// SetWorktreeLockRequest 描述切换 worktree 锁定状态的请求。
//
// Locked=true 表示执行 `git worktree lock`，Locked=false 表示执行
// `git worktree unlock`。该请求仅适用于仍然存在 Git 记录的 linked worktree。
type SetWorktreeLockRequest struct {
	RepositoryID string `json:"repositoryId"`
	Path         string `json:"path"`
	Locked       bool   `json:"locked"`
}

// RemoveWorktreeRequest 描述删除 worktree 所需的入参。
type RemoveWorktreeRequest struct {
	RepositoryID string `json:"repositoryId"`
	Path         string `json:"path"`
	Force        bool   `json:"force"`
}

// RemoveWorktreeResult 描述删除操作结果。
//
// 对于脏目录拦截场景，RequiresForce 会为 true，前端可据此切换到强制删除确认。
type RemoveWorktreeResult struct {
	Success       bool           `json:"success"`
	Stage         string         `json:"stage"`
	Message       string         `json:"message"`
	RequiresForce bool           `json:"requiresForce"`
	View          RepositoryView `json:"view"`
}

// RetryDeleteFolderRequest 描述物理目录重试清理请求。
type RetryDeleteFolderRequest struct {
	RepositoryID string `json:"repositoryId"`
	Path         string `json:"path"`
}

// RetryDeleteFolderResult 描述目录重试清理结果。
type RetryDeleteFolderResult struct {
	Success bool           `json:"success"`
	Message string         `json:"message"`
	View    RepositoryView `json:"view"`
}

// LaunchToolRequest 描述外部 AI CLI 启动请求。
//
// Branch 仅用于参数模板占位符替换，不参与 Git 操作。
type LaunchToolRequest struct {
	ToolID       string `json:"toolId"`
	RepositoryID string `json:"repositoryId"`
	WorktreePath string `json:"worktreePath"`
	Branch       string `json:"branch"`
}

// DirectoryDialogRequest 描述原生目录选择器的参数。
type DirectoryDialogRequest struct {
	Title       string `json:"title"`
	DefaultPath string `json:"defaultPath"`
}

// PendingCleanup 保存 Git 已注销但物理目录仍未删除完成的项目。
//
// 该结构会持久化到配置文件中，确保刷新或重启后仍能继续展示和清理。
type PendingCleanup struct {
	RepositoryID string `json:"repositoryId"`
	Path         string `json:"path"`
	Branch       string `json:"branch"`
	Head         string `json:"head"`
	LastError    string `json:"lastError"`
}
