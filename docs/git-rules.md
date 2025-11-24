# Git 工作流规范

> Flow Detection 项目 Git 协作规范文档
> 
> 最后更新：2025-11-24

## 📋 目录

- [1. 概述](#1-概述)
- [2. 分支管理规范](#2-分支管理规范)
- [3. 提交规范](#3-提交规范)
- [4. 代码审查规范](#4-代码审查规范)
- [5. 合并规范](#5-合并规范)
- [6. 标签管理](#6-标签管理)
- [7. 常见场景操作指南](#7-常见场景操作指南)

---

## 1. 概述

### 1.1 目的

本规范旨在为 Flow Detection 项目团队提供统一的 Git 工作流程，确保：
- 代码版本管理的规范性和可追溯性
- 团队协作的高效性
- 代码质量的稳定性
- 项目历史的清晰性

### 1.2 适用范围

本规范适用于所有参与 Flow Detection 项目开发的团队成员。

### 1.3 基本原则

- **主干稳定**：`main` 分支始终保持可部署状态
- **功能隔离**：每个功能在独立分支上开发
- **及时同步**：定期从主分支拉取最新代码
- **小步提交**：提交粒度适中，每次提交解决一个明确的问题
- **清晰描述**：提交信息清晰明确，便于追溯

---

## 2. 分支管理规范

### 2.1 分支模型

本项目采用 **Git Flow 简化模型**，包含以下分支类型：

```
main (主分支)
  ↓
develop (开发分支)
  ↓
feature/* (功能分支)
hotfix/* (热修复分支)
release/* (发布分支)
```

### 2.2 分支说明

#### 2.2.1 主分支 (main)

- **用途**：生产环境代码，始终保持稳定可发布状态
- **保护**：设置分支保护，禁止直接推送
- **合并来源**：仅接受来自 `release/*` 和 `hotfix/*` 的合并
- **标签**：每次合并到 main 后需打版本标签

#### 2.2.2 开发分支 (develop)

- **用途**：日常开发的主分支，包含最新的开发进度
- **保护**：设置分支保护，要求 PR 审查
- **合并来源**：接受来自 `feature/*` 的合并
- **同步**：定期从 main 同步稳定代码

#### 2.2.3 功能分支 (feature/*)

- **命名规范**：`feature/功能描述` 或 `feature/issue编号-功能描述`
  - 示例：`feature/flow-detection-algorithm`
  - 示例：`feature/123-add-user-authentication`
- **生命周期**：功能开发完成并合并后删除
- **基于分支**：从 `develop` 创建
- **合并目标**：合并回 `develop`

**示例**：
```bash
# 创建功能分支
git checkout develop
git pull origin develop
git checkout -b feature/flow-detection-algorithm

# 开发完成后合并
git checkout develop
git pull origin develop
git merge --no-ff feature/flow-detection-algorithm
git push origin develop
git branch -d feature/flow-detection-algorithm
```

#### 2.2.4 发布分支 (release/*)

- **命名规范**：`release/版本号`
  - 示例：`release/v1.0.0`
- **用途**：准备发布版本，进行最后的测试和bug修复
- **基于分支**：从 `develop` 创建
- **合并目标**：合并到 `main` 和 `develop`
- **生命周期**：发布完成后删除

**流程**：
```bash
# 创建发布分支
git checkout develop
git pull origin develop
git checkout -b release/v1.0.0

# 在 release 分支进行版本号更新、文档更新等

# 发布完成后合并到 main
git checkout main
git merge --no-ff release/v1.0.0
git tag -a v1.0.0 -m "Release version 1.0.0"
git push origin main --tags

# 同时合并回 develop
git checkout develop
git merge --no-ff release/v1.0.0
git push origin develop

# 删除发布分支
git branch -d release/v1.0.0
```

#### 2.2.5 热修复分支 (hotfix/*)

- **命名规范**：`hotfix/问题描述` 或 `hotfix/issue编号-问题描述`
  - 示例：`hotfix/fix-memory-leak`
  - 示例：`hotfix/456-fix-critical-bug`
- **用途**：紧急修复生产环境问题
- **基于分支**：从 `main` 创建
- **合并目标**：合并到 `main` 和 `develop`
- **生命周期**：修复完成并合并后删除

**流程**：
```bash
# 创建热修复分支
git checkout main
git pull origin main
git checkout -b hotfix/fix-memory-leak

# 修复完成后合并到 main
git checkout main
git merge --no-ff hotfix/fix-memory-leak
git tag -a v1.0.1 -m "Hotfix: fix memory leak"
git push origin main --tags

# 同时合并回 develop
git checkout develop
git merge --no-ff hotfix/fix-memory-leak
git push origin develop

# 删除热修复分支
git branch -d hotfix/fix-memory-leak
```

### 2.3 分支命名规则

- 使用小写字母和连字符（-）
- 简洁明了，能够清楚表达分支用途
- 可包含 issue 编号，便于追踪
- 避免使用特殊字符和空格

**✅ 良好示例**：
- `feature/packet-capture`
- `feature/123-add-logging`
- `hotfix/fix-connection-timeout`
- `release/v2.1.0`

**❌ 不良示例**：
- `Feature/Packet_Capture`（大写字母和下划线）
- `fix-bug`（不明确是什么类型的分支）
- `temp`（临时分支命名不规范）
- `张三的分支`（使用中文和特殊字符）

---

## 3. 提交规范

### 3.1 提交信息格式

采用 **Conventional Commits** 规范，格式如下：

```
<type>(<scope>): <subject>

<body>

<footer>
```

#### 3.1.1 type（必填）

提交类型，使用以下关键字：

| Type | 说明 | 示例 |
|------|------|------|
| feat | 新功能 | feat: 添加流量包捕获功能 |
| fix | Bug 修复 | fix: 修复内存泄漏问题 |
| docs | 文档修改 | docs: 更新 API 文档 |
| style | 代码格式调整（不影响功能） | style: 格式化代码缩进 |
| refactor | 代码重构 | refactor: 重构流量分析模块 |
| perf | 性能优化 | perf: 优化数据包处理速度 |
| test | 测试相关 | test: 添加单元测试 |
| chore | 构建/工具链修改 | chore: 更新依赖包版本 |
| ci | CI/CD 配置修改 | ci: 添加 GitHub Actions 配置 |
| revert | 回滚提交 | revert: 回滚 commit abc123 |

#### 3.1.2 scope（可选）

影响范围，表示此次修改涉及的模块或组件：

- `core`：核心模块
- `detector`：检测模块
- `analyzer`：分析模块
- `api`：API 接口
- `ui`：用户界面
- `config`：配置
- `deps`：依赖
- `*`：影响多个模块

#### 3.1.3 subject（必填）

简短描述，不超过 50 个字符：

- 使用祈使句，现在时态
- 第一个字母小写
- 结尾不加句号
- 清晰表达做了什么

#### 3.1.4 body（可选）

详细描述，解释：
- **为什么**进行这次修改
- **如何**解决问题
- 有什么**影响**或**副作用**

#### 3.1.5 footer（可选）

关联信息：
- 关闭 Issue：`Closes #123` 或 `Fixes #456`
- 破坏性变更：`BREAKING CHANGE: 说明`
- 关联 PR：`Related PR: #789`

### 3.2 提交信息示例

#### 示例 1：简单功能提交
```
feat(detector): 添加 TCP 流量检测功能

实现了基于 TCP 协议的流量检测算法，支持：
- 连接状态跟踪
- 数据包计数
- 异常流量识别

Closes #123
```

#### 示例 2：Bug 修复
```
fix(analyzer): 修复多线程环境下的数据竞争问题

在高并发场景下，流量分析器存在数据竞争导致的崩溃问题。
通过添加互斥锁保护共享数据结构解决此问题。

Fixes #456
```

#### 示例 3：文档更新
```
docs: 更新安装指南

- 添加 Windows 平台安装步骤
- 更新依赖库版本说明
- 修正配置文件示例中的错误
```

#### 示例 4：破坏性变更
```
refactor(api): 重构 REST API 接口

将 API 版本从 v1 升级到 v2，优化接口设计。

BREAKING CHANGE: 
- /api/flows 端点更改为 /api/v2/flows
- 响应格式从 XML 改为 JSON
- 需要更新客户端代码以适配新接口
```

### 3.3 提交最佳实践

#### 3.3.1 提交粒度

- **单一职责**：每次提交只做一件事
- **原子性**：提交应该是完整的、可编译的、可测试的
- **适度大小**：避免过大或过小的提交

**✅ 良好实践**：
```bash
git commit -m "feat(detector): 添加 HTTP 协议支持"
git commit -m "test(detector): 添加 HTTP 检测单元测试"
git commit -m "docs(detector): 更新 HTTP 检测文档"
```

**❌ 不良实践**：
```bash
# 一次提交包含多个不相关的修改
git commit -m "添加功能、修复bug、更新文档"
```

#### 3.3.2 提交频率

- 每完成一个小功能点就提交
- 至少每天提交一次工作进度
- 下班前确保代码已提交到远程分支

#### 3.3.3 提交前检查

```bash
# 1. 查看修改内容
git status
git diff

# 2. 添加文件到暂存区
git add <file>
# 或添加所有修改（谨慎使用）
git add .

# 3. 检查暂存区内容
git diff --staged

# 4. 提交
git commit -m "feat: 提交信息"

# 5. 推送到远程
git push origin <branch-name>
```

### 3.4 修改历史提交

#### 3.4.1 修改最后一次提交

```bash
# 修改提交信息
git commit --amend -m "新的提交信息"

# 添加遗漏的文件
git add forgotten-file.py
git commit --amend --no-edit
```

#### 3.4.2 合并多个提交

使用交互式 rebase：

```bash
# 合并最近 3 次提交
git rebase -i HEAD~3

# 在编辑器中将要合并的提交标记为 squash 或 fixup
# squash: 保留提交信息
# fixup: 丢弃提交信息
```

**注意**：⚠️ 不要修改已推送到公共分支的提交历史

---

## 4. 代码审查规范

### 4.1 Pull Request (PR) 流程

#### 4.1.1 创建 PR

1. 确保代码已通过本地测试
2. 推送分支到远程仓库
3. 在 GitHub/GitLab 上创建 Pull Request
4. 填写 PR 模板（见 4.2）
5. 指定审查者（Reviewer）

#### 4.1.2 PR 命名规范

- 使用清晰的标题，概括主要变更
- 可以使用 emoji 增强可读性（可选）

**示例**：
- `[Feature] 添加流量统计功能`
- `[Fix] 修复内存泄漏问题`
- `[Refactor] 重构数据处理模块`
- `🚀 [Feature] 实现实时流量监控`

### 4.2 PR 描述模板

创建文件 `.github/pull_request_template.md`：

```markdown
## 变更类型
<!-- 选择适用的类型 -->
- [ ] 新功能 (Feature)
- [ ] Bug 修复 (Fix)
- [ ] 代码重构 (Refactor)
- [ ] 性能优化 (Performance)
- [ ] 文档更新 (Documentation)
- [ ] 测试 (Test)
- [ ] 其他 (Other)

## 变更描述
<!-- 简要描述本次变更的内容 -->


## 关联 Issue
<!-- 关联的 Issue 编号，例如：Closes #123 -->


## 变更详情
<!-- 详细说明技术实现、设计决策等 -->


## 测试说明
<!-- 说明如何测试这些变更 -->
- [ ] 单元测试已通过
- [ ] 集成测试已通过
- [ ] 手动测试已完成

## 截图/演示
<!-- 如果适用，添加截图或 GIF 演示 -->


## 检查清单
- [ ] 代码遵循项目编码规范
- [ ] 已添加必要的注释和文档
- [ ] 已添加或更新相关测试
- [ ] 所有测试通过
- [ ] 无 linter 错误
- [ ] 已更新相关文档
- [ ] 无破坏性变更（或已在描述中说明）

## 其他说明
<!-- 其他需要说明的内容 -->

```

### 4.3 代码审查标准

#### 4.3.1 审查者职责

- **及时响应**：24 小时内完成审查
- **仔细检查**：关注代码质量、性能、安全性
- **建设性反馈**：提出具体、可操作的建议
- **沟通友好**：保持专业和尊重的态度

#### 4.3.2 审查要点

**功能性**：
- [ ] 代码是否实现了预期功能
- [ ] 是否存在逻辑错误
- [ ] 边界条件是否处理正确

**代码质量**：
- [ ] 代码是否清晰易读
- [ ] 是否遵循项目编码规范
- [ ] 变量命名是否恰当
- [ ] 是否有重复代码

**性能**：
- [ ] 是否存在性能瓶颈
- [ ] 算法复杂度是否合理
- [ ] 资源使用是否高效

**安全性**：
- [ ] 是否存在安全漏洞
- [ ] 输入验证是否充分
- [ ] 敏感信息是否加密

**测试**：
- [ ] 测试覆盖率是否足够
- [ ] 测试用例是否合理
- [ ] 是否包含边界测试

**文档**：
- [ ] 代码注释是否充分
- [ ] API 文档是否更新
- [ ] README 是否需要更新

### 4.4 审查反馈分类

使用标签标注反馈的重要性：

- `[MUST]`：必须修改，否则不能合并
- `[SHOULD]`：建议修改，但不强制
- `[NITS]`：小问题，可选修改
- `[QUESTION]`：疑问，需要讨论

**示例**：
```
[MUST] 第 45 行存在空指针解引用风险，需要添加空值检查
[SHOULD] 建议将这个函数拆分为更小的函数，提高可读性
[NITS] 变量名 `tmp` 可以改为更有意义的名字
[QUESTION] 为什么选择使用算法 A 而不是算法 B？
```

### 4.5 合并条件

PR 满足以下条件才能合并：

- [ ] 至少 1 位审查者批准（重要功能需要 2 位）
- [ ] 所有 CI/CD 检查通过
- [ ] 没有未解决的 [MUST] 级别反馈
- [ ] 代码冲突已解决
- [ ] 所有讨论已解决或达成共识

---

## 5. 合并规范

### 5.1 合并策略

根据不同场景选择合并策略：

#### 5.1.1 Merge Commit（推荐）

```bash
git merge --no-ff feature/xxx
```

**优点**：
- 保留完整的分支历史
- 清晰显示功能开发过程
- 便于回滚整个功能

**适用场景**：
- 功能分支合并到 develop
- 发布分支合并到 main

#### 5.1.2 Squash and Merge

```bash
git merge --squash feature/xxx
git commit -m "feat: 功能描述"
```

**优点**：
- 简化提交历史
- 将多次提交合并为一次

**适用场景**：
- 功能分支提交历史混乱
- 需要清理提交记录

#### 5.1.3 Rebase and Merge

```bash
git rebase develop
git checkout develop
git merge feature/xxx
```

**优点**：
- 线性历史，易于阅读
- 避免合并提交

**适用场景**：
- 个人开发分支
- 提交历史需要保持线性

**注意**：⚠️ 不要对已推送到公共分支的提交进行 rebase

### 5.2 解决冲突

#### 5.2.1 冲突预防

```bash
# 开发过程中定期同步主分支
git checkout feature/xxx
git fetch origin
git merge origin/develop
```

#### 5.2.2 冲突解决流程

```bash
# 1. 拉取最新代码
git fetch origin

# 2. 合并目标分支
git merge origin/develop

# 3. 查看冲突文件
git status

# 4. 手动解决冲突
# 编辑冲突文件，选择保留哪些修改

# 5. 标记为已解决
git add <resolved-files>

# 6. 完成合并
git commit

# 7. 推送
git push origin feature/xxx
```

#### 5.2.3 冲突解决原则

- 理解双方的修改意图
- 保留所有必要的功能
- 测试解决后的代码
- 必要时与相关开发者讨论

### 5.3 合并前检查清单

- [ ] 代码已通过所有测试
- [ ] 已解决所有代码审查意见
- [ ] 已更新相关文档
- [ ] 已解决所有冲突
- [ ] CI/CD 检查通过
- [ ] 已获得必要的审批

---

## 6. 标签管理

### 6.1 版本号规范

采用 **语义化版本**（Semantic Versioning）：`v主版本号.次版本号.修订号`

格式：`vMAJOR.MINOR.PATCH`

- **MAJOR**：重大更新，不兼容的 API 变更
- **MINOR**：向下兼容的功能新增
- **PATCH**：向下兼容的问题修正

**示例**：
- `v1.0.0`：首个稳定版本
- `v1.1.0`：添加新功能
- `v1.1.1`：修复 bug
- `v2.0.0`：重大更新，API 不兼容

### 6.2 预发布版本

- `v1.0.0-alpha.1`：内部测试版
- `v1.0.0-beta.1`：公开测试版
- `v1.0.0-rc.1`：候选发布版

### 6.3 创建标签

```bash
# 创建轻量标签
git tag v1.0.0

# 创建附注标签（推荐）
git tag -a v1.0.0 -m "Release version 1.0.0"

# 为历史提交打标签
git tag -a v0.9.0 <commit-hash> -m "Version 0.9.0"

# 推送标签到远程
git push origin v1.0.0

# 推送所有标签
git push origin --tags
```

### 6.4 标签管理

```bash
# 查看所有标签
git tag

# 查看标签详情
git show v1.0.0

# 删除本地标签
git tag -d v1.0.0

# 删除远程标签
git push origin --delete v1.0.0

# 检出标签
git checkout v1.0.0
```

---

## 7. 常见场景操作指南

### 7.1 开始新功能开发

```bash
# 1. 切换到 develop 分支并更新
git checkout develop
git pull origin develop

# 2. 创建功能分支
git checkout -b feature/flow-analysis

# 3. 开发并提交
# ... 进行开发 ...
git add .
git commit -m "feat(analyzer): 添加流量分析功能"

# 4. 推送到远程
git push origin feature/flow-analysis

# 5. 创建 Pull Request
# 在 GitHub/GitLab 上创建 PR
```

### 7.2 同步远程分支

```bash
# 方法 1：Fetch + Merge
git fetch origin
git merge origin/develop

# 方法 2：Pull（相当于 fetch + merge）
git pull origin develop

# 方法 3：Pull with Rebase
git pull --rebase origin develop
```

### 7.3 撤销操作

#### 7.3.1 撤销工作区修改

```bash
# 撤销单个文件
git checkout -- <file>

# 撤销所有修改
git checkout -- .
```

#### 7.3.2 撤销暂存区修改

```bash
# 取消暂存
git reset HEAD <file>

# 取消所有暂存
git reset HEAD
```

#### 7.3.3 撤销提交

```bash
# 撤销最后一次提交，保留修改
git reset --soft HEAD~1

# 撤销最后一次提交，不保留修改
git reset --hard HEAD~1

# 创建新提交来撤销历史提交（推荐）
git revert <commit-hash>
```

### 7.4 暂存当前工作

```bash
# 暂存当前修改
git stash save "工作进度描述"

# 查看暂存列表
git stash list

# 恢复最近的暂存
git stash pop

# 恢复特定暂存
git stash apply stash@{0}

# 删除暂存
git stash drop stash@{0}

# 清空所有暂存
git stash clear
```

### 7.5 查看历史

```bash
# 查看提交历史
git log

# 简洁格式
git log --oneline

# 图形化显示
git log --graph --oneline --all

# 查看某个文件的历史
git log -- <file>

# 查看某个作者的提交
git log --author="作者名"

# 查看特定时间范围
git log --since="2 weeks ago"
```

### 7.6 cherry-pick 特定提交

```bash
# 将特定提交应用到当前分支
git cherry-pick <commit-hash>

# 应用多个提交
git cherry-pick <commit1> <commit2>

# 应用提交范围
git cherry-pick <start-commit>..<end-commit>
```

### 7.7 清理本地分支

```bash
# 查看已合并的分支
git branch --merged

# 删除本地分支
git branch -d feature/xxx

# 强制删除（未合并的分支）
git branch -D feature/xxx

# 删除远程分支
git push origin --delete feature/xxx

# 清理已删除的远程分支引用
git fetch --prune
```

---

## 📝 附录

### A. Git 配置建议

```bash
# 设置用户信息
git config --global user.name "你的名字"
git config --global user.email "your.email@example.com"

# 设置默认编辑器
git config --global core.editor "vim"

# 设置默认分支名
git config --global init.defaultBranch main

# 启用颜色输出
git config --global color.ui auto

# 设置别名
git config --global alias.st status
git config --global alias.co checkout
git config --global alias.br branch
git config --global alias.cm commit
git config --global alias.lg "log --graph --oneline --all"
```

### B. .gitignore 模板

根据项目类型添加适当的 `.gitignore` 文件：

```gitignore
# Python
__pycache__/
*.py[cod]
*$py.class
*.so
.Python
env/
venv/
ENV/
.venv

# IDE
.vscode/
.idea/
*.swp
*.swo
*~

# OS
.DS_Store
Thumbs.db

# Project specific
logs/
*.log
data/
temp/
```

### C. 有用的 Git 工具

- **GitKraken**：可视化 Git 客户端
- **SourceTree**：免费的 Git GUI
- **Git LFS**：大文件存储
- **pre-commit**：Git hooks 管理
- **commitizen**：规范化提交信息

### D. 参考资料

- [Git 官方文档](https://git-scm.com/doc)
- [Conventional Commits](https://www.conventionalcommits.org/)
- [语义化版本](https://semver.org/lang/zh-CN/)
- [Git Flow 工作流](https://nvie.com/posts/a-successful-git-branching-model/)

---

## 📌 版本历史

| 版本 | 日期 | 修改内容 | 作者 |
|------|------|----------|------|
| v1.0.0 | 2025-11-24 | 初始版本，创建 Git 工作流规范 | AI Assistant |

---

## 🤝 反馈与建议

如果您对本规范有任何建议或发现需要改进的地方，请通过以下方式反馈：

1. 创建 Issue
2. 提交 Pull Request
3. 联系项目维护者

---

> **注意**：本规范是团队协作的基础，请所有成员严格遵守。规范会根据项目实际情况持续优化和更新。

