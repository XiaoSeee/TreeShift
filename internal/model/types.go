package model

// SettingsSchemaVersion 定义配置文件结构版本。
const SettingsSchemaVersion = 1

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
// Git 和 Windows Terminal 属于核心依赖，ExternalTools 对应用户在设置页配置的 AI CLI。
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
	SchemaVersion       int                 `json:"schemaVersion"`
	Repositories        []RepositoryBinding `json:"repositories"`
	DefaultWorktreeRoot string              `json:"defaultWorktreeRoot"`
	ExternalTools       []ExternalTool      `json:"externalTools"`
	UIPreferences       UIPreferences       `json:"uiPreferences"`
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

// WorktreeInfo 描述单个 worktree 的展示状态。
//
// Status 和 StatusMessage 同时覆盖 Git 正常 worktree 与“待清理”虚拟项。
type WorktreeInfo struct {
	Path          string `json:"path"`
	Branch        string `json:"branch"`
	Head          string `json:"head"`
	IsMain        bool   `json:"isMain"`
	IsDetached    bool   `json:"isDetached"`
	Status        string `json:"status"`
	StatusMessage string `json:"statusMessage"`
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
// Mode 允许 existing 或 new。new 模式要求同时提供 SourceBranch 与 BranchName。
type CreateWorktreeRequest struct {
	RepositoryID string `json:"repositoryId"`
	Mode         string `json:"mode"`
	SourceBranch string `json:"sourceBranch"`
	BranchName   string `json:"branchName"`
	TargetPath   string `json:"targetPath"`
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
// 它只存在于应用内存中，用于前端展示“待清理”虚拟卡片。
type PendingCleanup struct {
	RepositoryID string `json:"repositoryId"`
	Path         string `json:"path"`
	Branch       string `json:"branch"`
	Head         string `json:"head"`
	LastError    string `json:"lastError"`
}
