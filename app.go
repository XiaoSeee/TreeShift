package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"treeshift/internal/config"
	"treeshift/internal/environment"
	"treeshift/internal/git"
	"treeshift/internal/launcher"
	"treeshift/internal/model"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App 是 Wails 的前后端桥接对象。
//
// 它统一协调配置持久化、环境检查、Git worktree 管理和外部工具启动，
// 并将这些能力暴露给前端调用。
type App struct {
	ctx                context.Context
	mu                 sync.Mutex
	configService      *config.Service
	environmentService *environment.Service
	gitService         *git.Service
	launcherService    *launcher.Service
	settings           model.Settings
	pendingCleanup     map[string]model.PendingCleanup
	startupWarnings    []string
}

// NewApp 创建应用实例并加载本地配置。
//
// 如果配置读取失败，会回退到默认配置，并把失败原因保存在启动告警中，
// 供前端在诊断条里展示。
func NewApp() *App {
	configService := config.NewService("TreeShift", "")
	settings, err := configService.Load()

	app := &App{
		configService:      configService,
		environmentService: environment.NewService(),
		gitService:         git.NewService(),
		launcherService:    launcher.NewService(),
		settings:           settings,
		pendingCleanup:     map[string]model.PendingCleanup{},
		startupWarnings:    []string{},
	}

	if err != nil {
		app.startupWarnings = append(app.startupWarnings, fmt.Sprintf("配置文件读取失败，已回退默认配置：%v", err))
	}

	return app
}

// startup 在 Wails 应用启动时注入运行时上下文。
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// CheckEnvironment 扫描 Git、Windows Terminal 与外部工具的可用性。
func (a *App) CheckEnvironment() (model.EnvironmentStatus, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	return a.environmentService.Check(a.settings, a.startupWarnings), nil
}

// GetSettings 返回当前内存中的完整配置。
func (a *App) GetSettings() (model.Settings, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	return a.settings, nil
}

// SaveSettings 持久化前端提交的新配置。
//
// 保存前会执行字段归一化和活动仓库校正，避免配置出现悬空引用。
func (a *App) SaveSettings(settings model.Settings) (model.Settings, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	normalized := config.NormalizeSettings(settings)
	a.normalizeActiveRepositoryLocked(&normalized)

	if err := a.configService.Save(normalized); err != nil {
		return model.Settings{}, err
	}

	a.settings = normalized
	return a.settings, nil
}

// ListRepositories 返回仓库切换器所需的摘要列表。
func (a *App) ListRepositories() ([]model.RepositorySummary, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	summaries := make([]model.RepositorySummary, 0, len(a.settings.Repositories))
	for _, repository := range a.settings.Repositories {
		worktrees, err := a.gitService.ListWorktrees(repository)
		worktreeCount := len(worktrees)
		if err != nil {
			worktreeCount = 0
		}

		pendingCount := 0
		for _, cleanup := range a.pendingCleanup {
			if cleanup.RepositoryID == repository.ID {
				pendingCount++
			}
		}

		summaries = append(summaries, model.RepositorySummary{
			ID:                  repository.ID,
			DisplayName:         repository.DisplayName,
			MainWorktreePath:    repository.MainWorktreePath,
			DefaultWorktreeRoot: repository.DefaultWorktreeRoot,
			WorktreeCount:       worktreeCount,
			PendingCleanupCount: pendingCount,
		})
	}

	return summaries, nil
}

// BindRepository 绑定一个新的 Git 仓库。
//
// 即使传入的是某个链接 worktree，也会统一解析为同一个 common dir，从而避免重复绑定。
func (a *App) BindRepository(path string) (model.RepositorySummary, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	resolved, err := a.gitService.ResolveRepository(path)
	if err != nil {
		return model.RepositorySummary{}, err
	}

	index := -1
	for currentIndex := range a.settings.Repositories {
		if a.settings.Repositories[currentIndex].CommonDir == resolved.CommonDir {
			index = currentIndex
			break
		}
	}

	if index >= 0 {
		existing := a.settings.Repositories[index]
		existing.SelectedPath = resolved.SelectedPath
		existing.MainWorktreePath = resolved.MainWorktreePath
		if strings.TrimSpace(existing.DisplayName) == "" {
			existing.DisplayName = resolved.DisplayName
		}
		a.settings.Repositories[index] = existing
		a.settings.UIPreferences.LastSelectedRepositoryID = existing.ID
	} else {
		a.settings.Repositories = append(a.settings.Repositories, resolved)
		a.settings.UIPreferences.LastSelectedRepositoryID = resolved.ID
	}

	a.normalizeActiveRepositoryLocked(&a.settings)
	if err := a.configService.Save(a.settings); err != nil {
		return model.RepositorySummary{}, err
	}

	repository, err := a.repositoryByIDLocked(a.settings.UIPreferences.LastSelectedRepositoryID)
	if err != nil {
		return model.RepositorySummary{}, err
	}

	worktrees, worktreeErr := a.gitService.ListWorktrees(repository)
	if worktreeErr != nil {
		worktrees = []model.WorktreeInfo{}
	}

	pendingCount := 0
	for _, cleanup := range a.pendingCleanup {
		if cleanup.RepositoryID == repository.ID {
			pendingCount++
		}
	}

	return model.RepositorySummary{
		ID:                  repository.ID,
		DisplayName:         repository.DisplayName,
		MainWorktreePath:    repository.MainWorktreePath,
		DefaultWorktreeRoot: repository.DefaultWorktreeRoot,
		WorktreeCount:       len(worktrees),
		PendingCleanupCount: pendingCount,
	}, nil
}

// UnbindRepository 解除一个已绑定仓库。
//
// 该操作只修改本地配置，不会删除任何 Git 数据或物理目录。
func (a *App) UnbindRepository(repositoryID string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	index := -1
	for currentIndex := range a.settings.Repositories {
		if a.settings.Repositories[currentIndex].ID == repositoryID {
			index = currentIndex
			break
		}
	}
	if index < 0 {
		return fmt.Errorf("未找到指定仓库：%s", repositoryID)
	}

	a.settings.Repositories = append(a.settings.Repositories[:index], a.settings.Repositories[index+1:]...)
	for key, cleanup := range a.pendingCleanup {
		if cleanup.RepositoryID == repositoryID {
			delete(a.pendingCleanup, key)
		}
	}

	a.normalizeActiveRepositoryLocked(&a.settings)
	return a.configService.Save(a.settings)
}

// SelectRepository 切换当前活动仓库。
func (a *App) SelectRepository(repositoryID string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if repositoryID == "" {
		a.settings.UIPreferences.LastSelectedRepositoryID = ""
		return a.configService.Save(a.settings)
	}

	if _, err := a.repositoryByIDLocked(repositoryID); err != nil {
		return err
	}

	a.settings.UIPreferences.LastSelectedRepositoryID = repositoryID
	return a.configService.Save(a.settings)
}

// GetWorktrees 返回指定仓库的完整视图。
//
// 视图中会合并 Git 当前登记的 worktree 列表和应用内存中的待清理目录。
func (a *App) GetWorktrees(repositoryID string) (model.RepositoryView, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	repository, err := a.repositoryByIDLocked(repositoryID)
	if err != nil {
		return model.RepositoryView{}, err
	}

	return a.buildRepositoryViewLocked(repository)
}

// CreateWorktree 创建新的 worktree，并返回最新仓库视图。
func (a *App) CreateWorktree(request model.CreateWorktreeRequest) (model.RepositoryView, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	repository, err := a.repositoryByIDLocked(request.RepositoryID)
	if err != nil {
		return model.RepositoryView{}, err
	}

	if err := a.gitService.CreateWorktree(repository, request); err != nil {
		return model.RepositoryView{}, err
	}

	return a.buildRepositoryViewLocked(repository)
}

// RemoveWorktree 删除一个已存在的 worktree。
//
// 该方法先调用 Git 注销，再尝试物理删除目录。
// 对于“脏目录拦截”这类预期失败，会返回结构化结果给前端二次确认。
func (a *App) RemoveWorktree(request model.RemoveWorktreeRequest) (model.RemoveWorktreeResult, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	repository, err := a.repositoryByIDLocked(request.RepositoryID)
	if err != nil {
		return model.RemoveWorktreeResult{}, err
	}

	viewBefore, err := a.buildRepositoryViewLocked(repository)
	if err != nil {
		return model.RemoveWorktreeResult{}, err
	}

	removedBranch := "unknown"
	removedHead := ""
	for _, worktree := range viewBefore.Worktrees {
		if filepath.Clean(worktree.Path) == filepath.Clean(request.Path) {
			removedBranch = worktree.Branch
			removedHead = worktree.Head
			break
		}
	}

	removeErr := a.gitService.RemoveWorktree(repository, request.Path, request.Force)
	if removeErr != nil {
		if git.IsDirtyRemoveError(removeErr) {
			return model.RemoveWorktreeResult{
				Success:       false,
				Stage:         model.RemoveStageDirtyBlocked,
				Message:       removeErr.Error(),
				RequiresForce: true,
			}, nil
		}

		return model.RemoveWorktreeResult{
			Success: false,
			Stage:   model.RemoveStageGitFailed,
			Message: removeErr.Error(),
		}, nil
	}

	if err := a.deleteFolderLocked(request.Path); err != nil {
		a.pendingCleanup[filepath.Clean(request.Path)] = model.PendingCleanup{
			RepositoryID: repository.ID,
			Path:         filepath.Clean(request.Path),
			Branch:       removedBranch,
			Head:         removedHead,
			LastError:    err.Error(),
		}

		view, viewErr := a.buildRepositoryViewLocked(repository)
		if viewErr != nil {
			return model.RemoveWorktreeResult{}, viewErr
		}

		return model.RemoveWorktreeResult{
			Success: false,
			Stage:   model.RemoveStageFolderBusy,
			Message: fmt.Sprintf("Git 注销成功，但目录仍被占用：%v", err),
			View:    view,
		}, nil
	}

	delete(a.pendingCleanup, filepath.Clean(request.Path))
	view, viewErr := a.buildRepositoryViewLocked(repository)
	if viewErr != nil {
		return model.RemoveWorktreeResult{}, viewErr
	}

	return model.RemoveWorktreeResult{
		Success: true,
		Stage:   model.RemoveStageRemoved,
		Message: "Worktree 已删除。",
		View:    view,
	}, nil
}

// RetryDeleteFolder 对待清理目录再次执行物理删除。
//
// 该操作不会再调用 Git，只适用于用户关闭占用程序后的“重试清理”场景。
func (a *App) RetryDeleteFolder(request model.RetryDeleteFolderRequest) (model.RetryDeleteFolderResult, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	repository, err := a.repositoryByIDLocked(request.RepositoryID)
	if err != nil {
		return model.RetryDeleteFolderResult{}, err
	}

	if err := a.deleteFolderLocked(request.Path); err != nil {
		entry := a.pendingCleanup[filepath.Clean(request.Path)]
		entry.RepositoryID = repository.ID
		entry.Path = filepath.Clean(request.Path)
		entry.LastError = err.Error()
		a.pendingCleanup[filepath.Clean(request.Path)] = entry

		view, viewErr := a.buildRepositoryViewLocked(repository)
		if viewErr != nil {
			return model.RetryDeleteFolderResult{}, viewErr
		}

		return model.RetryDeleteFolderResult{
			Success: false,
			Message: fmt.Sprintf("目录仍被占用：%v", err),
			View:    view,
		}, nil
	}

	delete(a.pendingCleanup, filepath.Clean(request.Path))
	view, viewErr := a.buildRepositoryViewLocked(repository)
	if viewErr != nil {
		return model.RetryDeleteFolderResult{}, viewErr
	}

	return model.RetryDeleteFolderResult{
		Success: true,
		Message: "目录已清理完成。",
		View:    view,
	}, nil
}

// OpenInExplorer 在资源管理器中打开指定目录。
func (a *App) OpenInExplorer(path string) error {
	if strings.TrimSpace(path) == "" {
		return errors.New("目录路径不能为空")
	}

	return a.launcherService.OpenExplorer(path)
}

// OpenInTerminal 在指定目录下启动 Windows Terminal。
func (a *App) OpenInTerminal(path string) error {
	if strings.TrimSpace(path) == "" {
		return errors.New("目录路径不能为空")
	}

	return a.launcherService.OpenTerminal(path)
}

// LaunchTool 在指定 worktree 目录下启动外部 AI CLI 工具。
func (a *App) LaunchTool(request model.LaunchToolRequest) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	for _, tool := range a.settings.ExternalTools {
		if tool.ID == request.ToolID {
			return a.launcherService.LaunchExternalTool(tool, request.WorktreePath, request.Branch)
		}
	}

	return fmt.Errorf("未找到指定工具：%s", request.ToolID)
}

// ChooseDirectory 打开原生目录选择器。
func (a *App) ChooseDirectory(request model.DirectoryDialogRequest) (string, error) {
	if a.ctx == nil {
		return "", errors.New("应用尚未完成启动，暂时无法打开目录选择器")
	}

	return runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title:                request.Title,
		DefaultDirectory:     request.DefaultPath,
		CanCreateDirectories: true,
	})
}

// buildRepositoryViewLocked 构造仓库完整视图。
//
// 调用方必须持有 a.mu。该方法会合并 Git 当前 worktree 与待清理虚拟项。
func (a *App) buildRepositoryViewLocked(repository model.RepositoryBinding) (model.RepositoryView, error) {
	worktrees, err := a.gitService.ListWorktrees(repository)
	if err != nil {
		return model.RepositoryView{}, err
	}

	branches, err := a.gitService.ListBranches(repository)
	if err != nil {
		return model.RepositoryView{}, err
	}

	merged := append([]model.WorktreeInfo{}, worktrees...)
	existing := map[string]struct{}{}
	for _, worktree := range worktrees {
		existing[filepath.Clean(worktree.Path)] = struct{}{}
	}

	for _, cleanup := range a.pendingCleanup {
		if cleanup.RepositoryID != repository.ID {
			continue
		}

		if _, found := existing[filepath.Clean(cleanup.Path)]; found {
			continue
		}

		merged = append(merged, model.WorktreeInfo{
			Path:          cleanup.Path,
			Branch:        cleanup.Branch,
			Head:          cleanup.Head,
			Status:        model.WorktreeStatusPendingCleanup,
			StatusMessage: cleanup.LastError,
		})
	}

	return model.RepositoryView{
		Repository:        repository,
		Worktrees:         merged,
		AvailableBranches: branches,
		SuggestedRoot:     suggestedRootPath(repository, a.settings.DefaultWorktreeRoot),
	}, nil
}

// repositoryByIDLocked 根据仓库 ID 查找绑定记录。
//
// 调用方必须持有 a.mu。
func (a *App) repositoryByIDLocked(repositoryID string) (model.RepositoryBinding, error) {
	for _, repository := range a.settings.Repositories {
		if repository.ID == repositoryID {
			return repository, nil
		}
	}

	return model.RepositoryBinding{}, fmt.Errorf("未找到指定仓库：%s", repositoryID)
}

// normalizeActiveRepositoryLocked 修正最近选中的仓库 ID。
//
// 调用方必须持有 a.mu。若当前选中项不存在，则回退到第一个仓库或空值。
func (a *App) normalizeActiveRepositoryLocked(settings *model.Settings) {
	if settings == nil {
		return
	}

	if len(settings.Repositories) == 0 {
		settings.UIPreferences.LastSelectedRepositoryID = ""
		return
	}

	for _, repository := range settings.Repositories {
		if repository.ID == settings.UIPreferences.LastSelectedRepositoryID {
			return
		}
	}

	settings.UIPreferences.LastSelectedRepositoryID = settings.Repositories[0].ID
}

// deleteFolderLocked 负责文件系统级目录删除。
//
// 调用方必须持有 a.mu。若目录已不存在则视为成功。
func (a *App) deleteFolderLocked(path string) error {
	cleanPath := filepath.Clean(path)
	if cleanPath == "." || strings.TrimSpace(path) == "" {
		return errors.New("目录路径不能为空")
	}

	if _, err := os.Stat(cleanPath); err != nil {
		if os.IsNotExist(err) {
			delete(a.pendingCleanup, cleanPath)
			return nil
		}

		return err
	}

	if err := os.RemoveAll(cleanPath); err != nil {
		return err
	}

	if _, err := os.Stat(cleanPath); err == nil {
		return fmt.Errorf("目录 %s 仍然存在，可能被其他程序占用", cleanPath)
	} else if !os.IsNotExist(err) {
		return err
	}

	return nil
}

// suggestedRootPath 计算某个仓库的建议 worktree 根目录。
//
// 优先级依次为仓库级默认路径、全局默认路径和主仓库父目录下的 `_worktrees/<仓库名>`。
func suggestedRootPath(repository model.RepositoryBinding, globalDefault string) string {
	if strings.TrimSpace(repository.DefaultWorktreeRoot) != "" {
		return filepath.Clean(repository.DefaultWorktreeRoot)
	}

	if strings.TrimSpace(globalDefault) != "" {
		return filepath.Clean(globalDefault)
	}

	parent := filepath.Dir(repository.MainWorktreePath)
	return filepath.Join(parent, "_worktrees", repository.DisplayName)
}
