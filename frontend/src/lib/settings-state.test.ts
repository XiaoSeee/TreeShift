import {
  syncSettingsDraftWithIncomingSettings,
  toSettingsDraft,
} from "./formats";
import {
  deriveWorkspaceSnapshotFromSettings,
  resolveSuggestedRootPath,
} from "./settings-state";
import type {
  RepositoryBinding,
  RepositorySummary,
  RepositoryView,
  Settings,
} from "../types";

/**
 * createRepositoryBinding 创建测试使用的仓库绑定。
 *
 * @param overrides 需要覆盖的字段。
 * @returns 一个结构完整的仓库绑定对象。
 */
function createRepositoryBinding(overrides: Partial<RepositoryBinding>): RepositoryBinding {
  return {
    id: overrides.id ?? "repo-1",
    displayName: overrides.displayName ?? "demo",
    selectedPath: overrides.selectedPath ?? "D:\\Code\\demo",
    mainWorktreePath: overrides.mainWorktreePath ?? "D:\\Code\\demo",
    commonDir: overrides.commonDir ?? "D:\\Code\\demo\\.git",
    defaultWorktreeRoot: overrides.defaultWorktreeRoot ?? "",
  };
}

/**
 * createSettings 创建测试使用的完整设置对象。
 *
 * @param overrides 需要覆盖的字段。
 * @returns 一个结构完整的设置对象。
 */
function createSettings(overrides: Partial<Settings> = {}): Settings {
  return {
    schemaVersion: overrides.schemaVersion ?? 3,
    repositories: overrides.repositories ?? [
      createRepositoryBinding({ id: "repo-1", displayName: "alpha" }),
      createRepositoryBinding({
        id: "repo-2",
        displayName: "beta",
        selectedPath: "D:\\Code\\beta",
        mainWorktreePath: "D:\\Code\\beta",
        commonDir: "D:\\Code\\beta\\.git",
      }),
    ],
    defaultWorktreeRoot: overrides.defaultWorktreeRoot ?? "D:\\Worktrees",
    externalTools: overrides.externalTools ?? [],
    launchScript: overrides.launchScript ?? {
      powerShellScript: "",
      applyToTerminal: false,
      applyToExternalTools: false,
    },
    pendingCleanups: overrides.pendingCleanups ?? [],
    uiPreferences: overrides.uiPreferences ?? {
      lastSelectedRepositoryId: "repo-1",
    },
  };
}

/**
 * createRepositorySummaries 创建测试使用的仓库摘要列表。
 *
 * @returns 一组带有 worktree 数量的仓库摘要。
 */
function createRepositorySummaries(): RepositorySummary[] {
  return [
    {
      id: "repo-1",
      displayName: "alpha",
      mainWorktreePath: "D:\\Code\\alpha",
      defaultWorktreeRoot: "",
      worktreeCount: 3,
      pendingCleanupCount: 0,
    },
    {
      id: "repo-2",
      displayName: "beta",
      mainWorktreePath: "D:\\Code\\beta",
      defaultWorktreeRoot: "",
      worktreeCount: 1,
      pendingCleanupCount: 1,
    },
  ];
}

/**
 * createRepositoryView 创建测试使用的仓库视图。
 *
 * @returns 一个最小可用的仓库视图对象。
 */
function createRepositoryView(): RepositoryView {
  return {
    repository: createRepositoryBinding({
      id: "repo-1",
      displayName: "alpha",
      selectedPath: "D:\\Code\\alpha",
      mainWorktreePath: "D:\\Code\\alpha",
      commonDir: "D:\\Code\\alpha\\.git",
      defaultWorktreeRoot: "",
    }),
    worktrees: [
      {
        path: "D:\\Code\\alpha",
        branch: "main",
        head: "11111111",
        isMain: true,
        isDetached: false,
        isLocked: false,
        lockReason: "",
        status: "normal",
        statusMessage: "",
        changeSummary: {
          changedCount: 0,
          deletedCount: 0,
        },
      },
    ],
    availableBranches: ["main"],
    suggestedRoot: "D:\\Worktrees\\alpha",
  };
}

/**
 * describe 验证设置页相关前端状态推导逻辑。
 */
describe("settings-state", () => {
  /**
   * it 应在保存成功后用最新设置修补本地摘要和当前仓库视图。
   */
  it("会在刷新失败时保留最新保存的仓库配置快照", () => {
    const nextSettings = createSettings({
      repositories: [
        createRepositoryBinding({
          id: "repo-1",
          displayName: "alpha-renamed",
          selectedPath: "D:\\Code\\alpha",
          mainWorktreePath: "D:\\Code\\alpha",
          commonDir: "D:\\Code\\alpha\\.git",
          defaultWorktreeRoot: "D:\\Pinned\\alpha",
        }),
        createRepositoryBinding({
          id: "repo-2",
          displayName: "beta",
          selectedPath: "D:\\Code\\beta",
          mainWorktreePath: "D:\\Code\\beta",
          commonDir: "D:\\Code\\beta\\.git",
          defaultWorktreeRoot: "",
        }),
      ],
      defaultWorktreeRoot: "D:\\GlobalWorktrees",
      pendingCleanups: [
        {
          repositoryId: "repo-2",
          path: "D:\\Code\\beta\\_worktrees\\feature-a",
          branch: "feature-a",
          head: "22222222",
          lastError: "目录被占用",
        },
      ],
      uiPreferences: {
        lastSelectedRepositoryId: "repo-1",
      },
    });

    const snapshot = deriveWorkspaceSnapshotFromSettings({
      currentRepositories: createRepositorySummaries(),
      currentRepositoryView: createRepositoryView(),
      nextSettings,
      preferredRepositoryId: "repo-1",
    });

    expect(snapshot.repositories).toEqual([
      {
        id: "repo-1",
        displayName: "alpha-renamed",
        mainWorktreePath: "D:\\Code\\alpha",
        defaultWorktreeRoot: "D:\\Pinned\\alpha",
        worktreeCount: 3,
        pendingCleanupCount: 0,
      },
      {
        id: "repo-2",
        displayName: "beta",
        mainWorktreePath: "D:\\Code\\beta",
        defaultWorktreeRoot: "",
        worktreeCount: 1,
        pendingCleanupCount: 1,
      },
    ]);
    expect(snapshot.activeRepositoryId).toBe("repo-1");
    expect(snapshot.repositoryView?.repository.displayName).toBe("alpha-renamed");
    expect(snapshot.repositoryView?.repository.defaultWorktreeRoot).toBe("D:\\Pinned\\alpha");
    expect(snapshot.repositoryView?.suggestedRoot).toBe("D:\\Pinned\\alpha");
  });

  /**
   * it 应在解绑成功后立即移除仓库并清空已过期的仓库视图。
   */
  it("会在解绑成功后移除仓库并切换活动仓库", () => {
    const nextSettings = createSettings({
      repositories: [
        createRepositoryBinding({
          id: "repo-2",
          displayName: "beta",
          selectedPath: "D:\\Code\\beta",
          mainWorktreePath: "D:\\Code\\beta",
          commonDir: "D:\\Code\\beta\\.git",
          defaultWorktreeRoot: "",
        }),
      ],
      pendingCleanups: [],
      uiPreferences: {
        lastSelectedRepositoryId: "repo-2",
      },
    });

    const snapshot = deriveWorkspaceSnapshotFromSettings({
      currentRepositories: createRepositorySummaries(),
      currentRepositoryView: createRepositoryView(),
      nextSettings,
      preferredRepositoryId: "repo-1",
    });

    expect(snapshot.repositories).toEqual([
      {
        id: "repo-2",
        displayName: "beta",
        mainWorktreePath: "D:\\Code\\beta",
        defaultWorktreeRoot: "",
        worktreeCount: 1,
        pendingCleanupCount: 0,
      },
    ]);
    expect(snapshot.activeRepositoryId).toBe("repo-2");
    expect(snapshot.repositoryView).toBeNull();
  });

  /**
   * it 应复用后端推荐 worktree 根目录规则。
   */
  it("会按仓库优先级推导推荐的 worktree 根目录", () => {
    expect(resolveSuggestedRootPath(
      createRepositoryBinding({
        displayName: "alpha",
        defaultWorktreeRoot: "D:\\Pinned\\alpha",
      }),
      "D:\\GlobalWorktrees",
    )).toBe("D:\\Pinned\\alpha");

    expect(resolveSuggestedRootPath(
      createRepositoryBinding({
        displayName: "alpha",
        defaultWorktreeRoot: "",
        mainWorktreePath: "D:\\Code\\alpha",
      }),
      "D:\\GlobalWorktrees",
    )).toBe("D:\\GlobalWorktrees");
  });
});

/**
 * describe 验证设置页草稿在外部配置变化时的同步规则。
 */
describe("settings-draft-sync", () => {
  /**
   * it 应在页面没有未保存修改时整体切到最新设置。
   */
  it("会在草稿未变脏时直接覆盖草稿和基线", () => {
    const currentSettings = createSettings();
    const nextSettings = createSettings({
      defaultWorktreeRoot: "D:\\NextWorktrees",
      repositories: [
        createRepositoryBinding({
          id: "repo-1",
          displayName: "alpha-updated",
          selectedPath: "D:\\Code\\alpha",
          mainWorktreePath: "D:\\Code\\alpha",
          commonDir: "D:\\Code\\alpha\\.git",
        }),
      ],
    });
    const baselineDraft = toSettingsDraft(currentSettings);

    const result = syncSettingsDraftWithIncomingSettings(baselineDraft, baselineDraft, nextSettings);

    expect(result.preservedUnsavedChanges).toBe(false);
    expect(result.baselineDraft).toEqual(toSettingsDraft(nextSettings));
    expect(result.draft).toEqual(toSettingsDraft(nextSettings));
  });

  /**
   * it 应在页面已有未保存草稿时只更新基线，不覆盖当前输入。
   */
  it("会在草稿已变脏时保留当前输入", () => {
    const currentSettings = createSettings();
    const nextSettings = createSettings({
      repositories: [
        createRepositoryBinding({
          id: "repo-1",
          displayName: "alpha",
          selectedPath: "D:\\Code\\alpha",
          mainWorktreePath: "D:\\Code\\alpha",
          commonDir: "D:\\Code\\alpha\\.git",
        }),
        createRepositoryBinding({
          id: "repo-2",
          displayName: "beta",
          selectedPath: "D:\\Code\\beta",
          mainWorktreePath: "D:\\Code\\beta",
          commonDir: "D:\\Code\\beta\\.git",
        }),
        createRepositoryBinding({
          id: "repo-3",
          displayName: "gamma",
          selectedPath: "D:\\Code\\gamma",
          mainWorktreePath: "D:\\Code\\gamma",
          commonDir: "D:\\Code\\gamma\\.git",
        }),
      ],
    });
    const baselineDraft = toSettingsDraft(currentSettings);
    const dirtyDraft = {
      ...baselineDraft,
      defaultWorktreeRoot: "D:\\DraftOnly",
    };

    const result = syncSettingsDraftWithIncomingSettings(dirtyDraft, baselineDraft, nextSettings);

    expect(result.preservedUnsavedChanges).toBe(true);
    expect(result.baselineDraft).toEqual(toSettingsDraft(nextSettings));
    expect(result.draft).toEqual(dirtyDraft);
  });
});
