import {
  areSettingsDraftsEqual,
  fromSettingsDraft,
  argsToText,
  buildSuggestedPath,
  ensureRepositoryDisplayName,
  sanitizePathSegment,
  textToArgs,
  toSettingsDraft,
} from "./formats";
import type { Settings } from "../types";

/**
 * describe 用于覆盖设置草稿和路径格式化相关的工具函数。
 */
describe("formats", () => {
  /**
   * it 应把分支名转换为可用目录名。
   */
  it("会把分支名清洗为安全路径片段", () => {
    expect(sanitizePathSegment("feature/login fix")).toBe("feature-login-fix");
  });

  /**
   * it 应基于根目录和分支名生成建议路径。
   */
  it("会生成建议 worktree 路径", () => {
    expect(buildSuggestedPath("D:\\Worktrees", "feature/login")).toBe("D:\\Worktrees\\feature-login");
  });

  /**
   * it 应在文本与数组之间稳定转换工具参数。
   */
  it("会在参数文本与数组之间相互转换", () => {
    const rawText = "--model\n gpt-5 \n\n--approval-mode\nnever";
    expect(textToArgs(rawText)).toEqual(["--model", "gpt-5", "--approval-mode", "never"]);
    expect(argsToText(["--model", "gpt-5"])).toBe("--model\ngpt-5");
  });

  /**
   * it 应在设置与草稿之间保留启动脚本配置。
   */
  it("会在设置草稿转换中保留启动脚本配置", () => {
    const settings: Settings = {
      schemaVersion: 3,
      repositories: [],
      defaultWorktreeRoot: "D:\\Worktrees",
      externalTools: [
        {
          id: "tool-codex",
          name: "Codex CLI",
          command: "codex",
          args: ["--model", "gpt-5"],
          enabled: true,
        },
      ],
      launchScript: {
        powerShellScript: '$env:HTTP_PROXY="http://127.0.0.1:6789"',
        applyToTerminal: true,
        applyToExternalTools: true,
      },
      pendingCleanups: [],
      uiPreferences: {
        lastSelectedRepositoryId: "",
      },
    };

    const draft = toSettingsDraft(settings);
    expect(draft.launchScript).toEqual(settings.launchScript);
    expect(draft.externalTools[0].argsText).toBe("--model\ngpt-5");

    const restored = fromSettingsDraft(draft);
    expect(restored.launchScript).toEqual(settings.launchScript);
    expect(restored.externalTools[0].args).toEqual(["--model", "gpt-5"]);
  });

  /**
   * it 应能判断设置草稿是否已经发生未保存改动。
   */
  it("会识别设置草稿是否存在未保存修改", () => {
    const settings: Settings = {
      schemaVersion: 3,
      repositories: [],
      defaultWorktreeRoot: "D:\\Worktrees",
      externalTools: [],
      launchScript: {
        powerShellScript: "",
        applyToTerminal: false,
        applyToExternalTools: false,
      },
      pendingCleanups: [],
      uiPreferences: {
        lastSelectedRepositoryId: "",
      },
    };

    const baseline = toSettingsDraft(settings);
    const changedDraft = {
      ...baseline,
      defaultWorktreeRoot: "D:\\OtherWorktrees",
    };

    expect(areSettingsDraftsEqual(baseline, toSettingsDraft(settings))).toBe(true);
    expect(areSettingsDraftsEqual(baseline, changedDraft)).toBe(false);
  });

  /**
   * it 应在仓库显示名为空时回退到主工作区目录名。
   */
  it("会为缺失的仓库显示名补齐默认值", () => {
    const repository = ensureRepositoryDisplayName({
      id: "repo-1",
      displayName: "   ",
      selectedPath: "D:\\Code\\example",
      mainWorktreePath: "D:\\Code\\example",
      commonDir: "D:\\Code\\example\\.git",
      defaultWorktreeRoot: "",
    });

    expect(repository.displayName).toBe("example");
  });
});
