package git

import (
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"treeshift/internal/executil"
	"treeshift/internal/model"
)

// Service 封装所有 Git worktree 相关命令。
//
// 所有命令均通过参数数组调用，不经过 shell，避免路径和参数转义问题。
type Service struct{}

// NewService 创建 Git 服务实例。
func NewService() *Service {
	return &Service{}
}

// ResolveRepository 解析用户传入路径对应的 Git 仓库。
//
// 即使传入的是某个链接 worktree，也会统一解析出 common dir 和主工作区路径，用于仓库去重。
func (s *Service) ResolveRepository(path string) (model.RepositoryBinding, error) {
	cleanPath := filepath.Clean(strings.TrimSpace(path))
	if cleanPath == "." || strings.TrimSpace(path) == "" {
		return model.RepositoryBinding{}, errors.New("仓库路径不能为空")
	}

	stat, err := os.Stat(cleanPath)
	if err != nil {
		return model.RepositoryBinding{}, fmt.Errorf("读取仓库路径失败：%w", err)
	}
	if !stat.IsDir() {
		return model.RepositoryBinding{}, errors.New("绑定路径必须是目录")
	}

	topLevel, err := s.runGitSingleValue(cleanPath, "rev-parse", "--show-toplevel")
	if err != nil {
		return model.RepositoryBinding{}, fmt.Errorf("当前目录不是 Git 工作区：%w", err)
	}

	commonDir, err := s.runGitSingleValue(cleanPath, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return model.RepositoryBinding{}, fmt.Errorf("解析 Git common dir 失败：%w", err)
	}
	if !filepath.IsAbs(commonDir) {
		commonDir = filepath.Join(topLevel, commonDir)
	}

	worktrees, err := s.listWorktreesByPath(cleanPath)
	if err != nil {
		return model.RepositoryBinding{}, err
	}

	mainWorktreePath := filepath.Clean(topLevel)
	if len(worktrees) > 0 {
		mainWorktreePath = filepath.Clean(worktrees[0].Path)
	}

	return model.RepositoryBinding{
		ID:               stableID(commonDir),
		DisplayName:      filepath.Base(mainWorktreePath),
		SelectedPath:     cleanPath,
		MainWorktreePath: mainWorktreePath,
		CommonDir:        filepath.Clean(commonDir),
	}, nil
}

// ListWorktrees 返回当前仓库登记的全部 worktree。
func (s *Service) ListWorktrees(repository model.RepositoryBinding) ([]model.WorktreeInfo, error) {
	return s.listWorktreesByPath(repository.MainWorktreePath)
}

// ListBranches 列出仓库当前可见的本地分支。
//
// v1 只暴露本地分支，不主动 fetch，也不把远程分支直接放进创建向导。
func (s *Service) ListBranches(repository model.RepositoryBinding) ([]string, error) {
	output, err := s.runGit(repository.MainWorktreePath, "for-each-ref", "--format=%(refname:short)", "refs/heads")
	if err != nil {
		return nil, err
	}

	lines := splitLines(output)
	branches := make([]string, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			branches = append(branches, strings.TrimSpace(line))
		}
	}

	sort.Strings(branches)
	return branches, nil
}

// CreateWorktree 根据前端请求创建新的 worktree。
//
// detached 模式基于已有本地分支创建游离 HEAD worktree；
// existing 模式直接检出已有本地分支；
// new 模式基于 SourceBranch 新建本地分支后创建。
func (s *Service) CreateWorktree(repository model.RepositoryBinding, request model.CreateWorktreeRequest) error {
	targetPath := filepath.Clean(strings.TrimSpace(request.TargetPath))
	if strings.TrimSpace(targetPath) == "" || targetPath == "." {
		return errors.New("目标路径不能为空")
	}

	if err := s.ensureTargetPathAvailable(targetPath); err != nil {
		return err
	}

	switch request.Mode {
	case "detached":
		sourceBranch := strings.TrimSpace(request.SourceBranch)
		if sourceBranch == "" {
			return errors.New("请选择游离 HEAD 的来源分支")
		}
		if err := s.ensureLocalBranchExists(repository, sourceBranch); err != nil {
			return err
		}

		_, err := s.runGit(repository.MainWorktreePath, "worktree", "add", "--detach", targetPath, sourceBranch)
		return err
	case "existing":
		sourceBranch := strings.TrimSpace(request.SourceBranch)
		if sourceBranch == "" {
			return errors.New("请选择要检出的现有分支")
		}
		if err := s.ensureLocalBranchExists(repository, sourceBranch); err != nil {
			return err
		}

		_, err := s.runGit(repository.MainWorktreePath, "worktree", "add", targetPath, sourceBranch)
		return err
	case "new":
		sourceBranch := strings.TrimSpace(request.SourceBranch)
		branchName := strings.TrimSpace(request.BranchName)
		if sourceBranch == "" {
			return errors.New("请选择新分支的基线分支")
		}
		if err := s.ensureLocalBranchExists(repository, sourceBranch); err != nil {
			return err
		}
		if branchName == "" {
			return errors.New("请输入新分支名称")
		}
		if err := s.validateBranchName(repository, branchName); err != nil {
			return err
		}
		exists, err := s.branchExists(repository, branchName)
		if err != nil {
			return err
		}
		if exists {
			return fmt.Errorf("本地分支 %s 已存在", branchName)
		}

		_, err = s.runGit(repository.MainWorktreePath, "worktree", "add", "-b", branchName, targetPath, sourceBranch)
		return err
	default:
		return fmt.Errorf("不支持的创建模式：%s", request.Mode)
	}
}

// AttachDetachedWorktree 把游离 HEAD worktree 附着到一个分支。
//
// existing 模式会切换到已有本地分支；
// new 模式会基于当前游离 HEAD 创建新分支并切换过去。
func (s *Service) AttachDetachedWorktree(repository model.RepositoryBinding, request model.AttachDetachedWorktreeRequest) error {
	targetPath := filepath.Clean(strings.TrimSpace(request.Path))
	if strings.TrimSpace(request.Path) == "" || targetPath == "." {
		return errors.New("游离 HEAD Worktree 路径不能为空")
	}

	worktrees, err := s.listWorktreesByPath(repository.MainWorktreePath)
	if err != nil {
		return err
	}

	targetWorktree, found := findWorktreeByPath(worktrees, targetPath)
	if !found {
		return fmt.Errorf("未找到指定 Worktree：%s", request.Path)
	}
	if targetWorktree.Status != model.WorktreeStatusNormal {
		return errors.New("只有正常状态的游离 HEAD Worktree 才能附着分支")
	}
	if !targetWorktree.IsDetached {
		return errors.New("当前 Worktree 并非游离 HEAD 状态")
	}

	branchName := strings.TrimSpace(request.BranchName)
	switch request.Mode {
	case "existing":
		if branchName == "" {
			return errors.New("请选择要切换的现有分支")
		}
		if err := s.ensureLocalBranchExists(repository, branchName); err != nil {
			return err
		}

		_, err = s.runGit(targetWorktree.Path, "switch", branchName)
		return err
	case "new":
		if branchName == "" {
			return errors.New("请输入新分支名称")
		}
		if err := s.validateBranchName(repository, branchName); err != nil {
			return err
		}
		exists, existsErr := s.branchExists(repository, branchName)
		if existsErr != nil {
			return existsErr
		}
		if exists {
			return fmt.Errorf("本地分支 %s 已存在", branchName)
		}

		_, err = s.runGit(targetWorktree.Path, "switch", "-c", branchName)
		return err
	default:
		return fmt.Errorf("不支持的附着模式：%s", request.Mode)
	}
}

// RemoveWorktree 调用 Git 注销某个 worktree。
//
// 该方法只负责 Git 层面的 remove，不负责物理目录删除。
func (s *Service) RemoveWorktree(repository model.RepositoryBinding, path string, force bool) error {
	args := []string{"worktree", "remove"}
	if force {
		args = append(args, "--force")
	}
	args = append(args, filepath.Clean(path))

	_, err := s.runGit(repository.MainWorktreePath, args...)
	return err
}

// IsDirtyRemoveError 判断删除失败是否属于“脏目录拦截”。
func IsDirtyRemoveError(err error) bool {
	if err == nil {
		return false
	}

	message := strings.ToLower(err.Error())
	return strings.Contains(message, "modified or untracked files") ||
		strings.Contains(message, "contains modified") ||
		strings.Contains(message, "use --force to delete it")
}

// ParseWorktreePorcelain 解析 `git worktree list --porcelain` 输出。
//
// 该函数会尽量兼容空行和未知字段，并补充本地目录存在性检查结果。
func ParseWorktreePorcelain(output string) []model.WorktreeInfo {
	lines := splitLines(output)
	worktrees := make([]model.WorktreeInfo, 0)
	current := model.WorktreeInfo{
		Status: model.WorktreeStatusNormal,
	}
	hasEntry := false

	flush := func() {
		if !hasEntry {
			return
		}
		if current.Branch == "" && current.IsDetached {
			current.Branch = "detached"
		}
		if current.Status == "" {
			current.Status = model.WorktreeStatusNormal
		}
		worktrees = append(worktrees, current)
		current = model.WorktreeInfo{
			Status: model.WorktreeStatusNormal,
		}
		hasEntry = false
	}

	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			flush()
			continue
		}

		key := line
		value := ""
		if strings.Contains(line, " ") {
			parts := strings.SplitN(line, " ", 2)
			key = parts[0]
			value = parts[1]
		}

		switch key {
		case "worktree":
			flush()
			current.Path = filepath.Clean(value)
			current.Status = model.WorktreeStatusNormal
			hasEntry = true
		case "HEAD":
			current.Head = value
			hasEntry = true
		case "branch":
			current.Branch = shortenBranch(value)
			hasEntry = true
		case "detached":
			current.IsDetached = true
			current.Branch = "detached"
			hasEntry = true
		case "prunable":
			current.Status = model.WorktreeStatusMissing
			if strings.TrimSpace(value) != "" {
				current.StatusMessage = strings.TrimSpace(value)
			}
			hasEntry = true
		case "locked":
			if strings.TrimSpace(value) != "" {
				current.StatusMessage = strings.TrimSpace(value)
			}
			hasEntry = true
		default:
			// 这里保留对未知字段的兼容性，当前版本无需处理。
		}
	}

	flush()
	if len(worktrees) > 0 {
		worktrees[0].IsMain = true
	}

	for index := range worktrees {
		if _, err := os.Stat(worktrees[index].Path); err != nil && os.IsNotExist(err) {
			worktrees[index].Status = model.WorktreeStatusMissing
			if strings.TrimSpace(worktrees[index].StatusMessage) == "" {
				worktrees[index].StatusMessage = "目录不存在，可能已被外部删除。"
			}
		}
	}

	return worktrees
}

// ensureTargetPathAvailable 验证创建目标目录是否可用。
//
// 空目录允许复用；非空目录或普通文件会被直接拦截。
func (s *Service) ensureTargetPathAvailable(targetPath string) error {
	stat, err := os.Stat(targetPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}

		return err
	}

	if !stat.IsDir() {
		return fmt.Errorf("目标路径 %s 已存在且不是目录", targetPath)
	}

	entries, err := os.ReadDir(targetPath)
	if err != nil {
		return err
	}
	if len(entries) > 0 {
		return fmt.Errorf("目标目录 %s 已存在且非空", targetPath)
	}

	return nil
}

// validateBranchName 使用 Git 官方规则校验分支名是否合法。
func (s *Service) validateBranchName(repository model.RepositoryBinding, branchName string) error {
	if _, err := s.runGit(repository.MainWorktreePath, "check-ref-format", "--branch", branchName); err != nil {
		return fmt.Errorf("分支名无效：%s", branchName)
	}

	return nil
}

// branchExists 检查本地分支是否已经存在。
func (s *Service) branchExists(repository model.RepositoryBinding, branchName string) (bool, error) {
	command := exec.Command("git", "-C", repository.MainWorktreePath, "show-ref", "--verify", "--quiet", "refs/heads/"+branchName)
	executil.PrepareBackgroundCommand(command)

	if err := command.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return false, nil
		}

		return false, fmt.Errorf("检查本地分支是否存在失败：%w", err)
	}

	return true, nil
}

// ensureLocalBranchExists 校验目标本地分支确实存在。
//
// 该校验用于把“分支不存在”转换为更直接的业务错误，
// 避免前端只能看到 Git 原始报错。
func (s *Service) ensureLocalBranchExists(repository model.RepositoryBinding, branchName string) error {
	exists, err := s.branchExists(repository, branchName)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("未找到本地分支 %s", branchName)
	}

	return nil
}

// listWorktreesByPath 以任意工作区路径为入口列出 worktree。
func (s *Service) listWorktreesByPath(path string) ([]model.WorktreeInfo, error) {
	output, err := s.runGit(path, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, err
	}

	worktrees := ParseWorktreePorcelain(output)
	for index := range worktrees {
		if worktrees[index].Status != model.WorktreeStatusNormal {
			continue
		}

		changeSummary, summaryErr := s.readWorktreeChangeSummary(worktrees[index].Path)
		if summaryErr == nil {
			worktrees[index].ChangeSummary = changeSummary
		}
	}

	return worktrees, nil
}

// runGitSingleValue 执行只返回单行结果的 Git 命令。
func (s *Service) runGitSingleValue(path string, args ...string) (string, error) {
	output, err := s.runGit(path, args...)
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(output), nil
}

// readWorktreeChangeSummary 读取单个 worktree 的轻量改动摘要。
//
// 这里使用 `git status --porcelain` 统计文件级变化数量，
// 只为界面提供“+N|-M”提示，不追求替代完整 diff 视图。
func (s *Service) readWorktreeChangeSummary(path string) (model.WorktreeChangeSummary, error) {
	output, err := s.runGit(path, "status", "--porcelain", "--untracked-files=all")
	if err != nil {
		return model.WorktreeChangeSummary{}, err
	}

	return ParseWorktreeChangeSummary(output), nil
}

// runGit 执行 Git 命令并返回合并后的输出文本。
//
// 命令失败时会优先保留 Git 原始输出，便于前端直接展示给用户。
func (s *Service) runGit(path string, args ...string) (string, error) {
	commandArgs := append([]string{"-C", path}, args...)
	command := exec.Command("git", commandArgs...)
	executil.PrepareBackgroundCommand(command)

	output, err := command.CombinedOutput()
	trimmed := strings.TrimSpace(string(output))
	if err != nil {
		if trimmed == "" {
			return "", err
		}

		return "", errors.New(trimmed)
	}

	return trimmed, nil
}

// ParseWorktreeChangeSummary 解析 `git status --porcelain` 输出为轻量摘要。
//
// 解析规则刻意保持简单：只区分“删除类文件”和“其他改动文件”两组，
// 这样前端就能稳定展示 `+N|-M`，同时避免把信息量做得过重。
func ParseWorktreeChangeSummary(output string) model.WorktreeChangeSummary {
	summary := model.WorktreeChangeSummary{}
	for _, line := range splitLines(output) {
		if len(line) < 2 {
			continue
		}

		statusCode := line[:2]
		if statusCode == "  " {
			continue
		}

		if strings.Contains(statusCode, "D") {
			summary.DeletedCount++
		} else {
			summary.ChangedCount++
		}
	}

	return summary
}

// splitLines 把命令输出按行拆分，同时兼容 Windows 换行格式。
func splitLines(value string) []string {
	normalized := strings.ReplaceAll(value, "\r\n", "\n")
	return strings.Split(normalized, "\n")
}

// shortenBranch 把 refs/heads/xxx 转换为更适合 UI 展示的分支名。
func shortenBranch(value string) string {
	if strings.HasPrefix(value, "refs/heads/") {
		return strings.TrimPrefix(value, "refs/heads/")
	}

	return value
}

// stableID 根据 common dir 生成稳定仓库 ID。
func stableID(value string) string {
	sum := sha1.Sum([]byte(value))
	return hex.EncodeToString(sum[:])[:12]
}

// findWorktreeByPath 根据路径在 worktree 列表中定位目标项。
//
// 这里统一使用 filepath.Clean 归一化路径，保证主工作区和链接 worktree
// 在 Windows 路径大小写或分隔符差异下仍能稳定匹配。
func findWorktreeByPath(worktrees []model.WorktreeInfo, targetPath string) (model.WorktreeInfo, bool) {
	cleanTargetPath := filepath.Clean(targetPath)
	for _, worktree := range worktrees {
		if strings.EqualFold(filepath.Clean(worktree.Path), cleanTargetPath) {
			return worktree, true
		}
	}

	return model.WorktreeInfo{}, false
}
