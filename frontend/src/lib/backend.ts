import type {
  CreateWorktreeRequest,
  DirectoryDialogRequest,
  EnvironmentStatus,
  LaunchToolRequest,
  RemoveWorktreeRequest,
  RemoveWorktreeResult,
  RepositorySummary,
  RepositoryView,
  RetryDeleteFolderRequest,
  RetryDeleteFolderResult,
  Settings,
} from "../types";

/**
 * BackendApp 描述 Wails 在 window.go.main.App 上暴露的后端方法集合。
 */
interface BackendApp {
  CheckEnvironment(): Promise<EnvironmentStatus>;
  GetSettings(): Promise<Settings>;
  SaveSettings(settings: Settings): Promise<Settings>;
  ListRepositories(): Promise<RepositorySummary[]>;
  BindRepository(path: string): Promise<RepositorySummary>;
  UnbindRepository(repositoryId: string): Promise<void>;
  SelectRepository(repositoryId: string): Promise<void>;
  GetWorktrees(repositoryId: string): Promise<RepositoryView>;
  CreateWorktree(request: CreateWorktreeRequest): Promise<RepositoryView>;
  RemoveWorktree(request: RemoveWorktreeRequest): Promise<RemoveWorktreeResult>;
  RetryDeleteFolder(request: RetryDeleteFolderRequest): Promise<RetryDeleteFolderResult>;
  OpenInExplorer(path: string): Promise<void>;
  OpenInTerminal(path: string): Promise<void>;
  LaunchTool(request: LaunchToolRequest): Promise<void>;
  ChooseDirectory(request: DirectoryDialogRequest): Promise<string>;
}

declare global {
  interface Window {
    go?: {
      main?: {
        App?: BackendApp;
      };
    };
  }
}

/**
 * getBackend 解析 Wails 注入的后端绑定对象。
 */
function getBackend(): BackendApp {
  const backend = window.go?.main?.App;
  if (!backend) {
    throw new Error("Wails 后端绑定尚未就绪。");
  }

  return backend;
}

/**
 * backend 为前端提供统一、类型化的后端调用入口。
 */
export const backend = {
  checkEnvironment: () => getBackend().CheckEnvironment(),
  getSettings: () => getBackend().GetSettings(),
  saveSettings: (settings: Settings) => getBackend().SaveSettings(settings),
  listRepositories: () => getBackend().ListRepositories(),
  bindRepository: (path: string) => getBackend().BindRepository(path),
  unbindRepository: (repositoryId: string) => getBackend().UnbindRepository(repositoryId),
  selectRepository: (repositoryId: string) => getBackend().SelectRepository(repositoryId),
  getWorktrees: (repositoryId: string) => getBackend().GetWorktrees(repositoryId),
  createWorktree: (request: CreateWorktreeRequest) => getBackend().CreateWorktree(request),
  removeWorktree: (request: RemoveWorktreeRequest) => getBackend().RemoveWorktree(request),
  retryDeleteFolder: (request: RetryDeleteFolderRequest) => getBackend().RetryDeleteFolder(request),
  openInExplorer: (path: string) => getBackend().OpenInExplorer(path),
  openInTerminal: (path: string) => getBackend().OpenInTerminal(path),
  launchTool: (request: LaunchToolRequest) => getBackend().LaunchTool(request),
  chooseDirectory: (request: DirectoryDialogRequest) => getBackend().ChooseDirectory(request),
};
