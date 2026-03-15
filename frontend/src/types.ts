/**
 * ToolStatus 描述单个外部依赖的可用性状态。
 */
export interface ToolStatus {
  name: string;
  executable: string;
  available: boolean;
  message: string;
}

/**
 * EnvironmentStatus 描述启动环境诊断结果。
 */
export interface EnvironmentStatus {
  git: ToolStatus;
  terminal: ToolStatus;
  externalTools: ToolStatus[];
  warnings: string[];
}

/**
 * ExternalTool 定义一个外部 CLI 工具模板。
 */
export interface ExternalTool {
  id: string;
  name: string;
  command: string;
  args: string[];
  enabled: boolean;
}

/**
 * RepositoryBinding 描述一个已绑定仓库的配置记录。
 */
export interface RepositoryBinding {
  id: string;
  displayName: string;
  selectedPath: string;
  mainWorktreePath: string;
  commonDir: string;
  defaultWorktreeRoot: string;
}

/**
 * UIPreferences 保存前端使用的轻量偏好。
 */
export interface UIPreferences {
  lastSelectedRepositoryId: string;
}

/**
 * Settings 对应后端持久化配置结构。
 */
export interface Settings {
  schemaVersion: number;
  repositories: RepositoryBinding[];
  defaultWorktreeRoot: string;
  externalTools: ExternalTool[];
  pendingCleanups: PendingCleanup[];
  uiPreferences: UIPreferences;
}

/**
 * RepositorySummary 是仓库切换器的摘要结构。
 */
export interface RepositorySummary {
  id: string;
  displayName: string;
  mainWorktreePath: string;
  defaultWorktreeRoot: string;
  worktreeCount: number;
  pendingCleanupCount: number;
}

/**
 * PendingCleanup 描述 Git 已移除但目录仍残留的待清理项。
 */
export interface PendingCleanup {
  repositoryId: string;
  path: string;
  branch: string;
  head: string;
  lastError: string;
}

/**
 * WorktreeChangeSummary 描述卡片右上角展示的轻量改动摘要。
 */
export interface WorktreeChangeSummary {
  changedCount: number;
  deletedCount: number;
}

/**
 * WorktreeStatus 定义 worktree 卡片可展示的状态枚举。
 */
export type WorktreeStatus = "normal" | "missing" | "pending_cleanup";

/**
 * WorktreeInfo 描述单个 worktree 卡片所需的信息。
 */
export interface WorktreeInfo {
  path: string;
  branch: string;
  head: string;
  isMain: boolean;
  isDetached: boolean;
  status: WorktreeStatus;
  statusMessage: string;
  changeSummary: WorktreeChangeSummary;
}

/**
 * RepositoryView 是主界面的完整仓库视图。
 */
export interface RepositoryView {
  repository: RepositoryBinding;
  worktrees: WorktreeInfo[];
  availableBranches: string[];
  suggestedRoot: string;
}

/**
 * CreateMode 定义 worktree 创建模式。
 */
export type CreateMode = "detached" | "existing" | "new";

/**
 * CreateWorktreeRequest 描述创建 worktree 所需入参。
 *
 * `detached` 与 `existing` 模式需要来源分支；
 * `new` 模式还需要提供新分支名称。
 */
export interface CreateWorktreeRequest {
  repositoryId: string;
  mode: CreateMode;
  sourceBranch: string;
  branchName: string;
  targetPath: string;
}

/**
 * AttachDetachedMode 定义游离 HEAD 附着分支时的操作模式。
 */
export type AttachDetachedMode = "existing" | "new";

/**
 * AttachDetachedWorktreeRequest 描述把游离 HEAD worktree 附着到分支的请求。
 */
export interface AttachDetachedWorktreeRequest {
  repositoryId: string;
  path: string;
  mode: AttachDetachedMode;
  branchName: string;
}

/**
 * RemoveWorktreeRequest 描述删除 worktree 请求。
 */
export interface RemoveWorktreeRequest {
  repositoryId: string;
  path: string;
  force: boolean;
}

/**
 * RemoveStage 定义删除流程的结果阶段。
 */
export type RemoveStage = "removed" | "dirty_blocked" | "git_failed" | "folder_busy";

/**
 * RemoveWorktreeResult 描述删除结果。
 */
export interface RemoveWorktreeResult {
  success: boolean;
  stage: RemoveStage;
  message: string;
  requiresForce: boolean;
  view: RepositoryView;
}

/**
 * RetryDeleteFolderRequest 描述目录重试清理请求。
 */
export interface RetryDeleteFolderRequest {
  repositoryId: string;
  path: string;
}

/**
 * RetryDeleteFolderResult 描述目录重试清理结果。
 */
export interface RetryDeleteFolderResult {
  success: boolean;
  message: string;
  view: RepositoryView;
}

/**
 * LaunchToolRequest 描述外部工具启动请求。
 */
export interface LaunchToolRequest {
  toolId: string;
  repositoryId: string;
  worktreePath: string;
  branch: string;
}

/**
 * DirectoryDialogRequest 描述目录选择器请求。
 */
export interface DirectoryDialogRequest {
  title: string;
  defaultPath: string;
}
