import type { ExternalTool, WorktreeInfo } from "../types";

/**
 * WorktreeCardProps 定义单张 worktree 卡片的输入。
 */
interface WorktreeCardProps {
  worktree: WorktreeInfo;
  tools: ExternalTool[];
  onAttachDetached: (worktree: WorktreeInfo) => void;
  onOpenExplorer: (path: string) => void;
  onOpenTerminal: (path: string) => void;
  onLaunchTool: (toolId: string, worktree: WorktreeInfo) => void;
  onToggleLock: (worktree: WorktreeInfo) => void;
  onRemove: (worktree: WorktreeInfo) => void;
}

/**
 * hasWorktreeChanges 判断当前卡片是否需要展示改动摘要。
 */
function hasWorktreeChanges(worktree: WorktreeInfo): boolean {
  return worktree.changeSummary.changedCount > 0 || worktree.changeSummary.deletedCount > 0;
}

/**
 * buildWorktreeDetailMessage 汇总状态说明与锁定原因。
 *
 * 状态说明优先保留 Git 或应用层已有提示；
 * 若 worktree 带有锁定原因，则额外拼接成“锁定原因：...”便于用户理解当前模式。
 */
function buildWorktreeDetailMessage(worktree: WorktreeInfo): string {
  const messages = [
    worktree.statusMessage.trim(),
    worktree.lockReason.trim() ? `锁定原因：${worktree.lockReason.trim()}` : "",
  ].filter((message) => message !== "");

  return messages.join(" · ");
}

/**
 * WorktreeCard 渲染单个 worktree 的路径、分支、模式标签与快捷动作。
 */
export function WorktreeCard({
  worktree,
  tools,
  onAttachDetached,
  onOpenExplorer,
  onOpenTerminal,
  onLaunchTool,
  onToggleLock,
  onRemove,
}: WorktreeCardProps) {
  const isUnavailable = worktree.status !== "normal";
  const canDelete = !worktree.isMain && !worktree.isLocked;
  const canToggleLock = !worktree.isMain && worktree.status !== "pending_cleanup";
  const canAttachDetached = worktree.isDetached && worktree.status === "normal";
  const shouldShowChangeSummary = worktree.status === "normal" && hasWorktreeChanges(worktree);
  const detailMessage = buildWorktreeDetailMessage(worktree);

  const cardTone =
    worktree.status === "pending_cleanup"
      ? "border-rose-200/80 bg-rose-50/60"
      : worktree.status === "missing"
        ? "border-amber-200/80 bg-amber-50/60"
        : "";
  const pathTone =
    worktree.status === "pending_cleanup"
      ? "border-rose-200/80 bg-white/75"
      : worktree.status === "missing"
        ? "border-amber-200/80 bg-white/75"
        : "border-stone-200/80 bg-white/55";
  const modeTone = worktree.isLocked ? "bg-sky-100 text-sky-800" : "bg-emerald-100 text-emerald-800";
  const lifecycleTone =
    worktree.status === "pending_cleanup"
      ? "bg-rose-100 text-rose-700"
      : "bg-amber-100 text-amber-900";
  const branchTagTone = worktree.isDetached ? "bg-ember-200 text-ember-900" : "bg-stone-900 text-white";

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
    <article className={`glass-panel overflow-hidden ${cardTone}`.trim()}>
      <div className="border-b border-stone-200/80 px-3 py-3">
        <div className="space-y-2">
          <div className="flex flex-wrap items-start justify-between gap-2">
            <div className="flex min-w-0 flex-wrap items-center gap-1.5">
              {canAttachDetached ? (
                <button
                  className={`tag cursor-pointer border border-transparent px-2 py-0.5 text-[10px] transition hover:border-ember-500 hover:bg-ember-300 ${branchTagTone}`}
                  onClick={() => onAttachDetached(worktree)}
                  type="button"
                >
                  DETACHED
                </button>
              ) : (
                <span className={`tag px-2 py-0.5 text-[10px] ${branchTagTone}`}>
                  {worktree.isDetached ? "DETACHED" : worktree.branch || "未识别分支"}
                </span>
              )}

              {canToggleLock ? (
                <button
                  className={`tag cursor-pointer border border-transparent px-2 py-0.5 text-[10px] transition hover:border-sky-400/70 hover:brightness-95 ${modeTone}`}
                  onClick={() => onToggleLock(worktree)}
                  title={worktree.isLocked ? "点击解锁当前 Worktree" : "点击锁定当前 Worktree"}
                  type="button"
                >
                  {worktree.isLocked ? "锁定" : "正常"}
                </button>
              ) : worktree.status !== "pending_cleanup" ? (
                <span className={`tag px-2 py-0.5 text-[10px] ${modeTone}`}>
                  {worktree.isLocked ? "锁定" : "正常"}
                </span>
              ) : null}

              {worktree.status === "pending_cleanup" ? (
                <span className={`tag px-2 py-0.5 text-[10px] ${lifecycleTone}`}>待清理</span>
              ) : worktree.status === "missing" ? (
                <span className={`tag px-2 py-0.5 text-[10px] ${lifecycleTone}`}>目录缺失</span>
              ) : null}

              {worktree.isMain ? <span className="tag bg-ember-100 px-2 py-0.5 text-[10px] text-ember-800">主工作区</span> : null}
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

          <div className={`flex items-center gap-2 rounded-xl border px-3 py-2 ${pathTone}`.trim()}>
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
              <button
                className="ghost-button shrink-0 px-2 py-1 text-[11px]"
                disabled={!canDelete}
                onClick={() => onRemove(worktree)}
                title={worktree.isLocked ? "请先解锁后再删除" : undefined}
                type="button"
              >
                删除
              </button>
              {secondaryActions}
            </div>
          </div>
        </div>

        {detailMessage ? (
          <p className="mt-2 rounded-xl border border-stone-200/80 bg-white/70 px-3 py-2 text-xs text-stone-700">
            {detailMessage}
          </p>
        ) : null}
      </div>
    </article>
  );
}
