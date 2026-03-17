import { joinWindowsPath } from "./formats";
import type { RepositoryBinding, RepositorySummary, RepositoryView, Settings } from "../types";

/**
 * WorkspaceSnapshot 描述仅根据最新设置即可在前端立即落地的工作区状态快照。
 *
 * 该快照不会重新拉取 Git 视图，而是尽量复用当前前端已持有的摘要和仓库视图，
 * 用于“设置已持久化，但后续刷新失败”时先把界面收敛到不误导用户的状态。
 */
export interface WorkspaceSnapshot {
  settings: Settings;
  repositories: RepositorySummary[];
  activeRepositoryId: string;
  repositoryView: RepositoryView | null;
}

/**
 * DeriveWorkspaceSnapshotOptions 描述推导本地工作区快照时需要的输入。
 */
interface DeriveWorkspaceSnapshotOptions {
  currentRepositories: RepositorySummary[];
  currentRepositoryView: RepositoryView | null;
  nextSettings: Settings;
  preferredRepositoryId?: string;
}

/**
 * buildPendingCleanupCountMap 按仓库统计待清理目录数量。
 *
 * @param settings 最新设置。
 * @returns 以仓库 ID 为键的待清理数量映射。
 */
function buildPendingCleanupCountMap(settings: Settings): Map<string, number> {
  const counts = new Map<string, number>();

  for (const cleanup of settings.pendingCleanups) {
    const currentCount = counts.get(cleanup.repositoryId) ?? 0;
    counts.set(cleanup.repositoryId, currentCount + 1);
  }

  return counts;
}

/**
 * normalizeWindowsPath 统一整理 Windows 风格路径分隔符与尾部分隔符。
 *
 * @param value 原始路径文本。
 * @returns 归一化后的路径；若原始值为空，则返回空字符串。
 */
function normalizeWindowsPath(value: string): string {
  const trimmed = value.trim();
  if (trimmed === "") {
    return "";
  }

  return trimmed.replace(/[\\/]+/g, "\\").replace(/\\$/, "");
}

/**
 * resolveWindowsParentPath 返回一个 Windows 路径的父目录。
 *
 * @param value 需要取父目录的路径。
 * @returns 父目录路径；若无法安全判断，则返回原值。
 */
function resolveWindowsParentPath(value: string): string {
  const normalized = normalizeWindowsPath(value);
  const lastSeparatorIndex = normalized.lastIndexOf("\\");

  if (lastSeparatorIndex < 0) {
    return normalized;
  } else if (lastSeparatorIndex === 2 && /^[A-Za-z]:/.test(normalized.slice(0, 2))) {
    return `${normalized.slice(0, 2)}\\`;
  } else if (lastSeparatorIndex === 0) {
    return normalized;
  }

  return normalized.slice(0, lastSeparatorIndex);
}

/**
 * resolveSuggestedRootPath 在前端复用后端推荐 worktree 根目录的推导规则。
 *
 * @param repository 当前仓库绑定。
 * @param globalDefaultRoot 全局默认根目录。
 * @returns 该仓库当前应使用的建议 worktree 根目录。
 */
export function resolveSuggestedRootPath(repository: RepositoryBinding, globalDefaultRoot: string): string {
  const repositoryRoot = normalizeWindowsPath(repository.defaultWorktreeRoot);
  if (repositoryRoot !== "") {
    return repositoryRoot;
  }

  const globalRoot = normalizeWindowsPath(globalDefaultRoot);
  if (globalRoot !== "") {
    return globalRoot;
  }

  const parentPath = resolveWindowsParentPath(repository.mainWorktreePath);
  return joinWindowsPath(parentPath, "_worktrees", repository.displayName);
}

/**
 * pickActiveRepositoryId 根据最新设置与仓库摘要选择当前应展示的仓库。
 *
 * @param repositories 当前可选仓库摘要。
 * @param settings 最新设置。
 * @param preferredRepositoryId 希望尽量保留的仓库 ID。
 * @returns 前端下一步应展示的仓库 ID；若不存在则返回空字符串。
 */
export function pickActiveRepositoryId(
  repositories: RepositorySummary[],
  settings: Settings,
  preferredRepositoryId?: string,
): string {
  const ids = new Set(repositories.map((repository) => repository.id));

  if (preferredRepositoryId && ids.has(preferredRepositoryId)) {
    return preferredRepositoryId;
  } else if (settings.uiPreferences.lastSelectedRepositoryId && ids.has(settings.uiPreferences.lastSelectedRepositoryId)) {
    return settings.uiPreferences.lastSelectedRepositoryId;
  }

  return repositories[0]?.id ?? "";
}

/**
 * deriveWorkspaceSnapshotFromSettings 仅基于最新设置推导一个本地可展示的工作区快照。
 *
 * 该方法会保留已有仓库摘要中的 worktree 数量，避免在刷新失败时把已有列表全部清零；
 * 同时会同步更新仓库名称、默认路径、待清理计数与当前仓库视图中的绑定信息。
 *
 * @param options 推导快照所需的上下文。
 * @returns 一个可立即应用到前端状态树的本地快照。
 */
export function deriveWorkspaceSnapshotFromSettings(options: DeriveWorkspaceSnapshotOptions): WorkspaceSnapshot {
  const pendingCleanupCountMap = buildPendingCleanupCountMap(options.nextSettings);
  const currentSummaryMap = new Map(options.currentRepositories.map((repository) => [repository.id, repository]));
  const repositories = options.nextSettings.repositories.map((repository) => {
    const currentSummary = currentSummaryMap.get(repository.id);

    return {
      id: repository.id,
      displayName: repository.displayName,
      mainWorktreePath: repository.mainWorktreePath,
      defaultWorktreeRoot: repository.defaultWorktreeRoot,
      worktreeCount: currentSummary?.worktreeCount ?? 0,
      pendingCleanupCount: pendingCleanupCountMap.get(repository.id) ?? 0,
    };
  });
  const activeRepositoryId = pickActiveRepositoryId(repositories, options.nextSettings, options.preferredRepositoryId);
  const activeRepository = options.nextSettings.repositories.find((repository) => repository.id === activeRepositoryId);

  let repositoryView: RepositoryView | null = null;
  if (options.currentRepositoryView && activeRepository && options.currentRepositoryView.repository.id === activeRepositoryId) {
    repositoryView = {
      ...options.currentRepositoryView,
      repository: activeRepository,
      suggestedRoot: resolveSuggestedRootPath(activeRepository, options.nextSettings.defaultWorktreeRoot),
    };
  }

  return {
    settings: options.nextSettings,
    repositories,
    activeRepositoryId,
    repositoryView,
  };
}
