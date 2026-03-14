# Repository Guidelines

## 项目结构与模块组织
`main.go` 和 `app.go` 是 Wails 应用入口与前后端桥接层。核心后端逻辑放在 `internal/`，其中 `config/` 处理本地配置，`git/` 处理 worktree，`launcher/` 与 `environment/` 负责外部工具和环境检查，`model/` 定义共享数据结构。前端位于 `frontend/src/`，组件在 `components/`，工具函数在 `lib/`。`frontend/wailsjs/`、`frontend/dist/`、`build/bin/` 和根目录打包产物都属于生成物，不要手改。静态资源在 `assets/`，Wails/Windows 打包资源在 `build/windows/` 与 `build/appicon.png`。

## 产品行为约束
本项目是一个面向 Windows 的 Git worktree 桌面管理器，不是通用 Git GUI。后续修改必须优先保护现有产品语义，而不是追求表面上的“更通用”或“更简化”。

Worktree 视图必须保留 `normal`、`missing`、`pending_cleanup` 三种状态，以及删除流程中的 `removed`、`dirty_blocked`、`git_failed`、`folder_busy` 结果语义。不要把删除流程改写成单一的成功/失败布尔值，也不要静默吞掉“Git 记录已删除但目录仍被占用”这类中间状态。

`PendingCleanups` 是正式产品数据，不是临时缓存。凡是 Git 记录已移除但物理目录尚未清理完成的场景，都要继续写回配置并在界面中展示，直到目录被成功删除或确认已不存在。修改配置保存、仓库列表、仓库视图或删除逻辑时，必须检查这条链路是否仍然成立。

仓库解绑只影响本地配置，不应删除任何 Git 数据或物理目录。缺失目录的 worktree 只移除 Git 记录；`pending_cleanup` 卡片只重试物理删除；脏目录的删除需要明确二次确认。不要把这些分支合并成同一种行为。

## 平台与路径约束
项目以 Windows 为一等平台设计。推荐路径生成、终端打开、资源管理器打开、提权启动和外部 CLI 拉起都按 Windows 语义实现。除非用户明确要求，不要主动把这些行为抽象成跨平台通用方案。

修改平台相关逻辑时，同步检查 `*_windows.go` 与 `*_other.go`。其中 `*_other.go` 主要用于保持跨平台编译通过，不代表 macOS 或 Linux 已具备与 Windows 对等的产品能力；不要因为存在 `*_other.go` 就假设项目已经完成跨平台设计。

前端推荐 worktree 路径当前明确使用 Windows 路径拼接规则。不要擅自把这部分改成 `path.posix`、URL 风格路径或所谓“跨平台统一分隔符”。

## 构建、测试与开发命令
首次进入前端目录执行 `npm.cmd install`。日常开发优先使用 `wails dev`，它会启动 Go 后端并连接 Vite 开发服务器。单独验证前端可用 `npm.cmd run build`；该命令会先执行 `tsc -b` 再进行 Vite 打包。后端测试使用 `go test ./...`，前端测试使用 `npm.cmd test`。Windows 本地打包可运行 `build_windows.cmd`，CI 与发布流程使用 `wails build -clean`。

## 编码风格与命名约定
Go 代码统一交给 `gofmt`，包名保持小写短名，导出标识符使用 `PascalCase`，内部变量使用 `camelCase`。TypeScript/React 文件保持现有 2 空格缩进；组件文件采用 `PascalCase.tsx`，工具模块采用小写文件名，例如 `formats.ts`。新增类、结构体、函数和方法请补充简体中文注释，优先写清职责、参数、返回值和关键实现说明。

除非已有模式明显不同，前端状态编排优先延续 `App.tsx` 现有的显式状态机写法，不要为了“拆分更细”而把少量局部逻辑过度抽象成难追踪的 hooks 或层层包装。后端错误返回优先保持结构化，便于前端根据阶段做不同提示，而不是把所有错误都压成一条字符串。

外部工具参数保持“命令 + 参数数组模板”的模型；`{path}`、`{branch}` 占位符由后端展开。不要把它改成整串 shell 命令拼接，也不要引入依赖 shell 解析的实现。

## Wails 前端约束
前端需要支持 fresh clone 场景下独立构建。除非用户明确要求，不要引入对生成版 `frontend/wailsjs/` 或 `wailsjs/runtime` 的硬依赖；当前后端调用和运行时拖拽桥接应继续保持在生成文件缺失时也能通过前端构建。

目录拖拽优先使用 Wails 原生 `OnFileDrop` / `OnFileDropOff`，因为它能稳定拿到系统绝对路径。不要为了追求“纯 Web 写法”改回只依赖 DOM `drag/drop` 的实现。

## 测试约定
Go 测试文件命名为 `*_test.go`，前端测试命名为 `*.test.ts`，当前使用 Vitest + `jsdom`。提交前至少覆盖你改动涉及的成功路径、错误路径和边界条件；只改格式化或路径处理逻辑时，优先补充 `frontend/src/lib/` 或对应 `internal/` 包下的单元测试。仓库没有单独覆盖率门槛，但不要在无测试的情况下合并行为变更。

如果修改了删除流程、配置归一化、外部工具参数展开、Windows Terminal 启动、PowerShell 命令拼接或 worktree 状态解析，默认应补对应单元测试。涉及 `pending_cleanup`、缺失目录、脏目录强删确认的变更时，测试应覆盖成功路径和失败路径，而不是只测 happy path。

## 提交与 Pull Request 约定
历史提交以简短、直接的中文说明为主，例如“添加示例图片”；基础设施改动可使用 `ci:` 这类前缀。每个提交只做一件事，标题使用祈使句，避免把重构、功能和资源更新混在一起。PR 需要写清变更目的、主要影响和验证方式；涉及界面改动时附截图或录屏，涉及发布流程时说明是否影响 `v*` 标签触发的 Windows 构建。

## 配置与安全提示
不要提交个人工作目录、真实仓库路径或临时生成的本地配置。开发模式下，配置默认优先落在仓库根目录的 `config.json`；打包后默认落在可执行文件同级目录。除非用户明确要求，不要擅自把配置迁移到用户目录、注册表或其他新位置。

涉及外部工具命令、Git 路径或文件删除的改动时，优先保持默认值保守，并在说明中写明平台前提和回退行为。Windows 下外部工具和 Terminal 当前会走提权启动链路，并通过 `wt.exe` + PowerShell 承载命令；修改这部分逻辑时，必须明确评估 UAC、工作目录、参数转义和 CLI 退出后窗口保留行为。
