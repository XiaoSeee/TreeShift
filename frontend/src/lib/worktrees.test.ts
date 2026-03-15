import type { RepositoryView } from "../types";
import {
  buildAttachBranchOptions,
  pickDefaultAttachExistingBranch,
  resolveCreateBranchFieldLabel,
  resolveDefaultCreateSourceBranch,
} from "./worktrees";

/**
 * createRepositoryView 构造前端 worktree 逻辑测试使用的最小仓库视图。
 *
 * 这里刻意只保留当前测试关心的字段，
 * 让每个用例都能直接表达分支占用和默认值推导规则。
 */
function createRepositoryView(): RepositoryView {
  return {
    repository: {
      id: "repo-1",
      displayName: "demo",
      selectedPath: "D:\\Code\\demo",
      mainWorktreePath: "D:\\Code\\demo",
      commonDir: "D:\\Code\\demo\\.git",
      defaultWorktreeRoot: "D:\\Code\\demo\\_worktrees",
    },
    worktrees: [
      {
        path: "D:\\Code\\demo",
        branch: "main",
        head: "11111111",
        isMain: true,
        isDetached: false,
        status: "normal",
        statusMessage: "",
        changeSummary: { changedCount: 0, deletedCount: 0 },
      },
      {
        path: "D:\\Code\\demo\\_worktrees\\detached",
        branch: "detached",
        head: "22222222",
        isMain: false,
        isDetached: true,
        status: "normal",
        statusMessage: "",
        changeSummary: { changedCount: 0, deletedCount: 0 },
      },
      {
        path: "D:\\Code\\demo\\_worktrees\\missing",
        branch: "feature-missing",
        head: "33333333",
        isMain: false,
        isDetached: false,
        status: "missing",
        statusMessage: "目录不存在",
        changeSummary: { changedCount: 0, deletedCount: 0 },
      },
      {
        path: "D:\\Code\\demo\\_worktrees\\pending",
        branch: "feature-pending",
        head: "44444444",
        isMain: false,
        isDetached: false,
        status: "pending_cleanup",
        statusMessage: "目录占用",
        changeSummary: { changedCount: 0, deletedCount: 0 },
      },
    ],
    availableBranches: ["main", "feature-free", "feature-missing", "feature-pending"],
    suggestedRoot: "D:\\Code\\demo\\_worktrees",
  };
}

/**
 * describe worktree 纯逻辑测试。
 */
describe("worktrees", () => {
  /**
   * it 应优先使用主工作区当前分支作为 detached 创建默认来源。
   */
  it("会优先选择主工作区分支作为默认来源分支", () => {
    const view = createRepositoryView();

    expect(resolveDefaultCreateSourceBranch(view)).toBe("main");
  });

  /**
   * it 应在主工作区为 detached 时回退到本地分支列表首项。
   */
  it("会在主工作区也是 detached 时回退到首个本地分支", () => {
    const view = createRepositoryView();
    view.worktrees[0] = {
      ...view.worktrees[0],
      branch: "detached",
      isDetached: true,
    };

    expect(resolveDefaultCreateSourceBranch(view)).toBe("main");
  });

  /**
   * it 应返回创建弹窗各模式的分支字段文案。
   */
  it("会返回创建模式对应的分支字段标题", () => {
    expect(resolveCreateBranchFieldLabel("detached")).toBe("来源分支");
    expect(resolveCreateBranchFieldLabel("existing")).toBe("现有分支");
    expect(resolveCreateBranchFieldLabel("new")).toBe("基线分支");
  });

  /**
   * it 应把已被其他 worktree 占用的分支标记为禁用，并忽略 pending_cleanup。
   */
  it("会正确标记可切换与不可切换的现有分支", () => {
    const view = createRepositoryView();

    const options = buildAttachBranchOptions(view, "D:\\Code\\demo\\_worktrees\\detached");

    expect(options).toEqual([
      { branch: "main", disabled: true, occupiedPath: "D:\\Code\\demo" },
      { branch: "feature-free", disabled: false, occupiedPath: "" },
      { branch: "feature-missing", disabled: true, occupiedPath: "D:\\Code\\demo\\_worktrees\\missing" },
      { branch: "feature-pending", disabled: false, occupiedPath: "" },
    ]);
    expect(pickDefaultAttachExistingBranch(options)).toBe("feature-free");
  });
});
