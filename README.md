# 🌸 Liz v3.0

Liz is an AI coding assistant that uses the **real Claude Code skeleton** with ALL its tools, powered by **GLM-5.2** via **NVIDIA NIM**.

## What's new in v3.0

- **Uses the actual Claude Code binary** — not a reimplementation
- **ALL Claude Code tools available**: multi-file editing, Git, MCP, slash commands, search, bash, etc.
- **Single .exe file** — no multiple Go files
- **Built-in proxy** — Anthropic→OpenAI translation happens automatically
- **Auto-installs dependencies** — Python proxy and Claude Code are installed on first run if missing

## 📦 Install

### Option 1 — PowerShell installer (recommended)

```powershell
powershell -ExecutionPolicy Bypass -Command "irm https://raw.githubusercontent.com/caos1codex-hash/liz/main/install.ps1 | iex"
```

### Option 2 — Manual

1. Download `hola.exe` from the [latest release](https://github.com/caos1codex-hash/liz/releases/latest)
2. Place it in a folder (e.g., `C:\Liz\`)
3. Add that folder to your PATH
4. Open a terminal and type `hola liz`

## 🚀 Usage

```cmd
hola liz              :: Start interactive session (full Claude Code)
hola liz -p "prompt"  :: Non-interactive prompt
hola liz --version    :: Show version
hola liz --help       :: Show help
```

## 🛠️ All Claude Code tools included

| Tool | Description |
|---|---|
| **Read** | Read file contents |
| **Write** | Write/create files |
| **Edit** | Edit files with diff preview |
| **MultiEdit** | Edit multiple files at once |
| **Bash** | Execute shell commands |
| **Glob** | Find files by pattern |
| **Grep** | Search file contents |
| **LS** | List directories |
| **WebSearch** | Search the web |
| **WebFetch** | Fetch web pages |
| **Task** | Launch sub-agents |
| **TodoWrite** | Manage task lists |
| **NotebookEdit** | Edit Jupyter notebooks |
| **BashOutput** | Read bash command output |
| **KillBash** | Kill running bash commands |
| **Slash commands** | /help, /clear, /tools, /cost, /compact, /review, etc. |
| **MCP** | Model Context Protocol servers |
| **Git** | Status, diff, commit, PR workflows |
| **Project awareness** | .claude/ directory, CLAUDE.md, settings |

## 🔧 Requirements

- **Windows 10/11** x64
- **Python 3.10+** (auto-installed if missing) → https://www.python.org/downloads/
- **Node.js 18+** (for Claude Code) → https://nodejs.org/

Liz will auto-install `claude-code-proxy` (pip) and `@anthropic-ai/claude-code` (npm) on first run if they're missing.

## 🎨 Design

| Aspect | Value |
|---|---|
| Skeleton | Claude Code v2.1.207 (real binary) |
| Model | z-ai/glm-5.2 |
| API | NVIDIA NIM |
| Proxy | claude-code-proxy (Python, auto-started) |
| Launcher | Go binary (5.1 MB) |
| Colors | Pink/purple (ANSI 256-color) |
| Command | `hola liz` |

## ⚠️ Security

The NVIDIA API key is embedded in the launcher. Anyone with `hola.exe` can extract it. Rotate the key at https://build.nvidia.com → API Keys if needed.

## 📜 License

MIT
