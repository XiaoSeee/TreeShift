import { useEffect, useState } from "react";

import { backend } from "../lib/backend";
import { toErrorMessage } from "../lib/errors";
import {
  areSettingsDraftsEqual,
  createEmptyToolDraft,
  ensureRepositoryDisplayName,
  fromSettingsDraft,
  syncSettingsDraftWithIncomingSettings,
  toSettingsDraft,
  type SettingsDraft,
} from "../lib/formats";
import type { Settings } from "../types";

/**
 * SettingsSection 定义设置页左侧分组导航可切换的内容区域。
 */
type SettingsSection = "general" | "launchScript" | "repositories" | "externalTools";

/**
 * SettingsSectionMeta 描述单个设置分组的标题。
 */
interface SettingsSectionMeta {
  id: SettingsSection;
  title: string;
}

/**
 * SettingsPageProps 定义设置页组件对外暴露的交互能力。
 */
interface SettingsPageProps {
  settings: Settings;
  onBack: () => void;
  onSave: (settings: Settings) => Promise<Settings>;
  onUnbindRepository: (repositoryId: string, displayName: string) => Promise<Settings | null>;
  onNotice: (tone: "success" | "error" | "warning", message: string) => void;
}

/**
 * settingsSections 固定描述设置页左侧导航顺序。
 *
 * 这里保持分组顺序稳定，避免用户每次进入设置页都要重新寻找内容位置。
 */
const settingsSections: SettingsSectionMeta[] = [
  {
    id: "general",
    title: "常规",
  },
  {
    id: "launchScript",
    title: "启动脚本",
  },
  {
    id: "repositories",
    title: "仓库管理",
  },
  {
    id: "externalTools",
    title: "外部工具",
  },
];

/**
 * SettingsPage 承载 TreeShift 的完整设置中心界面。
 *
 * 组件内部维护编辑草稿、分组切换和未保存提醒，
 * 但真正持久化配置仍通过父级传入的回调统一落到应用层。
 */
export function SettingsPage({
  settings,
  onBack,
  onSave,
  onUnbindRepository,
  onNotice,
}: SettingsPageProps) {
  const [activeSection, setActiveSection] = useState<SettingsSection>("general");
  const [baselineDraft, setBaselineDraft] = useState<SettingsDraft>(() => toSettingsDraft(settings));
  const [draft, setDraft] = useState<SettingsDraft>(() => toSettingsDraft(settings));
  const [saving, setSaving] = useState(false);
  const [unbindingRepositoryId, setUnbindingRepositoryId] = useState("");

  /**
   * 当父级提供的新配置发生变化时，重置当前基线与编辑草稿。
   *
   * 设置页只会在“刚进入页面”或“某次保存/解绑成功后”收到新配置，
   * 这时应以最新持久化结果覆盖本地编辑态。
   */
  useEffect(() => {
    const nextDraftState = syncSettingsDraftWithIncomingSettings(draft, baselineDraft, settings);
    setBaselineDraft(nextDraftState.baselineDraft);
    setDraft(nextDraftState.draft);
  }, [settings]);

  const pageBusy = saving || unbindingRepositoryId !== "";
  const hasUnsavedChanges = !areSettingsDraftsEqual(draft, baselineDraft);

  /**
   * updateRepositoryDraft 统一更新单个仓库草稿项。
   *
   * @param repositoryId 需要更新的仓库 ID。
   * @param updater 基于当前仓库草稿返回新草稿的函数。
   */
  function updateRepositoryDraft(
    repositoryId: string,
    updater: (current: SettingsDraft["repositories"][number]) => SettingsDraft["repositories"][number],
  ) {
    setDraft((current) => ({
      ...current,
      repositories: current.repositories.map((repository) =>
        repository.id === repositoryId
          ? updater(repository)
          : repository,
      ),
    }));
  }

  /**
   * updateToolDraft 统一更新单个外部工具草稿项。
   *
   * @param toolId 需要更新的工具 ID。
   * @param updater 基于当前工具草稿返回新草稿的函数。
   */
  function updateToolDraft(
    toolId: string,
    updater: (current: SettingsDraft["externalTools"][number]) => SettingsDraft["externalTools"][number],
  ) {
    setDraft((current) => ({
      ...current,
      externalTools: current.externalTools.map((tool) =>
        tool.id === toolId
          ? updater(tool)
          : tool,
      ),
    }));
  }

  /**
   * syncDraftWithSavedSettings 用最新持久化配置覆盖当前草稿基线。
   *
   * @param nextSettings 已经被后端接受并归一化后的最新配置。
   */
  function syncDraftWithSavedSettings(nextSettings: Settings) {
    const nextBaseline = toSettingsDraft(nextSettings);
    setBaselineDraft(nextBaseline);
    setDraft(nextBaseline);
  }

  /**
   * renderSectionHeading 渲染右侧内容区的分组标题与说明。
   *
   * @param title 当前分组标题。
   * @param description 当前分组的简短说明。
   * @returns 分组标题片段。
   */
  function renderSectionHeading(title: string, description: string) {
    return (
      <div className="space-y-1">
        <h2 className="text-xl font-semibold text-stone-900">{title}</h2>
        <p className="max-w-2xl text-sm leading-6 text-stone-600">{description}</p>
      </div>
    );
  }

  /**
   * handleBack 尝试离开设置页并返回工作区。
   *
   * 若当前仍有未保存修改，则先弹出确认，避免用户误丢草稿。
   */
  function handleBack() {
    if (pageBusy) {
      return;
    }

    if (!hasUnsavedChanges) {
      onBack();
      return;
    }

    const accepted = window.confirm("当前还有未保存修改，确定要返回工作区并放弃这些修改吗？");
    if (accepted) {
      onBack();
    }
  }

  /**
   * handleDiscardChanges 放弃本轮编辑草稿并恢复到最近一次保存状态。
   */
  function handleDiscardChanges() {
    if (!hasUnsavedChanges || pageBusy) {
      return;
    }

    const accepted = window.confirm("确定要放弃当前未保存的修改吗？");
    if (accepted) {
      setDraft(baselineDraft);
    }
  }

  /**
   * handleSave 持久化当前设置草稿。
   *
   * 保存前会补齐空仓库名称，确保后端收到的仍是完整配置结构。
   */
  async function handleSave() {
    if (!hasUnsavedChanges || pageBusy) {
      return;
    }

    setSaving(true);
    try {
      const normalizedDraft: SettingsDraft = {
        ...draft,
        repositories: draft.repositories.map((repository) => ensureRepositoryDisplayName(repository)),
      };
      const savedSettings = await onSave(fromSettingsDraft(normalizedDraft));
      syncDraftWithSavedSettings(savedSettings);
    } catch {
      return;
    } finally {
      setSaving(false);
    }
  }

  /**
   * handleBrowseForGlobalDefaultRoot 打开目录选择器并回填全局默认路径。
   */
  async function handleBrowseForGlobalDefaultRoot() {
    try {
      const selectedPath = await backend.chooseDirectory({
        title: "选择全局默认 Worktree 根目录",
        defaultPath: draft.defaultWorktreeRoot,
      });

      if (selectedPath) {
        setDraft((current) => ({
          ...current,
          defaultWorktreeRoot: selectedPath,
        }));
      }
    } catch (error) {
      onNotice("error", toErrorMessage(error));
    }
  }

  /**
   * handleBrowseForRepositoryRoot 打开目录选择器并回填仓库级默认路径。
   *
   * @param repositoryId 需要修改的仓库 ID。
   * @param defaultPath 当前仓库草稿中的默认路径。
   */
  async function handleBrowseForRepositoryRoot(repositoryId: string, defaultPath: string) {
    try {
      const selectedPath = await backend.chooseDirectory({
        title: "选择仓库默认 Worktree 根目录",
        defaultPath,
      });

      if (selectedPath) {
        updateRepositoryDraft(repositoryId, (repository) => ({
          ...repository,
          defaultWorktreeRoot: selectedPath,
        }));
      }
    } catch (error) {
      onNotice("error", toErrorMessage(error));
    }
  }

  /**
   * handleUnbindRepositoryClick 执行仓库解绑，并在成功后刷新本地草稿。
   *
   * 若当前还存在其他未保存修改，会先提示这次即时解绑将覆盖草稿，
   * 避免用户误以为解绑只是本地临时操作。
   *
   * @param repositoryId 需要解绑的仓库 ID。
   * @param displayName 用于提示文案的仓库显示名。
   */
  async function handleUnbindRepositoryClick(repositoryId: string, displayName: string) {
    if (pageBusy) {
      return;
    }

    if (hasUnsavedChanges) {
      const accepted = window.confirm("当前还有未保存的其他修改。继续解绑会刷新设置页并丢弃这些草稿，是否继续？");
      if (!accepted) {
        return;
      }
    }

    setUnbindingRepositoryId(repositoryId);
    try {
      const nextSettings = await onUnbindRepository(repositoryId, displayName);
      if (nextSettings) {
        syncDraftWithSavedSettings(nextSettings);
      }
    } catch {
      return;
    } finally {
      setUnbindingRepositoryId("");
    }
  }

  /**
   * renderGeneralSection 渲染“常规”分组内容。
   *
   * @returns 常规设置表单片段。
   */
  function renderGeneralSection() {
    return (
      <section className="space-y-6">
        {renderSectionHeading(
          "常规",
          "控制默认 worktree 路径，不会改写 Git 仓库结构。",
        )}

        <div className="space-y-4 border-t border-stone-200/80 pt-5">
          <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
            <div className="space-y-1">
              <h3 className="text-sm font-medium text-stone-700">全局默认路径</h3>
              <p className="text-xs leading-5 text-stone-500">
                留空时，创建 Worktree 会继续使用仓库级默认路径或当前仓库的推荐目录。
              </p>
            </div>
            <button
              className="ghost-button px-4 py-2 text-xs"
              disabled={pageBusy}
              onClick={() => void handleBrowseForGlobalDefaultRoot()}
              type="button"
            >
              浏览目录
            </button>
          </div>

          <label className="flex flex-col gap-2.5">
            <span className="text-sm font-medium text-stone-700">Worktree 根目录</span>
            <input
              className="field-shell w-full"
              disabled={pageBusy}
              onChange={(event) =>
                setDraft((current) => ({
                  ...current,
                  defaultWorktreeRoot: event.target.value,
                }))
              }
              placeholder="例如：D:\Worktrees"
              value={draft.defaultWorktreeRoot}
            />
          </label>
        </div>
      </section>
    );
  }

  /**
   * renderLaunchScriptSection 渲染“启动脚本”分组内容。
   *
   * @returns 启动脚本设置表单片段。
   */
  function renderLaunchScriptSection() {
    return (
      <section className="space-y-6">
        {renderSectionHeading(
          "启动脚本",
          "在打开终端或启动外部 CLI 前执行 PowerShell 脚本。",
        )}

        <div className="space-y-4 border-t border-stone-200/80 pt-5">
          <label className="flex flex-col gap-2.5">
            <span className="text-sm font-medium text-stone-700">PowerShell 脚本</span>
            <textarea
              className="field-shell min-h-[180px] w-full resize-y font-mono text-[12px]"
              disabled={pageBusy}
              onChange={(event) =>
                setDraft((current) => ({
                  ...current,
                  launchScript: {
                    ...current.launchScript,
                    powerShellScript: event.target.value,
                  },
                }))
              }
              placeholder={'$env:HTTP_PROXY="http://127.0.0.1:6789"\n$env:HTTPS_PROXY="http://127.0.0.1:6789"'}
              value={draft.launchScript.powerShellScript}
            />
          </label>
        </div>

        <div className="space-y-3 border-t border-stone-200/80 pt-5">
          <span className="text-sm font-medium text-stone-700">应用范围</span>

          <div className="flex flex-col gap-3">
            <label className="flex items-center gap-2 text-sm text-stone-700">
              <input
                checked={draft.launchScript.applyToTerminal}
                className="h-4 w-4 rounded border-stone-300 text-ink-900 focus:ring-ember-200"
                disabled={pageBusy}
                onChange={(event) =>
                  setDraft((current) => ({
                    ...current,
                    launchScript: {
                      ...current.launchScript,
                      applyToTerminal: event.target.checked,
                    },
                  }))
                }
                type="checkbox"
              />
              <span>用于打开终端</span>
            </label>

            <label className="flex items-center gap-2 text-sm text-stone-700">
              <input
                checked={draft.launchScript.applyToExternalTools}
                className="h-4 w-4 rounded border-stone-300 text-ink-900 focus:ring-ember-200"
                disabled={pageBusy}
                onChange={(event) =>
                  setDraft((current) => ({
                    ...current,
                    launchScript: {
                      ...current.launchScript,
                      applyToExternalTools: event.target.checked,
                    },
                  }))
                }
                type="checkbox"
              />
              <span>用于启动外部 CLI</span>
            </label>
          </div>
        </div>

        <div className="space-y-1 border-l-2 border-amber-300/90 pl-4 text-xs leading-5 text-amber-900">
          <p>启用“用于打开终端”后，该入口会固定使用 PowerShell，而不是 Windows Terminal 的默认 Profile。</p>
          <p>启用“用于启动外部 CLI”后，会先执行这段脚本；脚本报错或返回非零退出码时，不会继续启动 CLI。</p>
        </div>
      </section>
    );
  }

  /**
   * renderRepositoriesSection 渲染“仓库管理”分组内容。
   *
   * @returns 仓库管理设置表单片段。
   */
  function renderRepositoriesSection() {
    return (
      <section className="space-y-6">
        {renderSectionHeading(
          "仓库管理",
          "调整绑定仓库名称、默认路径和解绑。",
        )}

        {draft.repositories.length === 0 ? (
          <div className="border-t border-dashed border-stone-300/80 py-6 text-sm text-stone-600">
            当前还没有已绑定仓库。返回工作区后可以先绑定一个主 Git 仓库。
          </div>
        ) : (
          <div className="divide-y divide-stone-200/80 border-t border-stone-200/80">
            {draft.repositories.map((repository) => (
              <div key={repository.id} className="space-y-4 py-5 first:pt-0 last:pb-0">
                <div className="space-y-1">
                  <h3 className="text-base font-semibold text-stone-900">{ensureRepositoryDisplayName(repository).displayName}</h3>
                  <p className="break-all text-xs leading-5 text-stone-500">
                    主工作区：{repository.mainWorktreePath}
                  </p>
                </div>

                <div className="grid gap-4 xl:grid-cols-2">
                  <label className="flex flex-col gap-2.5">
                    <span className="text-sm font-medium text-stone-700">显示名称</span>
                    <input
                      className="field-shell w-full"
                      disabled={pageBusy}
                      onChange={(event) =>
                        updateRepositoryDraft(repository.id, (current) => ({
                          ...current,
                          displayName: event.target.value,
                        }))
                      }
                      value={repository.displayName}
                    />
                  </label>

                  <label className="flex flex-col gap-2.5">
                    <span className="text-sm font-medium text-stone-700">仓库默认 Worktree 根目录</span>
                    <input
                      className="field-shell w-full"
                      disabled={pageBusy}
                      onChange={(event) =>
                        updateRepositoryDraft(repository.id, (current) => ({
                          ...current,
                          defaultWorktreeRoot: event.target.value,
                        }))
                      }
                      placeholder="留空则回退到全局默认路径"
                      value={repository.defaultWorktreeRoot}
                    />
                  </label>
                </div>

                <div className="flex flex-wrap gap-2">
                  <button
                    className="ghost-button px-4 py-2 text-xs"
                    disabled={pageBusy}
                    onClick={() => void handleBrowseForRepositoryRoot(repository.id, repository.defaultWorktreeRoot)}
                    type="button"
                  >
                    浏览目录
                  </button>
                  <button
                    className="ghost-button border-rose-300/80 px-4 py-2 text-xs text-rose-700 hover:border-rose-400 hover:bg-rose-50 hover:text-rose-800"
                    disabled={pageBusy}
                    onClick={() => void handleUnbindRepositoryClick(repository.id, repository.displayName)}
                    type="button"
                  >
                    {unbindingRepositoryId === repository.id ? "解绑中…" : "解除绑定"}
                  </button>
                </div>
              </div>
            ))}
          </div>
        )}
      </section>
    );
  }

  /**
   * renderExternalToolsSection 渲染“外部工具”分组内容。
   *
   * @returns 外部工具设置表单片段。
   */
  function renderExternalToolsSection() {
    return (
      <section className="space-y-6">
        <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
          {renderSectionHeading(
            "外部工具",
            "配置 worktree 卡片上的外部 CLI。",
          )}

          <button
            className="ghost-button px-4 py-2 text-xs sm:mt-0.5"
            disabled={pageBusy}
            onClick={() =>
              setDraft((current) => ({
                ...current,
                externalTools: [...current.externalTools, createEmptyToolDraft()],
              }))
            }
            type="button"
          >
            添加工具
          </button>
        </div>

        {draft.externalTools.length === 0 ? (
          <div className="border-t border-dashed border-stone-300/80 py-6 text-sm text-stone-600">
            当前还没有可直接启动的外部工具模板。你可以先添加一个，例如 Codex CLI。
          </div>
        ) : (
          <div className="divide-y divide-stone-200/80 border-t border-stone-200/80">
            {draft.externalTools.map((tool) => (
              <div key={tool.id} className="space-y-4 py-5 first:pt-0 last:pb-0">
                <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
                  <div className="space-y-1">
                    <h3 className="text-base font-semibold text-stone-900">{tool.name.trim() || "未命名工具"}</h3>
                    <p className="text-xs leading-5 text-stone-500">
                      命令和参数会按“命令 + 参数数组”的方式传给后端，不依赖 shell 拼接。
                    </p>
                  </div>

                  <div className="flex flex-wrap items-center gap-3">
                    <label className="flex items-center gap-2 text-sm text-stone-700">
                      <input
                        checked={tool.enabled}
                        className="h-4 w-4 rounded border-stone-300 text-ink-900 focus:ring-ember-200"
                        disabled={pageBusy}
                        onChange={(event) =>
                          updateToolDraft(tool.id, (current) => ({
                            ...current,
                            enabled: event.target.checked,
                          }))
                        }
                        type="checkbox"
                      />
                      <span>启用</span>
                    </label>

                    <button
                      className="ghost-button border-rose-300/80 px-4 py-2 text-xs text-rose-700 hover:border-rose-400 hover:bg-rose-50 hover:text-rose-800"
                      disabled={pageBusy}
                      onClick={() =>
                        setDraft((current) => ({
                          ...current,
                          externalTools: current.externalTools.filter((item) => item.id !== tool.id),
                        }))
                      }
                      type="button"
                    >
                      移除工具
                    </button>
                  </div>
                </div>

                <div className="grid gap-4 xl:grid-cols-2">
                  <label className="flex flex-col gap-2.5">
                    <span className="text-sm font-medium text-stone-700">工具名称</span>
                    <input
                      className="field-shell w-full"
                      disabled={pageBusy}
                      onChange={(event) =>
                        updateToolDraft(tool.id, (current) => ({
                          ...current,
                          name: event.target.value,
                        }))
                      }
                      placeholder="例如：Codex CLI"
                      value={tool.name}
                    />
                  </label>

                  <label className="flex flex-col gap-2.5">
                    <span className="text-sm font-medium text-stone-700">命令</span>
                    <input
                      className="field-shell w-full"
                      disabled={pageBusy}
                      onChange={(event) =>
                        updateToolDraft(tool.id, (current) => ({
                          ...current,
                          command: event.target.value,
                        }))
                      }
                      placeholder="例如：codex 或 C:\Tools\codex.exe"
                      value={tool.command}
                    />
                  </label>
                </div>

                <label className="flex flex-col gap-2.5">
                  <span className="text-sm font-medium text-stone-700">参数（每行一个）</span>
                  <textarea
                    className="field-shell min-h-[140px] w-full resize-y"
                    disabled={pageBusy}
                    onChange={(event) =>
                      updateToolDraft(tool.id, (current) => ({
                        ...current,
                        argsText: event.target.value,
                      }))
                    }
                    placeholder={"例如：\n--model\ngpt-5\n--cwd\n{path}"}
                    value={tool.argsText}
                  />
                </label>
              </div>
            ))}
          </div>
        )}
      </section>
    );
  }

  /**
   * renderSectionContent 根据当前分组渲染对应设置内容。
   *
   * @returns 当前激活分组的 JSX 片段。
   */
  function renderSectionContent() {
    switch (activeSection) {
      case "general":
        return renderGeneralSection();
      case "launchScript":
        return renderLaunchScriptSection();
      case "repositories":
        return renderRepositoriesSection();
      case "externalTools":
        return renderExternalToolsSection();
      default:
        return renderGeneralSection();
    }
  }

  return (
    <section className="flex min-h-0 flex-1 flex-col overflow-hidden">
      <div className="shrink-0 border-b border-stone-200/80 pb-3">
        <div className="flex flex-wrap items-center justify-between gap-2">
          <button className="ghost-button px-4 py-2 text-xs" disabled={pageBusy} onClick={handleBack} type="button">
            返回
          </button>

          <div className="flex flex-wrap items-center gap-2">
            <button
              className="ghost-button px-4 py-2 text-xs"
              disabled={!hasUnsavedChanges || pageBusy}
              onClick={handleDiscardChanges}
              type="button"
            >
              放弃
            </button>

            <button
              className="primary-button px-4 py-2 text-xs"
              disabled={!hasUnsavedChanges || pageBusy}
              onClick={() => void handleSave()}
              type="button"
            >
              {saving ? "保存中…" : "保存"}
            </button>
          </div>
        </div>
      </div>

      <div className="min-h-0 flex flex-1 flex-col gap-4 pt-4 sm:flex-row">
        <aside className="no-scrollbar min-h-0 shrink-0 overflow-x-auto border-b border-stone-200/80 pb-4 sm:w-[188px] sm:overflow-x-visible sm:overflow-y-auto sm:border-b-0 sm:border-r sm:pb-0 sm:pr-4">
          <nav className="flex gap-1.5 sm:flex-col">
            {settingsSections.map((section) => (
              <button
                key={section.id}
                className={`min-w-[112px] shrink-0 border-b-2 px-1 py-2.5 text-left text-sm font-medium transition sm:min-w-0 sm:border-b-0 sm:border-l-2 sm:px-0 sm:pl-3 ${
                  activeSection === section.id
                    ? "border-stone-900 text-stone-900"
                    : "border-transparent text-stone-500 hover:border-stone-300 hover:text-stone-900"
                }`}
                disabled={pageBusy}
                onClick={() => setActiveSection(section.id)}
                type="button"
              >
                {section.title}
              </button>
            ))}
          </nav>
        </aside>

        <div className="no-scrollbar min-h-0 flex-1 overflow-y-auto sm:pl-2">
          <div className="w-full max-w-[980px]">
            {renderSectionContent()}
          </div>
        </div>
      </div>
    </section>
  );
}
