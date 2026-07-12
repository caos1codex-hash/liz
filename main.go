// Liz v3.0 — Single-file launcher.
// Uses the REAL Claude Code skeleton with ALL its tools.
// Routes requests to GLM-5.2 via NVIDIA NIM through an embedded proxy.
package main

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// ============================================================
//  CONFIG
// ============================================================

const Version = "3.0.0"

// Embedded Python proxy script — translates Anthropic Messages API
// to OpenAI Chat Completions and forwards to NVIDIA NIM (GLM-5.2).
// Uses the proven claude-code-proxy library under the hood.
const proxyScript = `#!/usr/bin/env python3
import os, sys, socket
os.environ["OPENAI_API_KEY"]      = "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"
os.environ["OPENAI_API_BASE"]     = "https://integrate.api.nvidia.com/v1"
os.environ["BIG_MODEL"]           = "z-ai/glm-5.2"
os.environ["SMALL_MODEL"]         = "z-ai/glm-5.2"
os.environ["PREFERRED_PROVIDER"]  = "openai"
try:
    from server.fastapi import app
except ImportError:
    print("ERROR: claude-code-proxy not installed. Run: pip install claude-code-proxy", file=sys.stderr)
    sys.exit(1)
import uvicorn
port = 8082
while True:
    with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as s:
        if s.connect_ex(("127.0.0.1", port)) != 0:
            break
    port += 1
with open(os.path.join(os.environ.get("TEMP", "/tmp"), "liz_port.txt"), "w") as f:
    f.write(str(port))
uvicorn.run(app, host="127.0.0.1", port=port, log_level="error")
`

// ============================================================
//  COLORS (pink/purple palette)
// ============================================================

const (
	cReset    = "\033[0m"
	cFuchsia  = "\033[38;5;213m"
	cPurple   = "\033[38;5;141m"
	cLavender = "\033[38;5;147m"
	cGray     = "\033[38;5;245m"
	cRed      = "\033[38;5;203m"
	cGreen    = "\033[38;5;150m"
)

// ============================================================
//  MAIN
// ============================================================

func main() {
	args := os.Args[1:]

	if len(args) == 0 {
		printHelp()
		os.Exit(0)
	}

	switch args[0] {
	case "--help", "-h":
		printHelp()
		return
	case "--version", "-v":
		fmt.Printf("Liz v%s\n", Version)
		return
	}

	if strings.ToLower(args[0]) != "liz" {
		fmt.Printf("\n  %sDid you mean: hola liz%s\n", cFuchsia, cReset)
		fmt.Printf("\n  Liz doesn't recognize '%s'.\n", args[0])
		fmt.Printf("  To start Liz, type:\n\n")
		fmt.Printf("    %shola liz%s\n\n", cPurple, cReset)
		os.Exit(1)
	}

	if err := runLiz(args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "%serror:%s %v\n", cRed, cReset, err)
		os.Exit(1)
	}
}

// ============================================================
//  LAUNCHER — orchestrates proxy + Claude Code
// ============================================================

func runLiz(args []string) error {
	printBanner()

	// 1. Find Python
	fmt.Printf("%s  [1/4] Python...%s       ", cGray, cReset)
	pyCmd, err := findPython()
	if err != nil {
		fmt.Printf("%sFAIL%s\n", cRed, cReset)
		return err
	}
	fmt.Printf("%sOK%s (%s)\n", cGreen, cReset, pyCmd)

	// 2. Ensure claude-code-proxy is installed
	fmt.Printf("%s  [2/4] Proxy...%s         ", cGray, cReset)
	if err := ensureProxy(pyCmd); err != nil {
		fmt.Printf("%sFAIL%s\n", cRed, cReset)
		return err
	}
	fmt.Printf("%sOK%s\n", cGreen, cReset)

	// 3. Ensure Claude Code is installed
	fmt.Printf("%s  [3/4] Claude Code...%s   ", cGray, cReset)
	claudePath, err := ensureClaude()
	if err != nil {
		fmt.Printf("%sFAIL%s\n", cRed, cReset)
		return err
	}
	fmt.Printf("%sOK%s\n", cGreen, cReset)

	// 4. Start proxy + launch Claude Code
	fmt.Printf("%s  [4/4] Starting...%s      ", cGray, cReset)

	// Write proxy script to temp
	tmpDir := os.TempDir()
	proxyPath := filepath.Join(tmpDir, "liz-proxy.py")
	if err := os.WriteFile(proxyPath, []byte(proxyScript), 0644); err != nil {
		return fmt.Errorf("write proxy script: %w", err)
	}
	defer os.Remove(proxyPath)

	// Start proxy in background
	proxyCmd := exec.Command(pyCmd, proxyPath)
	proxyCmd.Stdout = nil
	proxyCmd.Stderr = nil
	if err := proxyCmd.Start(); err != nil {
		return fmt.Errorf("start proxy: %w", err)
	}
	defer func() {
		if proxyCmd.Process != nil {
			proxyCmd.Process.Kill()
			proxyCmd.Wait()
		}
	}()

	// Wait for proxy to be ready
	port, err := waitForProxy(20 * time.Second)
	if err != nil {
		return fmt.Errorf("proxy not ready: %w", err)
	}
	fmt.Printf("%sOK%s (port %d)\n\n", cGreen, cReset, port)

	// Launch Claude Code with proxy as the API endpoint
	cmd := exec.Command(claudePath, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(),
		"ANTHROPIC_BASE_URL=http://127.0.0.1:"+fmt.Sprintf("%d", port),
		"ANTHROPIC_AUTH_TOKEN=liz-local",
	)

	return cmd.Run()
}

// ============================================================
//  DEPENDENCY CHECKS
// ============================================================

func findPython() (string, error) {
	for _, cmd := range []string{"python", "python3", "py"} {
		if path, err := exec.LookPath(cmd); err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("Python not found.\n    Install from https://www.python.org/downloads/\n    Mark 'Add Python to PATH' during installation")
}

func ensureProxy(pyCmd string) error {
	// Quick check: can we import server.fastapi?
	cmd := exec.Command(pyCmd, "-c", "import server.fastapi")
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Run(); err == nil {
		return nil // already installed
	}

	// Install it
	fmt.Printf("\n%s  Installing claude-code-proxy...%s\n", cGray, cReset)
	installCmd := exec.Command(pyCmd, "-m", "pip", "install", "claude-code-proxy")
	installCmd.Stdout = os.Stdout
	installCmd.Stderr = os.Stderr
	if err := installCmd.Run(); err != nil {
		return fmt.Errorf("install claude-code-proxy: %w", err)
	}
	return nil
}

func ensureClaude() (string, error) {
	if path, err := exec.LookPath("claude"); err == nil {
		return path, nil
	}

	// Check if npm is available
	if _, err := exec.LookPath("npm"); err != nil {
		return "", fmt.Errorf("Claude Code not found and npm not available.\n    Install Node.js from https://nodejs.org/")
	}

	fmt.Printf("\n%s  Installing Claude Code via npm...%s\n", cGray, cReset)
	cmd := exec.Command("npm", "install", "-g", "@anthropic-ai/claude-code")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("install Claude Code: %w", err)
	}

	path, err := exec.LookPath("claude")
	if err != nil {
		return "", fmt.Errorf("Claude Code still not found after installation. Check npm global path.")
	}
	return path, nil
}

// ============================================================
//  PROXY READINESS CHECK
// ============================================================

func waitForProxy(timeout time.Duration) (int, error) {
	deadline := time.Now().Add(timeout)
	portFile := filepath.Join(os.TempDir(), "liz_port.txt")

	// Try default port first, then read from port file
	for time.Now().Before(deadline) {
		// Try to read the port file
		if data, err := os.ReadFile(portFile); err == nil {
			port := strings.TrimSpace(string(data))
			if port != "" {
				url := "http://127.0.0.1:" + port + "/"
				resp, err := http.Get(url)
				if err == nil && resp.StatusCode == 200 {
					resp.Body.Close()
					os.Remove(portFile)
					return atoi(port), nil
				}
			}
		}
		// Also try default port directly
		resp, err := http.Get("http://127.0.0.1:8082/")
		if err == nil && resp.StatusCode == 200 {
			resp.Body.Close()
			return 8082, nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	return 0, fmt.Errorf("timeout waiting for proxy")
}

func atoi(s string) int {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			break
		}
		n = n*10 + int(c-'0')
	}
	return n
}

// ============================================================
//  UI
// ============================================================

func printBanner() {
	fmt.Printf("\n%s", cFuchsia)
	fmt.Printf("        ██╗     ██╗ ██████╗ ██╗   ██╗\n")
	fmt.Printf("        ██║     ██║██╔═══██╗╚██╗ ██╔╝\n")
	fmt.Printf("        ██║     ██║██║   ██║ ╚████╔╝\n")
	fmt.Printf("        ╚██████╔╝╚██████╔╝  ╚██╔╝\n")
	fmt.Printf("         ╚═════╝  ╚═════╝    ╚═╝\n")
	fmt.Printf("%s\n", cReset)
	fmt.Printf("%s  Liz v%s — Powered by GLM-5.2 via NVIDIA NIM%s\n", cLavender, Version, cReset)
	fmt.Printf("%s  Uses Claude Code skeleton — all tools enabled.%s\n\n", cGray, cReset)
}

func printHelp() {
	fmt.Printf(`
  %sLiz — Usage%s

    %shola liz%s              Start interactive session (full Claude Code)
    %shola liz -p "prompt"%s  Non-interactive prompt
    %shola liz --version%s    Show version
    %shola liz --help%s       Show this help

  %sLiz uses the real Claude Code skeleton with ALL its tools:%s
    %s• Multi-file editing with diff visualization%s
    %s• Git integration (status, diff, commit)%s
    %s• MCP (Model Context Protocol) support%s
    %s• Slash commands (/help, /clear, /tools, /cost, etc.)%s
    %s• Project awareness (.claude/ directory)%s
    %s• File operations (Read, Write, Edit, Search)%s
    %s• Shell command execution (Bash)%s
    %s• Web search and fetch%s
    %s• Codebase search and grep%s
    %s• And everything else Claude Code offers%s

  %sRequirements:%s
    %s• Python 3.10+ (for proxy)%s
    %s• Node.js 18+ (for Claude Code)%s

  %sPowered by GLM-5.2 via NVIDIA NIM.%s

`, cPurple, cReset,
		cFuchsia, cReset,
		cFuchsia, cReset,
		cFuchsia, cReset,
		cFuchsia, cReset,
		cGray, cReset,
		cLavender, cReset,
		cLavender, cReset,
		cLavender, cReset,
		cLavender, cReset,
		cLavender, cReset,
		cLavender, cReset,
		cLavender, cReset,
		cLavender, cReset,
		cLavender, cReset,
		cLavender, cReset,
		cGray, cReset,
		cLavender, cReset,
		cLavender, cReset,
		cFuchsia, cReset)
}
