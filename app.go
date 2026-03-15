package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
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
		pendingCleanup:     pendingCleanupMapFromSlice(settings.PendingCleanups),
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

// CheckEnvironment 扫描启动阶段必须依赖的环境项。
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
	normalized.PendingCleanups = pendingCleanupSlice(a.pendingCleanup)
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

	if err := a.reconcilePendingCleanupsLocked(); err != nil {
		return nil, err
	}

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
	if err := a.saveCurrentSettingsLocked(); err != nil {
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
	return a.saveCurrentSettingsLocked()
}

// SelectRepository 切换当前活动仓库。
func (a *App) SelectRepository(repositoryID string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if repositoryID == "" {
		a.settings.UIPreferences.LastSelectedRepositoryID = ""
		return a.saveCurrentSettingsLocked()
	}

	if _, err := a.repositoryByIDLocked(repositoryID); err != nil {
		return err
	}

	a.settings.UIPreferences.LastSelectedRepositoryID = repositoryID
	return a.saveCurrentSettingsLocked()
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

	targetWorktree, found := worktreeByPath(viewBefore.Worktrees, request.Path)
	if !found {
		return model.RemoveWorktreeResult{}, fmt.Errorf("未找到指定 Worktree：%s", request.Path)
	}

	if targetWorktree.Status == model.WorktreeStatusPendingCleanup {
		return a.removePendingCleanupFolderLocked(repository, targetWorktree)
	}

	if targetWorktree.Status == model.WorktreeStatusMissing {
		return a.removeMissingWorktreeLocked(repository, targetWorktree)
	}

	removeErr := a.gitService.RemoveWorktree(repository, request.Path, request.Force)
	if removeErr != nil {
		if git.IsDirtyRemoveError(removeErr) {
			return model.RemoveWorktreeResult{
				Success:       false,
				Stage:         model.RemoveStageDirtyBlocked,
				Message:       "当前 Worktree 中仍有未提交内容。强制删除后，这些内容不会保留。",
				RequiresForce: true,
			}, nil
		}

		return a.handleRemoveWorktreeErrorLocked(repository, targetWorktree, request.Path, removeErr)
	}

	return a.finalizeRemovedWorktreeLocked(repository, targetWorktree, request.Path)
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
		if saveErr := a.saveCurrentSettingsLocked(); saveErr != nil {
			return model.RetryDeleteFolderResult{}, saveErr
		}

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
	if err := a.saveCurrentSettingsLocked(); err != nil {
		return model.RetryDeleteFolderResult{}, err
	}
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
	if err := a.reconcilePendingCleanupsLocked(); err != nil {
		return model.RepositoryView{}, err
	}

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

// removePendingCleanupFolderLocked 删除 Git 已注销后残留的物理目录。
//
// 该分支不会再调用 Git，只负责把残留目录从磁盘和持久化待清理列表中移除。
func (a *App) removePendingCleanupFolderLocked(repository model.RepositoryBinding, worktree model.WorktreeInfo) (model.RemoveWorktreeResult, error) {
	if err := a.deleteFolderLocked(worktree.Path); err != nil {
		entry := a.pendingCleanup[filepath.Clean(worktree.Path)]
		entry.RepositoryID = repository.ID
		entry.Path = filepath.Clean(worktree.Path)
		if strings.TrimSpace(entry.Branch) == "" {
			entry.Branch = worktree.Branch
		}
		if strings.TrimSpace(entry.Head) == "" {
			entry.Head = worktree.Head
		}
		entry.LastError = err.Error()
		a.pendingCleanup[filepath.Clean(worktree.Path)] = entry
		if saveErr := a.saveCurrentSettingsLocked(); saveErr != nil {
			return model.RemoveWorktreeResult{}, saveErr
		}

		view, viewErr := a.buildRepositoryViewLocked(repository)
		if viewErr != nil {
			return model.RemoveWorktreeResult{}, viewErr
		}

		return model.RemoveWorktreeResult{
			Success: false,
			Stage:   model.RemoveStageFolderBusy,
			Message: fmt.Sprintf("残留目录仍被占用：%v", err),
			View:    view,
		}, nil
	}

	delete(a.pendingCleanup, filepath.Clean(worktree.Path))
	if err := a.saveCurrentSettingsLocked(); err != nil {
		return model.RemoveWorktreeResult{}, err
	}

	view, viewErr := a.buildRepositoryViewLocked(repository)
	if viewErr != nil {
		return model.RemoveWorktreeResult{}, viewErr
	}

	return model.RemoveWorktreeResult{
		Success: true,
		Stage:   model.RemoveStageRemoved,
		Message: "残留目录已删除。",
		View:    view,
	}, nil
}

// removeMissingWorktreeLocked 删除“Git 记录仍在但目录已缺失”的 worktree。
//
// 该分支只移除 Git worktree 记录，不再尝试物理删除目录。
func (a *App) removeMissingWorktreeLocked(repository model.RepositoryBinding, worktree model.WorktreeInfo) (model.RemoveWorktreeResult, error) {
	if err := a.gitService.RemoveWorktree(repository, worktree.Path, true); err != nil {
		return model.RemoveWorktreeResult{
			Success: false,
			Stage:   model.RemoveStageGitFailed,
			Message: err.Error(),
		}, nil
	}

	delete(a.pendingCleanup, filepath.Clean(worktree.Path))
	if err := a.saveCurrentSettingsLocked(); err != nil {
		return model.RemoveWorktreeResult{}, err
	}

	view, viewErr := a.buildRepositoryViewLocked(repository)
	if viewErr != nil {
		return model.RemoveWorktreeResult{}, viewErr
	}

	return model.RemoveWorktreeResult{
		Success: true,
		Stage:   model.RemoveStageRemoved,
		Message: "Git Worktree 记录已移除。",
		View:    view,
	}, nil
}

// handleRemoveWorktreeErrorLocked 处理 Git remove 返回错误后的分流逻辑。
//
// 若 Git 记录实际上已经被移除，则继续走物理目录收尾；否则按普通 Git 失败返回。
func (a *App) handleRemoveWorktreeErrorLocked(
	repository model.RepositoryBinding,
	worktree model.WorktreeInfo,
	path string,
	removeErr error,
) (model.RemoveWorktreeResult, error) {
	stillRegistered, err := a.isWorktreeRegisteredLocked(repository, path)
	if err != nil {
		return model.RemoveWorktreeResult{}, err
	}

	if stillRegistered {
		return model.RemoveWorktreeResult{
			Success: false,
			Stage:   model.RemoveStageGitFailed,
			Message: removeErr.Error(),
		}, nil
	}

	return a.finalizeRemovedWorktreeLocked(repository, worktree, path)
}

// finalizeRemovedWorktreeLocked 在 Git 记录已移除后完成目录收尾。
//
// 目录删除成功时直接返回成功；若目录仍被占用，则写入待清理记录并返回异常卡片。
func (a *App) finalizeRemovedWorktreeLocked(
	repository model.RepositoryBinding,
	worktree model.WorktreeInfo,
	path string,
) (model.RemoveWorktreeResult, error) {
	if err := a.deleteFolderLocked(path); err != nil {
		a.pendingCleanup[filepath.Clean(path)] = model.PendingCleanup{
			RepositoryID: repository.ID,
			Path:         filepath.Clean(path),
			Branch:       worktree.Branch,
			Head:         worktree.Head,
			LastError:    err.Error(),
		}
		if saveErr := a.saveCurrentSettingsLocked(); saveErr != nil {
			return model.RemoveWorktreeResult{}, saveErr
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

	delete(a.pendingCleanup, filepath.Clean(path))
	if err := a.saveCurrentSettingsLocked(); err != nil {
		return model.RemoveWorktreeResult{}, err
	}
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

// reconcilePendingCleanupsLocked 清理已经被外部手动删除的残留目录记录。
//
// 若发现待清理目录已经不存在，则会同步移除持久化记录，避免刷新后继续显示失效卡片。
func (a *App) reconcilePendingCleanupsLocked() error {
	changed := false
	for cleanupPath := range a.pendingCleanup {
		if _, err := os.Stat(cleanupPath); err == nil {
			continue
		} else if os.IsNotExist(err) {
			delete(a.pendingCleanup, cleanupPath)
			changed = true
		}
	}

	if !changed {
		return nil
	}

	return a.saveCurrentSettingsLocked()
}

// saveCurrentSettingsLocked 把当前内存配置与待清理记录一起落盘。
//
// 调用方必须持有 a.mu。该方法会先把 map 形态的待清理列表同步回 Settings。
func (a *App) saveCurrentSettingsLocked() error {
	a.settings.PendingCleanups = pendingCleanupSlice(a.pendingCleanup)
	return a.configService.Save(a.settings)
}

// pendingCleanupMapFromSlice 把持久化切片转换为按路径索引的运行态映射。
func pendingCleanupMapFromSlice(cleanups []model.PendingCleanup) map[string]model.PendingCleanup {
	result := map[string]model.PendingCleanup{}
	for _, cleanup := range cleanups {
		cleanPath := filepath.Clean(cleanup.Path)
		if cleanPath == "." || strings.TrimSpace(cleanup.Path) == "" {
			continue
		}

		cleanup.Path = cleanPath
		result[cleanPath] = cleanup
	}

	return result
}

// pendingCleanupSlice 把运行态映射转换为稳定排序的持久化切片。
func pendingCleanupSlice(cleanups map[string]model.PendingCleanup) []model.PendingCleanup {
	keys := make([]string, 0, len(cleanups))
	for cleanupPath := range cleanups {
		keys = append(keys, cleanupPath)
	}
	sort.Strings(keys)

	result := make([]model.PendingCleanup, 0, len(keys))
	for _, cleanupPath := range keys {
		result = append(result, cleanups[cleanupPath])
	}

	return result
}

// worktreeByPath 根据物理路径从当前视图中查找目标卡片。
func worktreeByPath(worktrees []model.WorktreeInfo, path string) (model.WorktreeInfo, bool) {
	cleanPath := filepath.Clean(path)
	for _, worktree := range worktrees {
		if filepath.Clean(worktree.Path) == cleanPath {
			return worktree, true
		}
	}

	return model.WorktreeInfo{}, false
}

// isWorktreeRegisteredLocked 判断目标路径是否仍然存在于 Git worktree 列表中。
func (a *App) isWorktreeRegisteredLocked(repository model.RepositoryBinding, path string) (bool, error) {
	worktrees, err := a.gitService.ListWorktrees(repository)
	if err != nil {
		return false, err
	}

	_, found := worktreeByPath(worktrees, path)
	return found, nil
}
