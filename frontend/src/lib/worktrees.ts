import type { CreateMode, RepositoryView, WorktreeInfo, WorktreeStatus } from "../types";

/**
 * AttachBranchOption 描述游离 HEAD 附着弹窗中一个现有分支选项的展示状态。
 */
export interface AttachBranchOption {
  branch: string;
  disabled: boolean;
  occupiedPath: string;
}

/**
 * normalizePathKey 把 Windows 路径归一化为稳定比较键。
 *
 * 这里统一把斜杠转换为反斜杠并转为小写，
 * 用于前端在不同来源的路径字符串之间做等值比较。
 */
function normalizePathKey(path: string): string {
  return path.trim().replace(/[\\/]+/g, "\\").replace(/\\$/, "").toLowerCase();
}

/**
 * findMainWorktree 查找当前仓库视图中的主工作区卡片。
 *
 * TreeShift 约定主工作区始终由后端标记为 `isMain=true`，
 * 前端据此推导 detached 创建时的默认来源分支。
 */
function findMainWorktree(worktrees: WorktreeInfo[]): WorktreeInfo | undefined {
  return worktrees.find((worktree) => worktree.isMain);
}

/**
 * canOccupyBranch 判断某个 worktree 是否仍然占用一个本地分支。
 *
 * 只有 Git 记录仍存在且当前不是 detached 的 worktree 才会占用分支；
 * `pending_cleanup` 是应用内虚拟卡片，Git 记录已移除，因此不占用分支。
 */
function canOccupyBranch(status: WorktreeStatus, isDetached: boolean): boolean {
  if (status === "pending_cleanup") {
    return false;
  }
  if (isDetached) {
    return false;
  }

  return true;
}

/**
 * resolveDefaultCreateSourceBranch 计算新建 worktree 弹窗的默认来源分支。
 *
 * 优先使用主工作区当前检出的真实分支；
 * 如果主工作区本身处于 detached 状态，则回退到本地分支列表中的第一个。
 */
export function resolveDefaultCreateSourceBranch(view: RepositoryView): string {
  const mainWorktree = findMainWorktree(view.worktrees);
  if (mainWorktree && !mainWorktree.isDetached && mainWorktree.branch.trim()) {
    return mainWorktree.branch;
  }

  return view.availableBranches[0] ?? "";
}

/**
 * resolveCreateBranchFieldLabel 返回创建弹窗第二列字段的标题文案。
 *
 * detached 模式选择来源分支；
 * existing 模式选择要直接检出的现有分支；
 * new 模式选择新分支的基线分支。
 */
export function resolveCreateBranchFieldLabel(mode: CreateMode): string {
  if (mode === "detached") {
    return "来源分支";
  }
  if (mode === "new") {
    return "基线分支";
  }

  return "现有分支";
}

/**
 * buildAttachBranchOptions 构造游离 HEAD 切换现有分支时的下拉选项。
 *
 * 该函数会把已被其他 worktree 占用的本地分支标记为禁用，
 * 但仍保留在列表中，便于用户理解为什么当前不能切换过去。
 */
export function buildAttachBranchOptions(view: RepositoryView, currentWorktreePath: string): AttachBranchOption[] {
  const currentPathKey = normalizePathKey(currentWorktreePath);
  const occupiedPathsByBranch = new Map<string, string>();

  for (const worktree of view.worktrees) {
    if (!canOccupyBranch(worktree.status, worktree.isDetached)) {
      continue;
    }
    if (!worktree.branch.trim()) {
      continue;
    }
    if (normalizePathKey(worktree.path) === currentPathKey) {
      continue;
    }

    occupiedPathsByBranch.set(worktree.branch, worktree.path);
  }

  return view.availableBranches.map((branch) => {
    const occupiedPath = occupiedPathsByBranch.get(branch) ?? "";
    return {
      branch,
      disabled: occupiedPath !== "",
      occupiedPath,
    };
  });
}

/**
 * pickDefaultAttachExistingBranch 选择“切换到现有分支”模式下的默认分支。
 *
 * 默认选中第一个未被占用的本地分支；
 * 如果全部分支都被占用，则返回空字符串，交由界面提示不可切换。
 */
export function pickDefaultAttachExistingBranch(options: AttachBranchOption[]): string {
  return options.find((option) => !option.disabled)?.branch ?? "";
}
