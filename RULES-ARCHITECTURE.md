# Codex Rules Architecture

> 一套为 Codex CLI agent 设计的三级规则体系，覆盖全局 → 项目 → 场景维度。

---

## Overview

```
~/.codex/AGENTS.md          ← 全局兜底（跨项目通用原则）
    │
    └── 项目根目录/
            ├── AGENTS.md          ← 全局行为准则 + 规则索引（自动加载）
            ├── .codex-local.md    ← 项目本地配置（元数据 + 特有约束）
            ├── .codex-rules/      ← 场景规则目录（按需加载）
            │   ├── index.md       ←   规则索引（快速入口）
            │   ├── agent.md       ←   Agent 工作流程
            │   ├── go.md          ←   Go 编码规范
            │   ├── python.md      ←   Python 编码规范
            │   ├── testing.md     ←   单元测试规范
            │   ├── bug-fix.md     ←   Bug 修复工作流
            │   ├── commit.md      ←   提交信息格式
            │   └── plan.md        ←   工程规划路线
            └── init-codex-rules.sh ← 新项目初始化工具
```

---

## Three-Layer Architecture

### Layer 1: Global Rules

| File | Path | Scope | Loaded |
|------|------|-------|--------|
| `AGENTS.md` | `~/.codex/AGENTS.md` | All projects | Automatically |

**Purpose**: 跨项目兜底规则，优先级最低。只放不会变的通用原则。

- 安全默认值（不提交密钥、不删除未指出的文件）
- 记忆管理提醒
- 新项目初始化指引

---

### Layer 2: Project Behavioral Principles

| File | Path | Scope | Loaded |
|------|------|-------|--------|
| `AGENTS.md` | `<project>/AGENTS.md` | Project directory tree | Automatically every turn |

**Purpose**: 定义 Codex agent 在本项目中应该遵循的核心行为准则。

四条准则：

1. **Think Before Coding** — 不假设、不隐藏困惑、澄清后再动
2. **Simplicity First** — 最少代码、不推测性设计、不建无谓抽象
3. **Surgical Changes** — 只动必须动的、匹配现有风格、清理自己的遗留物
4. **Goal-Driven Execution** — 先写测试验证，再实现

此外包含 **规则索引表**——告诉 agent 在不同场景下加载 `.codex-rules/` 中的哪个文件。

---

### Layer 3: Project Context & Scene Rules

| File/Dir | Path | Purpose |
|----------|------|---------|
| `.codex-local.md` | `<project>/.codex-local.md` | 项目本地配置 + 特有代码修改约束 |
| `.codex-rules/index.md` | `<project>/.codex-rules/index.md` | 规则快速索引 |
| `.codex-rules/agent.md` | `.codex-rules/agent.md` | Agent 工作流程（开机必做、记忆管理） |
| `.codex-rules/go.md` | `.codex-rules/go.md` | Go 编码规范（format/vet/命名/导入顺序） |
| `.codex-rules/python.md` | `.codex-rules/python.md` | Python 编码规范（ruff/类型标注/命名） |
| `.codex-rules/testing.md` | `.codex-rules/testing.md` | 测试框架 & 运行方式 |
| `.codex-rules/bug-fix.md` | `.codex-rules/bug-fix.md` | Bug 修复工作流（先红后绿） |
| `.codex-rules/commit.md` | `.codex-rules/commit.md` | 提交信息 & PR 规范 |
| `.codex-rules/plan.md` | `.codex-rules/plan.md` | 持续性工程规划路线 |

#### `.codex-local.md` 详解

这个文件托管在项目仓库中，是**项目独有**的配置。它包含：

1. **项目元数据**
   ```markdown
   **语言**: Go
   **构建**: make build
   **测试**: make test
   **入口**: cmd/chat2responses/main.go
   ```

2. **代码修改约束**（项目特有同步规则）
   - 协议转换：改 `converter.go` 必须同步改 `stream.go`
   - 配置变更：改 `config.go` 同步更新 `config.json.example` 和 `README.md`
   - 数据结构：改 `model/` 时在多文件中同步定义
   - 自查清单：修改后的验证步骤

3. **AI 约束**
   - 单文件建议不超过 300 行（可声明例外 `// codex: max-lines(N)`）
   - 单次修改建议不超过 5 个文件（规则维护豁免）

---

## Loading Precedence

```
Highest  项目 AGENTS.md + .codex-local.md + .codex-rules/
              ↑ 覆盖冲突项
Medium   ~/.codex/AGENTS.md
              ↑ 覆盖冲突项
Lowest   Codex CLI 默认行为
```

**关键规则**：

1. `AGENTS.md` 和 `agent.md` 每次任务自动加载
2. 场景规则按需加载：写 Go 代码时 agent 自动索引到 `go.md` 并加载
3. `~/.codex/AGENTS.md` 优先级低于项目内任何规则文件
4. `.codex-local.md` 中的约束优先于场景规则中的冲突项

---

## Design Rationale

### Why three layers?

| Layer | Problem it solves |
|-------|-------------------|
| Global | 每个项目都要写安全默认值？太啰嗦 |
| Project | 每个项目的行为风格不同（有的追求简洁、有的追求健壮） |
| Scene | 把所有规约写在一个文件里，agent context 被撑爆 |

### Why separate `.codex-rules/` from `AGENTS.md`?

- **AGENTS.md** 是 Codex CLI 的原生入口，每次任务**自动加载** → 只放不变的原则
- **`.codex-rules/`** 中的文件**按需加载** → 节省 context，只在需要时注入细节
- 如果不分离，agent 每次都要加载所有场景的规则，浪费大量 token

### Why `init-codex-rules.sh`?

每个项目从零手写规则不现实。脚本提供两种模式：

- **符号链接模式（默认）**：所有项目共享同一份规则仓库，统一更新
- **独立模式（`--standalone`）**：复制规则到项目中独立维护，允许定制

---

## How to Use in a New Project

### Quick Start (Recommended)

```bash
# Step 1: 复制脚本到新项目
cp /path/to/chat2responses/init-codex-rules.sh /path/to/new-project/
cd /path/to/new-project

# Step 2: 运行初始化
# 符号链接模式（需要先设置 CODEX_RULES_REPO）
CODEX_RULES_REPO=/path/to/global-rules-repo bash init-codex-rules.sh

# 或者独立模式（直接复制到项目）
bash init-codex-rules.sh --standalone
```

### Step-by-Step Manual Setup

#### 1. 创建 `AGENTS.md`

复制四条行为准则和规则索引表到项目根目录。确保包含 "按需加载规则索引" 表格，指引 agent 找到 `.codex-rules/` 中的文件。

#### 2. 创建 `.codex-rules/` 目录

从 `chat2responses` 复制整个目录，然后按需裁剪：

```bash
# 复制全部规则
cp -r /path/to/chat2responses/.codex-rules /path/to/new-project/

# 删除不需要的规则文件
rm /path/to/new-project/.codex-rules/python.md   # 如果项目不是 Python
rm /path/to/new-project/.codex-rules/go.md       # 如果项目不是 Go
```

#### 3. 编辑 `.codex-local.md`

填写项目元数据：

```markdown
**语言**: Rust
**构建**: cargo build
**测试**: cargo test
**入口**: src/main.rs
```

然后添加项目特有的同步约束，例如：

```markdown
### 1. 接口定义同步

改 `proto/*.proto` 必须同步生成对应的 Rust 代码

### 2. 自查清单

1. `cargo fmt` — 格式化
2. `cargo clippy` — 静态检查
3. `cargo test` — 测试
4. 确认 proto 生成与手写代码一致
```

#### 4. 创建 `~/.codex/AGENTS.md`（可选）

如果还没有全局规约，可以从 `chat2responses` 项目的 `~/.codex/AGENTS.md` 模板创建：

```bash
cp ~/.codex/AGENTS.md ~/.codex/AGENTS.md.bak  # 如果已有则备份
# 编辑 ~/.codex/AGENTS.md 填入安全默认值和初始化指引
```

#### 5. 验证

在一个 Codex 会话中测试：

- 启动后 agent 是否自动加载了 `AGENTS.md` 中的准则
- 写 Go 代码时是否加载了 `go.md` 中的编码规范
- 修复 Bug 时是否触发 `bug-fix.md` 中的先红后绿流程

---

## Troubleshooting

| Symptom | Likely Cause | Fix |
|---------|-------------|-----|
| Agent 不知道 `.codex-rules/` 存在 | `AGENTS.md` 缺少规则索引表 | 检查 AGENTS.md 是否包含 "按需加载规则索引" 表格 |
| Agent 不遵守项目特有规则 | `.codex-local.md` 路径不对 | 确认文件在项目根目录，且包含 `## 代码修改规则` 标题 |
| `init-codex-rules.sh` 报 "规则仓库未找到" | `CODEX_RULES_REPO` 未设置且目录不存在 | 创建规则仓库或用 `--standalone` 模式 |
| Agent 在长对话中丢失上下文 | 未触发记忆管理 | 超过 20 轮时提醒 agent 使用 `memory-manager` skill 压缩 |

---

> 本文件托管在 chat2responses 仓库 (https://github.com/fooyii/chat2responses)
> 作为 Codex rules 架构的参考实现和文档。
