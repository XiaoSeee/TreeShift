import type { ExternalTool, WorktreeInfo } from "../types";

/**
 * WorktreeCardProps 定义单张 worktree 卡片的输入。
 */
interface WorktreeCardProps {
  worktree: WorktreeInfo;
  tools: ExternalTool[];
  onOpenExplorer: (path: string) => void;
  onOpenTerminal: (path: string) => void;
  onLaunchTool: (toolId: string, worktree: WorktreeInfo) => void;
  onRemove: (worktree: WorktreeInfo) => void;
  onRetryCleanup: (worktree: WorktreeInfo) => void;
}

/**
 * hasWorktreeChanges 判断当前卡片是否需要展示改动摘要。
 */
function hasWorktreeChanges(worktree: WorktreeInfo): boolean {
  return worktree.changeSummary.changedCount > 0 || worktree.changeSummary.deletedCount > 0;
}

/**
 * WorktreeCard 渲染单个 worktree 的路径、分支、状态和快捷动作。
 */
export function WorktreeCard({
  worktree,
  tools,
  onOpenExplorer,
  onOpenTerminal,
  onLaunchTool,
  onRemove,
  onRetryCleanup,
}: WorktreeCardProps) {
  const isUnavailable = worktree.status !== "normal";
  const canDelete = !worktree.isMain && worktree.status === "normal";
  const shouldShowChangeSummary = worktree.status === "normal" && hasWorktreeChanges(worktree);
  const statusTone =
    worktree.status === "pending_cleanup"
      ? "bg-amber-100 text-amber-900"
      : worktree.status === "missing"
        ? "bg-rose-100 text-rose-700"
        : "bg-emerald-100 text-emerald-800";
  const secondaryActions = tools.length === 0
    ? (
      <span className="rounded-full border border-dashed border-stone-300 px-2 py-1 text-[10px] text-stone-500">
        无 AI
      </span>
    )
    : (
      tools.map((tool) => (
        <button
          key={tool.id}
          className="ghost-button shrink-0 px-2 py-1 text-[11px]"
          disabled={isUnavailable}
          onClick={() => onLaunchTool(tool.id, worktree)}
          type="button"
        >
          {tool.name}
        </button>
      ))
    );

  return (
    <article className="glass-panel overflow-hidden">
      <div className="border-b border-stone-200/80 px-3 py-3">
        <div className="space-y-2">
          <div className="flex flex-wrap items-start justify-between gap-2">
            <div className="flex min-w-0 flex-wrap items-center gap-1.5">
              <span className="tag bg-stone-900 px-2 py-0.5 text-[10px] text-white">{worktree.branch || "未识别分支"}</span>
              <span className={`tag px-2 py-0.5 text-[10px] ${statusTone}`}>
                {worktree.status === "pending_cleanup"
                  ? "待清理"
                  : worktree.status === "missing"
                    ? "目录缺失"
                    : "正常"}
              </span>
              {worktree.isMain ? <span className="tag bg-ember-100 px-2 py-0.5 text-[10px] text-ember-800">主工作区</span> : null}
              {worktree.isDetached ? <span className="tag bg-stone-200 px-2 py-0.5 text-[10px] text-stone-700">Detached</span> : null}
              <span className="tag bg-stone-100 px-2 py-0.5 text-[10px] text-stone-700">HEAD {worktree.head ? worktree.head.slice(0, 8) : "N/A"}</span>
            </div>
            {shouldShowChangeSummary ? (
              <div
                className="worktree-change-pill shrink-0"
                title={`当前目录改动提示：+${worktree.changeSummary.changedCount}，删除：-${worktree.changeSummary.deletedCount}`}
              >
                <span className="worktree-change-pill__plus">+{worktree.changeSummary.changedCount}</span>
                <span className="worktree-change-pill__divider">|</span>
                <span className="worktree-change-pill__minus">-{worktree.changeSummary.deletedCount}</span>
              </div>
            ) : null}
          </div>
          <div className="flex items-center gap-2 rounded-xl border border-stone-200/80 bg-white/55 px-3 py-2">
            <div className="min-w-0 flex-1">
              <p className="truncate text-sm font-medium leading-5 text-stone-900" title={worktree.path}>
                {worktree.path}
              </p>
            </div>
            <div className="flex flex-none items-center gap-1 overflow-x-auto">
              <button
                className="ghost-button shrink-0 px-2 py-1 text-[11px]"
                disabled={worktree.status === "missing"}
                onClick={() => onOpenExplorer(worktree.path)}
                type="button"
              >
                目录
              </button>
              <button
                className="ghost-button shrink-0 px-2 py-1 text-[11px]"
                disabled={isUnavailable}
                onClick={() => onOpenTerminal(worktree.path)}
                type="button"
              >
                终端
              </button>
              {worktree.status === "pending_cleanup" ? (
                <button className="primary-button shrink-0 px-2 py-1 text-[11px]" onClick={() => onRetryCleanup(worktree)} type="button">
                  重试
                </button>
              ) : (
                <button className="ghost-button shrink-0 px-2 py-1 text-[11px]" disabled={!canDelete} onClick={() => onRemove(worktree)} type="button">
                  删除
                </button>
              )}
              {secondaryActions}
            </div>
          </div>
        </div>
        {worktree.statusMessage ? (
          <p className="mt-2 rounded-xl border border-stone-200/80 bg-white/70 px-3 py-2 text-xs text-stone-700">
            {worktree.statusMessage}
          </p>
        ) : null}
      </div>
    </article>
  );
}
