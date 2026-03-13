import type { CSSProperties, ChangeEvent, FormEvent } from "react";
import { useEffect, useState } from "react";
import { OnFileDrop, OnFileDropOff } from "../wailsjs/runtime/runtime";

import { EnvironmentStrip } from "./components/EnvironmentStrip";
import { Modal } from "./components/Modal";
import { WorktreeCard } from "./components/WorktreeCard";
import { backend } from "./lib/backend";
import {
  buildSuggestedPath,
  createEmptyToolDraft,
  ensureRepositoryDisplayName,
  fromSettingsDraft,
  toSettingsDraft,
  type SettingsDraft,
} from "./lib/formats";
import type {
  CreateMode,
  EnvironmentStatus,
  RepositorySummary,
  RepositoryView,
  Settings,
  WorktreeInfo,
} from "./types";

/**
 * NoticeTone 定义顶部通知条的视觉语义。
 */
type NoticeTone = "success" | "error" | "warning";

/**
 * NoticeState 描述顶部通知条状态。
 */
interface NoticeState {
  tone: NoticeTone;
  message: string;
}

/**
 * noticeAutoDismissDelayMs 定义顶部提示条的自动消失时长。
 */
const noticeAutoDismissDelayMs = 5000;

/**
 * wailsDropTargetStyle 为 Wails 原生文件拖拽提供命中目标标记。
 *
 * Wails 会通过该 CSS 自定义属性识别可接收文件拖拽的区域，
 * 并在拖入时自动为目标元素追加高亮类名。
 */
const wailsDropTargetStyle = {
  "--wails-drop-target": "drop",
} as CSSProperties;

/**
 * BindDialogState 描述仓库绑定弹窗状态。
 */
interface BindDialogState {
  open: boolean;
  path: string;
  busy: boolean;
  error: string;
}

/**
 * CreateDialogState 描述创建 worktree 弹窗状态。
 */
interface CreateDialogState {
  open: boolean;
  mode: CreateMode;
  sourceBranch: string;
  branchName: string;
  targetPath: string;
  targetEdited: boolean;
  submitting: boolean;
  error: string;
}

/**
 * RemoveDialogState 描述删除确认弹窗状态。
 */
interface RemoveDialogState {
  open: boolean;
  worktree: WorktreeInfo | null;
  forceStage: boolean;
  busy: boolean;
  message: string;
}

/**
 * emptyBindDialogState 返回默认绑定弹窗状态。
 */
function emptyBindDialogState(): BindDialogState {
  return {
    open: false,
    path: "",
    busy: false,
    error: "",
  };
}

/**
 * emptyCreateDialogState 返回默认创建弹窗状态。
 */
function emptyCreateDialogState(): CreateDialogState {
  return {
    open: false,
    mode: "existing",
    sourceBranch: "",
    branchName: "",
    targetPath: "",
    targetEdited: false,
    submitting: false,
    error: "",
  };
}

/**
 * emptyRemoveDialogState 返回默认删除弹窗状态。
 */
function emptyRemoveDialogState(): RemoveDialogState {
  return {
    open: false,
    worktree: null,
    forceStage: false,
    busy: false,
    message: "",
  };
}

/**
 * pickInitialRepositoryId 选择当前应展示的仓库 ID。
 */
function pickInitialRepositoryId(
  repositories: RepositorySummary[],
  settings: Settings,
  preferredRepositoryId?: string,
): string {
  const ids = new Set(repositories.map((repository) => repository.id));
  if (preferredRepositoryId && ids.has(preferredRepositoryId)) {
    return preferredRepositoryId;
  }

  if (settings.uiPreferences.lastSelectedRepositoryId && ids.has(settings.uiPreferences.lastSelectedRepositoryId)) {
    return settings.uiPreferences.lastSelectedRepositoryId;
  }

  return repositories[0]?.id ?? "";
}

/**
 * createDialogFromView 根据仓库视图生成创建弹窗默认值。
 */
function createDialogFromView(view: RepositoryView): CreateDialogState {
  const sourceBranch = view.availableBranches[0] ?? "";
  return {
    open: true,
    mode: "existing",
    sourceBranch,
    branchName: "",
    targetPath: buildSuggestedPath(view.suggestedRoot, sourceBranch || "new-worktree"),
    targetEdited: false,
    submitting: false,
    error: "",
  };
}

/**
 * resolveSuggestedTargetPath 计算创建弹窗的建议目标路径。
 */
function resolveSuggestedTargetPath(view: RepositoryView, mode: CreateMode, sourceBranch: string, branchName: string): string {
  const segment = mode === "new" ? branchName || sourceBranch || "new-worktree" : sourceBranch || branchName || "new-worktree";
  return buildSuggestedPath(view.suggestedRoot, segment);
}

/**
 * toErrorMessage 把未知异常转换成可展示文本。
 */
function toErrorMessage(error: unknown): string {
  if (error instanceof Error) {
    return error.message;
  }
  if (typeof error === "string") {
    return error;
  }

  return "发生了未知错误。";
}

/**
 * extractPathFromDrop 从拖拽事件中尽量解析目录路径。
 */
function extractPathFromDroppedFiles(paths: string[]): string {
  const candidate = paths[0] ?? "";
  return candidate.trim().replace(/^"+|"+$/g, "");
}

/**
 * App 是 TreeShift 的主界面。
 */
export default function App() {
  const [environment, setEnvironment] = useState<EnvironmentStatus | null>(null);
  const [settings, setSettings] = useState<Settings | null>(null);
  const [repositories, setRepositories] = useState<RepositorySummary[]>([]);
  const [activeRepositoryId, setActiveRepositoryId] = useState("");
  const [repositoryView, setRepositoryView] = useState<RepositoryView | null>(null);
  const [loadingMessage, setLoadingMessage] = useState("正在读取本地配置与仓库状态…");
  const [notice, setNotice] = useState<NoticeState | null>(null);
  const [bindDialog, setBindDialog] = useState<BindDialogState>(emptyBindDialogState());
  const [createDialog, setCreateDialog] = useState<CreateDialogState>(emptyCreateDialogState());
  const [removeDialog, setRemoveDialog] = useState<RemoveDialogState>(emptyRemoveDialogState());
  const [settingsDraft, setSettingsDraft] = useState<SettingsDraft | null>(null);
  const [settingsOpen, setSettingsOpen] = useState(false);
  const [settingsBusy, setSettingsBusy] = useState(false);

  useEffect(() => {
    void reloadWorkspace();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  /**
   * registerNativeFileDrop 订阅 Wails 原生文件拖拽事件。
   *
   * WebView2 普通 DOM 拖拽无法稳定拿到系统目录的绝对路径，
   * 因此这里统一改用 Wails 提供的文件拖拽通道。
   */
  useEffect(() => {
    const runtimeBridge = window as Window & {
      runtime?: {
        OnFileDrop?: typeof OnFileDrop;
        OnFileDropOff?: typeof OnFileDropOff;
      };
    };

    if (typeof runtimeBridge.runtime?.OnFileDrop !== "function") {
      return;
    }

    OnFileDrop((_x, _y, paths) => {
      const droppedPath = extractPathFromDroppedFiles(paths);
      if (!droppedPath) {
        return;
      }

      setBindDialog((current) => ({
        ...current,
        open: true,
        path: droppedPath,
        error: "",
      }));
    }, true);

    return () => {
      if (typeof runtimeBridge.runtime?.OnFileDropOff === "function") {
        OnFileDropOff();
      }
    };
  }, []);

  /**
   * 顶部提示条在展示 5 秒后自动消失。
   *
   * 当用户手动关闭提示，或新的提示覆盖旧提示时，会同步清理旧定时器，
   * 避免后续误清空最新提示。
   */
  useEffect(() => {
    if (!notice) {
      return;
    }

    const timer = window.setTimeout(() => {
      setNotice(null);
    }, noticeAutoDismissDelayMs);

    return () => {
      window.clearTimeout(timer);
    };
  }, [notice]);

  async function reloadWorkspace(preferredRepositoryId?: string) {
    setLoadingMessage("正在同步仓库状态…");

    try {
      const [nextEnvironment, nextSettings, nextRepositories] = await Promise.all([
        backend.checkEnvironment(),
        backend.getSettings(),
        backend.listRepositories(),
      ]);

      const nextActiveRepositoryId = pickInitialRepositoryId(nextRepositories, nextSettings, preferredRepositoryId);

      setEnvironment(nextEnvironment);
      setSettings(nextSettings);
      setRepositories(nextRepositories);
      setActiveRepositoryId(nextActiveRepositoryId);

      if (nextActiveRepositoryId) {
        setRepositoryView(await backend.getWorktrees(nextActiveRepositoryId));
      } else {
        setRepositoryView(null);
      }
    } catch (error) {
      setNotice({ tone: "error", message: toErrorMessage(error) });
    } finally {
      setLoadingMessage("");
    }
  }

  /**
   * runWithLoading 用统一的加载提示包装异步操作。
   *
   * 它用于会触发后台命令的交互，避免用户只看到界面停住而没有反馈。
   */
  async function runWithLoading<T>(message: string, action: () => Promise<T>): Promise<T> {
    setLoadingMessage(message);
    try {
      return await action();
    } finally {
      setLoadingMessage("");
    }
  }

  async function refreshRepositorySummaryOnly() {
    const nextRepositories = await backend.listRepositories();
    setRepositories(nextRepositories);
  }

  async function refreshActiveRepositoryView(repositoryId: string) {
    if (!repositoryId) {
      setRepositoryView(null);
      return;
    }

    setLoadingMessage("正在刷新 worktree 列表…");
    try {
      setRepositoryView(await backend.getWorktrees(repositoryId));
    } catch (error) {
      setNotice({ tone: "error", message: toErrorMessage(error) });
    } finally {
      setLoadingMessage("");
    }
  }

  /**
   * handleRepositoryChange 切换当前活动仓库。
   */
  async function handleRepositoryChange(event: ChangeEvent<HTMLSelectElement>) {
    const nextRepositoryId = event.target.value;

    try {
      await backend.selectRepository(nextRepositoryId);
      setActiveRepositoryId(nextRepositoryId);
      await refreshActiveRepositoryView(nextRepositoryId);
    } catch (error) {
      setNotice({ tone: "error", message: toErrorMessage(error) });
    }
  }

  /**
   * handleBindRepository 提交仓库绑定请求。
   */
  async function handleBindRepository(event?: FormEvent<HTMLFormElement>) {
    event?.preventDefault();
    if (!bindDialog.path.trim()) {
      setBindDialog((current) => ({ ...current, error: "请输入或拖入 Git 仓库目录。" }));
      return;
    }

    setBindDialog((current) => ({ ...current, busy: true, error: "" }));
    try {
      const summary = await runWithLoading("正在绑定仓库…", () => backend.bindRepository(bindDialog.path.trim()));
      setBindDialog(emptyBindDialogState());
      setNotice({ tone: "success", message: `已绑定仓库：${summary.displayName}` });
      await reloadWorkspace(summary.id);
    } catch (error) {
      setBindDialog((current) => ({ ...current, busy: false, error: toErrorMessage(error) }));
    }
  }

  /**
   * handleBrowseForBind 打开仓库目录选择器。
   */
  async function handleBrowseForBind() {
    try {
      const selectedPath = await backend.chooseDirectory({
        title: "选择主 Git 仓库目录",
        defaultPath: bindDialog.path || repositoryView?.repository.mainWorktreePath || "",
      });
      if (selectedPath) {
        setBindDialog((current) => ({ ...current, path: selectedPath, error: "" }));
      }
    } catch (error) {
      setBindDialog((current) => ({ ...current, error: toErrorMessage(error) }));
    }
  }

  /**
   * openCreateDialog 打开创建 worktree 弹窗。
   */
  function openCreateDialog() {
    if (!repositoryView) {
      return;
    }

    setCreateDialog(createDialogFromView(repositoryView));
  }

  /**
   * updateCreateDialog 统一更新创建弹窗字段。
   *
   * 当目标路径尚未被用户手动改写时，分支变化会同步刷新建议路径。
   */
  function updateCreateDialog(patch: Partial<CreateDialogState>) {
    if (!repositoryView) {
      return;
    }

    setCreateDialog((current) => {
      const nextState: CreateDialogState = {
        ...current,
        ...patch,
      };

      if (!nextState.targetEdited && (patch.mode || patch.sourceBranch !== undefined || patch.branchName !== undefined)) {
        nextState.targetPath = resolveSuggestedTargetPath(
          repositoryView,
          nextState.mode,
          nextState.sourceBranch,
          nextState.branchName,
        );
      }

      return nextState;
    });
  }

  /**
   * handleBrowseForCreatePath 选择创建 worktree 的目标目录。
   */
  async function handleBrowseForCreatePath() {
    try {
      const selectedPath = await backend.chooseDirectory({
        title: "选择 Worktree 目标目录",
        defaultPath: createDialog.targetPath || repositoryView?.suggestedRoot || "",
      });
      if (selectedPath) {
        setCreateDialog((current) => ({
          ...current,
          targetPath: selectedPath,
          targetEdited: true,
        }));
      }
    } catch (error) {
      setCreateDialog((current) => ({ ...current, error: toErrorMessage(error) }));
    }
  }

  /**
   * handleCreateWorktree 提交创建请求。
   */
  async function handleCreateWorktree(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!activeRepositoryId) {
      return;
    }

    setCreateDialog((current) => ({ ...current, submitting: true, error: "" }));
    try {
      const view = await runWithLoading("正在创建 Worktree…", () =>
        backend.createWorktree({
          repositoryId: activeRepositoryId,
          mode: createDialog.mode,
          sourceBranch: createDialog.sourceBranch,
          branchName: createDialog.branchName,
          targetPath: createDialog.targetPath,
        }),
      );

      setRepositoryView(view);
      setCreateDialog(emptyCreateDialogState());
      setNotice({ tone: "success", message: "Worktree 创建完成。" });
      await refreshRepositorySummaryOnly();
    } catch (error) {
      setCreateDialog((current) => ({ ...current, submitting: false, error: toErrorMessage(error) }));
    }
  }

  /**
   * openRemoveDialog 打开删除确认弹窗。
   */
  function openRemoveDialog(worktree: WorktreeInfo) {
    setRemoveDialog({
      open: true,
      worktree,
      forceStage: false,
      busy: false,
      message: "",
    });
  }

  /**
   * handleRemoveWorktree 执行删除或强制删除。
   */
  async function handleRemoveWorktree() {
    if (!activeRepositoryId || !removeDialog.worktree) {
      return;
    }

    const currentWorktree = removeDialog.worktree;

    setRemoveDialog((current) => ({ ...current, busy: true, message: "" }));
    try {
      const result = await runWithLoading("正在删除 Worktree…", () =>
        backend.removeWorktree({
          repositoryId: activeRepositoryId,
          path: currentWorktree.path,
          force: removeDialog.forceStage,
        }),
      );

      if (result.requiresForce) {
        setRemoveDialog((current) => ({
          ...current,
          busy: false,
          forceStage: true,
          message: result.message,
        }));
        return;
      }

      if (result.view?.repository?.id) {
        setRepositoryView(result.view);
      }
      await refreshRepositorySummaryOnly();

      if (result.success) {
        setNotice({ tone: "success", message: result.message });
        setRemoveDialog(emptyRemoveDialogState());
      } else if (result.stage === "folder_busy") {
        setNotice({ tone: "warning", message: result.message });
        setRemoveDialog(emptyRemoveDialogState());
      } else {
        setRemoveDialog((current) => ({ ...current, busy: false, message: result.message }));
      }
    } catch (error) {
      setRemoveDialog((current) => ({ ...current, busy: false, message: toErrorMessage(error) }));
    }
  }

  /**
   * handleRetryCleanup 对待清理目录执行重试删除。
   */
  async function handleRetryCleanup(worktree: WorktreeInfo) {
    if (!activeRepositoryId) {
      return;
    }

    try {
      const result = await runWithLoading("正在重试清理目录…", () =>
        backend.retryDeleteFolder({
          repositoryId: activeRepositoryId,
          path: worktree.path,
        }),
      );

      setRepositoryView(result.view);
      await refreshRepositorySummaryOnly();
      setNotice({ tone: result.success ? "success" : "warning", message: result.message });
    } catch (error) {
      setNotice({ tone: "error", message: toErrorMessage(error) });
    }
  }

  /**
   * handleLaunchTool 在指定 worktree 中启动 AI CLI。
   */
  async function handleLaunchTool(toolId: string, worktree: WorktreeInfo) {
    if (!activeRepositoryId) {
      return;
    }

    try {
      await backend.launchTool({
        toolId,
        repositoryId: activeRepositoryId,
        worktreePath: worktree.path,
        branch: worktree.branch,
      });
      setNotice({ tone: "success", message: `已启动工具：${toolId}` });
    } catch (error) {
      setNotice({ tone: "error", message: toErrorMessage(error) });
    }
  }

  /**
   * openSettingsDialog 打开设置弹窗。
   */
  function openSettingsDialog() {
    if (!settings) {
      return;
    }

    setSettingsDraft(toSettingsDraft(settings));
    setSettingsOpen(true);
  }

  /**
   * handleBrowseForGlobalDefaultRoot 选择全局默认输出路径。
   */
  async function handleBrowseForGlobalDefaultRoot() {
    if (!settingsDraft) {
      return;
    }

    try {
      const selectedPath = await backend.chooseDirectory({
        title: "选择全局默认 Worktree 根目录",
        defaultPath: settingsDraft.defaultWorktreeRoot,
      });

      if (selectedPath) {
        setSettingsDraft((current) =>
          current
            ? {
                ...current,
                defaultWorktreeRoot: selectedPath,
              }
            : current,
        );
      }
    } catch (error) {
      setNotice({ tone: "error", message: toErrorMessage(error) });
    }
  }

  /**
   * handleBrowseForRepositoryRoot 选择单仓库默认输出路径。
   */
  async function handleBrowseForRepositoryRoot(repositoryId: string, defaultPath: string) {
    try {
      const selectedPath = await backend.chooseDirectory({
        title: "选择仓库默认 Worktree 根目录",
        defaultPath,
      });

      if (selectedPath) {
        setSettingsDraft((current) => {
          if (!current) {
            return current;
          }

          return {
            ...current,
            repositories: current.repositories.map((repository) =>
              repository.id === repositoryId
                ? {
                    ...repository,
                    defaultWorktreeRoot: selectedPath,
                  }
                : repository,
            ),
          };
        });
      }
    } catch (error) {
      setNotice({ tone: "error", message: toErrorMessage(error) });
    }
  }

  /**
   * handleSaveSettings 持久化设置草稿。
   */
  async function handleSaveSettings() {
    if (!settingsDraft) {
      return;
    }

    setSettingsBusy(true);
    try {
      const normalizedDraft: SettingsDraft = {
        ...settingsDraft,
        repositories: settingsDraft.repositories.map((repository) => ensureRepositoryDisplayName(repository)),
      };

      const savedSettings = await runWithLoading("正在保存设置…", () =>
        backend.saveSettings(fromSettingsDraft(normalizedDraft)),
      );
      setSettings(savedSettings);
      setSettingsOpen(false);
      setSettingsDraft(null);
      setNotice({ tone: "success", message: "设置已保存。" });
      await reloadWorkspace(activeRepositoryId);
    } catch (error) {
      setNotice({ tone: "error", message: toErrorMessage(error) });
    } finally {
      setSettingsBusy(false);
    }
  }

  /**
   * handleUnbindRepository 解除绑定指定仓库。
   */
  async function handleUnbindRepository(repositoryId: string, displayName: string) {
    const accepted = window.confirm(`确定要解除绑定仓库“${displayName}”吗？这不会删除任何文件。`);
    if (!accepted) {
      return;
    }

    try {
      await runWithLoading("正在解除绑定…", () => backend.unbindRepository(repositoryId));
      setNotice({ tone: "success", message: `已解除绑定：${displayName}` });
      await reloadWorkspace();

      const refreshedSettings = await backend.getSettings();
      setSettings(refreshedSettings);
      setSettingsDraft(toSettingsDraft(refreshedSettings));
    } catch (error) {
      setNotice({ tone: "error", message: toErrorMessage(error) });
    }
  }

  const enabledTools = settings?.externalTools.filter((tool) => tool.enabled && tool.command.trim()) ?? [];

  return (
    <div className="relative h-screen overflow-hidden">
      <div className="pointer-events-none absolute inset-0">
        <div className="absolute left-[-12%] top-[-6%] h-[420px] w-[420px] rounded-full bg-ember-200/30 blur-3xl" />
        <div className="absolute bottom-[-14%] right-[-10%] h-[380px] w-[380px] rounded-full bg-moss-500/20 blur-3xl" />
      </div>

      {loadingMessage ? (
        <div className="fixed inset-0 z-[60] flex items-center justify-center bg-stone-950/10 backdrop-blur-[1px]">
          <div className="glass-panel flex items-center gap-3 px-4 py-3">
            <span className="h-4 w-4 rounded-full border-2 border-ember-300 border-t-ember-700 animate-spin" />
            <p className="text-sm font-medium text-stone-800">{loadingMessage}</p>
          </div>
        </div>
      ) : null}

      <main className="no-scrollbar relative mx-auto flex h-screen max-w-[1560px] flex-col gap-3 overflow-y-auto px-3 py-3 md:px-5">
        <EnvironmentStrip environment={environment} />

        {notice ? (
          <section
            className={`glass-panel border px-3 py-2.5 ${
              notice.tone === "success"
                ? "border-emerald-300/70 bg-emerald-50/85"
                : notice.tone === "warning"
                  ? "border-amber-300/70 bg-amber-50/85"
                  : "border-rose-300/70 bg-rose-50/85"
            }`}
          >
            <div className="flex items-center justify-between gap-4">
              <p className="text-sm font-medium text-stone-900">{notice.message}</p>
              <button className="ghost-button px-3 py-1" onClick={() => setNotice(null)} type="button">
                隐藏
              </button>
            </div>
          </section>
        ) : null}

        <section className="glass-panel px-3 py-2.5">
          <div className="no-scrollbar flex items-center gap-1.5 overflow-x-auto whitespace-nowrap">
            <label className="shrink-0 text-xs font-semibold uppercase tracking-[0.16em] text-stone-500">当前仓库</label>
            <select className="field-shell min-w-[220px] flex-1 px-3 py-1.5 text-xs" onChange={handleRepositoryChange} value={activeRepositoryId}>
              <option value="">请选择已绑定仓库</option>
              {repositories.map((repository) => (
                <option key={repository.id} value={repository.id}>
                  {repository.displayName} · {repository.worktreeCount} 个 Worktree
                  {repository.pendingCleanupCount > 0 ? ` · ${repository.pendingCleanupCount} 个待清理` : ""}
                </option>
              ))}
            </select>
            <button className="ghost-button shrink-0 px-2.5 py-1 text-xs" onClick={() => setBindDialog({ ...emptyBindDialogState(), open: true })} type="button">
              绑定
            </button>
            <button className="ghost-button shrink-0 px-2.5 py-1 text-xs" onClick={() => void refreshActiveRepositoryView(activeRepositoryId)} type="button">
              刷新
            </button>
            <button className="primary-button shrink-0 px-2.5 py-1 text-xs" disabled={!repositoryView} onClick={openCreateDialog} type="button">
              新建
            </button>
            <button className="ghost-button shrink-0 px-2.5 py-1 text-xs" onClick={openSettingsDialog} type="button">
              设置
            </button>
          </div>
        </section>

        {!repositoryView ? (
          <section
            className="glass-panel flex min-h-[240px] flex-col items-center justify-center gap-3 border-dashed border-stone-300/80 px-5 py-8 text-center"
            style={wailsDropTargetStyle}
          >
            <span className="tag bg-ember-100 px-2.5 py-0.5 text-[11px] text-ember-800">空工作区</span>
            <h2 className="font-display text-2xl font-semibold text-stone-900">先绑定一个主 Git 仓库</h2>
            <p className="max-w-xl text-sm leading-6 text-stone-600">
              你可以把仓库目录拖到这里，也可以点击按钮调用原生目录选择器。绑定后会自动读取
              `git worktree list` 结果并展示所有工作区卡片。
            </p>
            <div className="flex flex-wrap justify-center gap-3">
              <button className="primary-button" onClick={() => setBindDialog({ ...emptyBindDialogState(), open: true })} type="button">
                绑定仓库
              </button>
              <button className="ghost-button" onClick={() => void reloadWorkspace()} type="button">
                重新检查环境
              </button>
            </div>
          </section>
        ) : (
          <section className="space-y-2.5">
            <div className="flex items-center gap-3 px-1">
              <div className="h-px flex-1 bg-gradient-to-r from-transparent via-stone-300/80 to-stone-200/40" />
              <div className="h-1.5 w-1.5 rounded-full bg-ember-400/70" />
              <div className="h-px flex-1 bg-gradient-to-r from-stone-200/40 via-stone-300/80 to-transparent" />
            </div>

            <div className="grid gap-2.5 xl:grid-cols-2">
              {repositoryView.worktrees.map((worktree) => (
                <WorktreeCard
                  key={`${worktree.path}-${worktree.status}`}
                  onLaunchTool={(toolId, targetWorktree) => void handleLaunchTool(toolId, targetWorktree)}
                  onOpenExplorer={(path) =>
                    void backend.openInExplorer(path).catch((error) => setNotice({ tone: "error", message: toErrorMessage(error) }))
                  }
                  onOpenTerminal={(path) =>
                    void backend.openInTerminal(path).catch((error) => setNotice({ tone: "error", message: toErrorMessage(error) }))
                  }
                  onRemove={openRemoveDialog}
                  onRetryCleanup={(targetWorktree) => void handleRetryCleanup(targetWorktree)}
                  tools={enabledTools}
                  worktree={worktree}
                />
              ))}
            </div>
          </section>
        )}
      </main>

      <Modal
        description="支持拖入目录、直接粘贴路径或调用原生目录选择器。后端会自动解析 common dir，避免同一仓库重复绑定。"
        footer={
          <div className="flex flex-wrap justify-end gap-3">
            <button className="ghost-button" onClick={() => setBindDialog(emptyBindDialogState())} type="button">
              取消
            </button>
            <button className="primary-button" disabled={bindDialog.busy} onClick={() => void handleBindRepository()} type="button">
              {bindDialog.busy ? "绑定中…" : "绑定仓库"}
            </button>
          </div>
        }
        onClose={() => setBindDialog(emptyBindDialogState())}
        open={bindDialog.open}
        panelClassName="max-w-[620px]"
        title="绑定主 Git 仓库"
      >
        <form className="space-y-4" onSubmit={handleBindRepository}>
          <div
            className="rounded-[20px] border border-dashed border-stone-300 bg-stone-50/80 px-4 py-6 text-center"
            style={wailsDropTargetStyle}
          >
            <p className="font-display text-lg font-semibold text-stone-900">把仓库目录拖到这里</p>
            <p className="mt-1.5 text-sm leading-6 text-stone-600">也可以直接把 `D:\Code\your-repo` 这类路径粘贴到下面。</p>
          </div>

          <div className="space-y-2">
            <label className="text-sm font-medium text-stone-700">Git 仓库目录</label>
            <div className="flex flex-col gap-2 sm:flex-row">
              <input
                className="field-shell min-w-0 flex-1"
                onChange={(event) => setBindDialog((current) => ({ ...current, path: event.target.value, error: "" }))}
                placeholder="例如：D:\Code\your-repo"
                value={bindDialog.path}
              />
              <button className="ghost-button shrink-0 px-3 py-2 text-xs" onClick={() => void handleBrowseForBind()} type="button">
                浏览目录
              </button>
            </div>
            {bindDialog.error ? <p className="text-sm text-rose-700">{bindDialog.error}</p> : null}
          </div>
        </form>
      </Modal>

      <Modal
        description="先选择现有分支或基线分支，再确认输出目录。默认目录会基于建议根路径和分支名自动生成。"
        footer={
          <div className="flex flex-wrap justify-end gap-3">
            <button className="ghost-button" onClick={() => setCreateDialog(emptyCreateDialogState())} type="button">
              取消
            </button>
            <button
              className="primary-button"
              disabled={createDialog.submitting || !repositoryView}
              onClick={() => {
                const form = document.getElementById("create-worktree-form");
                if (form instanceof HTMLFormElement) {
                  form.requestSubmit();
                }
              }}
              type="button"
            >
              {createDialog.submitting ? "创建中…" : "创建 Worktree"}
            </button>
          </div>
        }
        onClose={() => setCreateDialog(emptyCreateDialogState())}
        open={createDialog.open}
        title="创建 Worktree"
      >
        <form className="space-y-5" id="create-worktree-form" onSubmit={handleCreateWorktree}>
          <div className="grid gap-5 md:grid-cols-2">
            <label className="space-y-2">
              <span className="text-sm font-medium text-stone-700">创建模式</span>
              <select
                className="field-shell w-full"
                onChange={(event) => updateCreateDialog({ mode: event.target.value as CreateMode })}
                value={createDialog.mode}
              >
                <option value="existing">基于现有分支创建</option>
                <option value="new">创建新分支并创建 Worktree</option>
              </select>
            </label>

            <label className="space-y-2">
              <span className="text-sm font-medium text-stone-700">
                {createDialog.mode === "new" ? "基线分支" : "现有分支"}
              </span>
              <select
                className="field-shell w-full"
                onChange={(event) => updateCreateDialog({ sourceBranch: event.target.value })}
                value={createDialog.sourceBranch}
              >
                <option value="">请选择分支</option>
                {repositoryView?.availableBranches.map((branch) => (
                  <option key={branch} value={branch}>
                    {branch}
                  </option>
                ))}
              </select>
            </label>
          </div>

          {createDialog.mode === "new" ? (
            <label className="space-y-2">
              <span className="text-sm font-medium text-stone-700">新分支名称</span>
              <input
                className="field-shell w-full"
                onChange={(event) => updateCreateDialog({ branchName: event.target.value })}
                placeholder="例如：feature/worktree-manager"
                value={createDialog.branchName}
              />
            </label>
          ) : null}

          <div className="space-y-2">
            <label className="text-sm font-medium text-stone-700">目标目录</label>
            <div className="flex flex-col gap-3 md:flex-row">
              <input
                className="field-shell min-w-0 flex-1"
                onChange={(event) =>
                  setCreateDialog((current) => ({
                    ...current,
                    targetPath: event.target.value,
                    targetEdited: true,
                    error: "",
                  }))
                }
                value={createDialog.targetPath}
              />
              <button className="ghost-button" onClick={() => void handleBrowseForCreatePath()} type="button">
                浏览目录
              </button>
            </div>
            <p className="text-xs text-stone-500">
              当前建议根目录：{repositoryView?.suggestedRoot ?? "未计算"}。手动修改后，分支变化将不再自动覆盖目标路径。
            </p>
          </div>

          {createDialog.error ? <p className="text-sm text-rose-700">{createDialog.error}</p> : null}
        </form>
      </Modal>

      <Modal
        description="删除会先执行 Git 注销，再尝试物理删除目录。若检测到未提交修改，会要求你再确认一次强制删除。"
        footer={
          <div className="flex flex-wrap justify-end gap-3">
            <button className="ghost-button" onClick={() => setRemoveDialog(emptyRemoveDialogState())} type="button">
              取消
            </button>
            <button className="primary-button" disabled={removeDialog.busy} onClick={() => void handleRemoveWorktree()} type="button">
              {removeDialog.busy ? "处理中…" : removeDialog.forceStage ? "强制删除" : "删除"}
            </button>
          </div>
        }
        onClose={() => setRemoveDialog(emptyRemoveDialogState())}
        open={removeDialog.open}
        title={removeDialog.forceStage ? "强制删除确认" : "删除 Worktree"}
      >
        <div className="space-y-4">
          <p className="text-sm text-stone-700">
            {removeDialog.worktree
              ? `目标分支：${removeDialog.worktree.branch} · 目录：${removeDialog.worktree.path}`
              : "尚未选择删除目标。"}
          </p>
          {removeDialog.message ? (
            <div className="rounded-2xl border border-amber-300/80 bg-amber-50/90 px-4 py-4 text-sm text-amber-900">
              {removeDialog.message}
            </div>
          ) : null}
        </div>
      </Modal>

      <Modal
        description="所有配置都以 JSON 存储在可执行文件同级目录。工具参数请按“一行一个参数”填写，支持 {path} 和 {branch} 占位符。"
        footer={
          <div className="flex flex-wrap justify-end gap-3">
            <button
              className="ghost-button"
              onClick={() => {
                setSettingsOpen(false);
                setSettingsDraft(null);
              }}
              type="button"
            >
              取消
            </button>
            <button className="primary-button" disabled={settingsBusy || !settingsDraft} onClick={() => void handleSaveSettings()} type="button">
              {settingsBusy ? "保存中…" : "保存设置"}
            </button>
          </div>
        }
        onClose={() => {
          setSettingsOpen(false);
          setSettingsDraft(null);
        }}
        open={settingsOpen}
        panelClassName="max-w-[760px]"
        title="设置"
      >
        {settingsDraft ? (
          <div className="space-y-6">
            <section className="space-y-3">
              <div className="flex flex-wrap items-center justify-between gap-2">
                <div>
                  <h3 className="font-display text-lg font-semibold text-stone-900">默认路径</h3>
                  <p className="text-sm text-stone-600">未为仓库单独配置时，新建 Worktree 会优先使用这里的根目录。</p>
                </div>
                <button className="ghost-button px-3 py-1.5 text-xs" onClick={() => void handleBrowseForGlobalDefaultRoot()} type="button">
                  浏览目录
                </button>
              </div>
              <input
                className="field-shell w-full"
                onChange={(event) =>
                  setSettingsDraft((current) =>
                    current
                      ? {
                          ...current,
                          defaultWorktreeRoot: event.target.value,
                        }
                      : current,
                  )
                }
                placeholder="例如：D:\Worktrees"
                value={settingsDraft.defaultWorktreeRoot}
              />
            </section>

            <section className="space-y-3">
              <div>
                <h3 className="font-display text-lg font-semibold text-stone-900">已绑定仓库</h3>
                <p className="text-sm text-stone-600">可以调整展示名称、单仓库默认输出路径，或解除绑定。</p>
              </div>
              <div className="space-y-3">
                {settingsDraft.repositories.map((repository) => (
                  <article key={repository.id} className="rounded-[20px] border border-stone-200/80 bg-stone-50/85 px-4 py-4">
                    <div className="space-y-3">
                      <label className="space-y-2">
                        <span className="text-sm font-medium text-stone-700">显示名称</span>
                        <input
                          className="field-shell w-full"
                          onChange={(event) =>
                            setSettingsDraft((current) =>
                              current
                                ? {
                                    ...current,
                                    repositories: current.repositories.map((item) =>
                                      item.id === repository.id
                                        ? {
                                            ...item,
                                            displayName: event.target.value,
                                          }
                                        : item,
                                    ),
                                  }
                                : current,
                            )
                          }
                          value={repository.displayName}
                        />
                      </label>

                      <label className="space-y-2">
                        <span className="text-sm font-medium text-stone-700">仓库默认 Worktree 根目录</span>
                        <input
                          className="field-shell w-full"
                          onChange={(event) =>
                            setSettingsDraft((current) =>
                              current
                                ? {
                                    ...current,
                                    repositories: current.repositories.map((item) =>
                                      item.id === repository.id
                                        ? {
                                            ...item,
                                            defaultWorktreeRoot: event.target.value,
                                          }
                                        : item,
                                    ),
                                  }
                                : current,
                            )
                          }
                          placeholder="留空则回退到全局默认路径"
                          value={repository.defaultWorktreeRoot}
                        />
                        <p className="break-all text-xs text-stone-500">主工作区：{repository.mainWorktreePath}</p>
                      </label>

                      <div className="flex flex-wrap gap-2">
                        <button
                          className="ghost-button px-3 py-1.5 text-xs"
                          onClick={() => void handleBrowseForRepositoryRoot(repository.id, repository.defaultWorktreeRoot)}
                          type="button"
                        >
                          浏览目录
                        </button>
                        <button
                          className="ghost-button border-rose-300/80 px-3 py-1.5 text-xs text-rose-700 hover:border-rose-400 hover:bg-rose-50 hover:text-rose-800"
                          onClick={() => void handleUnbindRepository(repository.id, repository.displayName)}
                          type="button"
                        >
                          解除绑定
                        </button>
                      </div>
                    </div>
                  </article>
                ))}
              </div>
            </section>

            <section className="space-y-3">
              <div className="flex flex-wrap items-center justify-between gap-2">
                <div>
                  <h3 className="font-display text-lg font-semibold text-stone-900">外部工具</h3>
                  <p className="text-sm text-stone-600">支持通用 CLI。每一行参数会作为独立参数传给命令本体。</p>
                </div>
                <button
                  className="ghost-button px-3 py-1.5 text-xs"
                  onClick={() =>
                    setSettingsDraft((current) =>
                      current
                        ? {
                            ...current,
                            externalTools: [...current.externalTools, createEmptyToolDraft()],
                          }
                        : current,
                    )
                  }
                  type="button"
                >
                  添加工具
                </button>
              </div>
              <div className="space-y-3">
                {settingsDraft.externalTools.map((tool) => (
                  <article key={tool.id} className="rounded-[20px] border border-stone-200/80 bg-stone-50/85 px-4 py-4">
                    <div className="space-y-3">
                      <label className="space-y-2">
                        <span className="text-sm font-medium text-stone-700">工具名称</span>
                        <input
                          className="field-shell w-full"
                          onChange={(event) =>
                            setSettingsDraft((current) =>
                              current
                                ? {
                                    ...current,
                                    externalTools: current.externalTools.map((item) =>
                                      item.id === tool.id
                                        ? {
                                            ...item,
                                            name: event.target.value,
                                          }
                                        : item,
                                    ),
                                  }
                                : current,
                            )
                          }
                          placeholder="例如：Codex CLI"
                          value={tool.name}
                        />
                      </label>

                      <label className="space-y-2">
                        <span className="text-sm font-medium text-stone-700">命令</span>
                        <input
                          className="field-shell w-full"
                          onChange={(event) =>
                            setSettingsDraft((current) =>
                              current
                                ? {
                                    ...current,
                                    externalTools: current.externalTools.map((item) =>
                                      item.id === tool.id
                                        ? {
                                            ...item,
                                            command: event.target.value,
                                          }
                                        : item,
                                    ),
                                  }
                                : current,
                            )
                          }
                          placeholder="例如：codex 或 C:\Tools\codex.exe"
                          value={tool.command}
                        />
                      </label>

                      <label className="space-y-2">
                        <span className="text-sm font-medium text-stone-700">参数（每行一个）</span>
                        <textarea
                          className="field-shell min-h-[120px] w-full resize-y"
                          onChange={(event) =>
                            setSettingsDraft((current) =>
                              current
                                ? {
                                    ...current,
                                    externalTools: current.externalTools.map((item) =>
                                      item.id === tool.id
                                        ? {
                                            ...item,
                                            argsText: event.target.value,
                                          }
                                        : item,
                                    ),
                                  }
                                : current,
                            )
                          }
                          placeholder={"例如：\n--model\ngpt-5\n--cwd\n{path}"}
                          value={tool.argsText}
                        />
                      </label>

                      <div className="flex flex-wrap items-center gap-2">
                        <label className="flex items-center gap-2 rounded-[18px] border border-stone-200/80 bg-white/75 px-3 py-2">
                          <input
                            checked={tool.enabled}
                            onChange={(event) =>
                              setSettingsDraft((current) =>
                                current
                                  ? {
                                      ...current,
                                      externalTools: current.externalTools.map((item) =>
                                        item.id === tool.id
                                          ? {
                                              ...item,
                                              enabled: event.target.checked,
                                            }
                                          : item,
                                      ),
                                    }
                                  : current,
                              )
                            }
                            type="checkbox"
                          />
                          <span className="text-xs font-medium text-stone-700">启用</span>
                        </label>

                        <button
                          className="ghost-button border-rose-300/80 px-3 py-1.5 text-xs text-rose-700 hover:border-rose-400 hover:bg-rose-50 hover:text-rose-800"
                          onClick={() =>
                            setSettingsDraft((current) =>
                              current
                                ? {
                                    ...current,
                                    externalTools: current.externalTools.filter((item) => item.id !== tool.id),
                                  }
                                : current,
                            )
                          }
                          type="button"
                        >
                          移除工具
                        </button>
                      </div>
                    </div>
                  </article>
                ))}
              </div>
            </section>
          </div>
        ) : null}
      </Modal>
    </div>
  );
}
