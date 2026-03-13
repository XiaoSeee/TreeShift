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
