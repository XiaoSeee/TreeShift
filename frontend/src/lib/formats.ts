import type { ExternalTool, RepositoryBinding, Settings } from "../types";

/**
 * SettingsToolDraft 是设置弹窗中用于编辑工具参数文本的草稿结构。
 */
export interface SettingsToolDraft extends ExternalTool {
  argsText: string;
}

/**
 * SettingsDraft 是设置弹窗的编辑态模型。
 */
export interface SettingsDraft extends Omit<Settings, "externalTools"> {
  externalTools: SettingsToolDraft[];
}

/**
 * SettingsDraftSyncResult 描述父组件传入新设置时，设置页草稿应如何同步。
 *
 * 当用户还没有未保存修改时，基线和草稿都应直接切到最新设置；
 * 当用户已经处于脏状态时，只更新基线，保留当前草稿，避免静默覆盖。
 */
export interface SettingsDraftSyncResult {
  baselineDraft: SettingsDraft;
  draft: SettingsDraft;
  preservedUnsavedChanges: boolean;
}

/**
 * areSettingsDraftsEqual 判断两个设置草稿是否完全一致。
 *
 * 设置草稿只包含可序列化字段，因此这里直接使用 JSON 字符串比对，
 * 以便设置页快速判断“是否存在未保存修改”。
 *
 * @param left 第一个设置草稿。
 * @param right 第二个设置草稿。
 * @returns 两份草稿完全一致时返回 true，否则返回 false。
 */
export function areSettingsDraftsEqual(left: SettingsDraft, right: SettingsDraft): boolean {
  return JSON.stringify(left) === JSON.stringify(right);
}

/**
 * joinWindowsPath 按 Windows 风格拼接路径片段。
 */
export function joinWindowsPath(...parts: string[]): string {
  return parts
    .map((part) => part.trim())
    .filter(Boolean)
    .reduce((result, part, index) => {
      const normalized = part.replace(/[\\/]+/g, "\\").replace(/\\$/, "");
      if (index === 0) {
        return normalized;
      }

      return `${result}\\${normalized.replace(/^\\/, "")}`;
    }, "");
}

/**
 * buildSuggestedPath 根据建议根目录和分支名生成默认 worktree 路径。
 */
export function buildSuggestedPath(root: string, branchOrName: string): string {
  const safeName = sanitizePathSegment(branchOrName || "new-worktree");
  return joinWindowsPath(root, safeName);
}

/**
 * sanitizePathSegment 把分支名转换为更安全的目录名片段。
 */
export function sanitizePathSegment(value: string): string {
  return value
    .trim()
    .replace(/[<>:"|?*]/g, "-")
    .replace(/[\\/]+/g, "-")
    .replace(/\s+/g, "-")
    .replace(/-+/g, "-")
    .replace(/^-|-$/g, "") || "new-worktree";
}

/**
 * argsToText 把工具参数数组转换为设置弹窗可编辑的多行文本。
 */
export function argsToText(args: string[]): string {
  return args.join("\n");
}

/**
 * textToArgs 把设置弹窗中的多行参数文本转换为数组。
 */
export function textToArgs(value: string): string[] {
  return value
    .split(/\r?\n/g)
    .map((line) => line.trim())
    .filter(Boolean);
}

/**
 * toSettingsDraft 把后端配置转换为前端设置草稿。
 */
export function toSettingsDraft(settings: Settings): SettingsDraft {
  return {
    ...settings,
    externalTools: settings.externalTools.map((tool) => ({
      ...tool,
      argsText: argsToText(tool.args),
    })),
  };
}

/**
 * fromSettingsDraft 把设置草稿转换回后端配置结构。
 */
export function fromSettingsDraft(draft: SettingsDraft): Settings {
  return {
    ...draft,
    externalTools: draft.externalTools.map(({ argsText, ...tool }) => ({
      ...tool,
      args: textToArgs(argsText),
    })),
  };
}

/**
 * syncSettingsDraftWithIncomingSettings 根据最新设置决定是否覆盖本地草稿。
 *
 * @param currentDraft 当前正在编辑的草稿。
 * @param baselineDraft 当前草稿所基于的基线配置。
 * @param nextSettings 父组件最新传入的设置。
 * @returns 新的草稿同步结果。
 */
export function syncSettingsDraftWithIncomingSettings(
  currentDraft: SettingsDraft,
  baselineDraft: SettingsDraft,
  nextSettings: Settings,
): SettingsDraftSyncResult {
  const nextBaseline = toSettingsDraft(nextSettings);
  const hasUnsavedChanges = !areSettingsDraftsEqual(currentDraft, baselineDraft);

  if (hasUnsavedChanges) {
    return {
      baselineDraft: nextBaseline,
      draft: currentDraft,
      preservedUnsavedChanges: true,
    };
  }

  return {
    baselineDraft: nextBaseline,
    draft: nextBaseline,
    preservedUnsavedChanges: false,
  };
}

/**
 * createEmptyToolDraft 生成一个新的工具草稿行。
 */
export function createEmptyToolDraft(): SettingsToolDraft {
  return {
    id: typeof crypto !== "undefined" && "randomUUID" in crypto ? crypto.randomUUID() : `tool-${Date.now()}`,
    name: "",
    command: "",
    args: [],
    argsText: "",
    enabled: true,
  };
}

/**
 * ensureRepositoryDisplayName 确保仓库草稿始终有可展示名称。
 */
export function ensureRepositoryDisplayName(repository: RepositoryBinding): RepositoryBinding {
  if (repository.displayName.trim()) {
    return repository;
  }

  const parts = repository.mainWorktreePath.split(/[\\/]/g).filter(Boolean);
  const lastPart = parts.length > 0 ? parts[parts.length - 1] : "未命名仓库";
  return {
    ...repository,
    displayName: lastPart,
  };
}
