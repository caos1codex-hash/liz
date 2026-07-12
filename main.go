// Liz v3.1 — Single-file launcher with custom proxy.
// Uses the REAL Claude Code skeleton with ALL its tools.
// Routes requests to GLM-5.2 via NVIDIA NIM through a custom embedded proxy.
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

const Version = "3.1.0"

// Custom proxy script — handles system role, tool calls, streaming.
// Uses litellm for proper Anthropic↔OpenAI translation.
const proxyScript = `#!/usr/bin/env python3
"""Liz Proxy v3.1 — Custom Anthropic-to-OpenAI translation layer."""
import os, sys, socket, json, uuid
from typing import Any, Optional, List
from fastapi import FastAPI, HTTPException
from fastapi.responses import StreamingResponse, JSONResponse
from pydantic import BaseModel
import litellm
import uvicorn

litellm.set_verbose = False
litellm.drop_params = True  # silently drop unsupported params

app = FastAPI()

NIM_KEY = os.environ.get("OPENAI_API_KEY", "")
NIM_BASE = os.environ.get("OPENAI_API_BASE", "")
NIM_MODEL = os.environ.get("BIG_MODEL", "z-ai/glm-5.2")

class Message(BaseModel):
    role: str = "user"
    content: Any = None
    tool_calls: Optional[List[Any]] = None
    tool_call_id: Optional[str] = None
    name: Optional[str] = None
    class Config:
        extra = "allow"

class MessagesRequest(BaseModel):
    model: str
    messages: List[Message]
    max_tokens: int = 4096
    system: Optional[Any] = None
    stream: bool = False
    temperature: Optional[float] = 1.0
    top_p: Optional[float] = None
    tools: Optional[List[Any]] = None
    tool_choice: Optional[Any] = None
    stop_sequences: Optional[List[str]] = None
    metadata: Optional[Any] = None
    thinking: Optional[Any] = None
    class Config:
        extra = "allow"

@app.get("/")
async def root():
    return {"message": "Liz Proxy v3.1 (litellm-based)"}

@app.post("/v1/messages")
async def messages(req: MessagesRequest):
    litellm_messages = []

    # Add top-level system message
    if req.system:
        if isinstance(req.system, str):
            litellm_messages.append({"role": "system", "content": req.system})
        elif isinstance(req.system, list):
            parts = []
            for s in req.system:
                if isinstance(s, dict) and s.get("type") == "text":
                    parts.append(s.get("text", ""))
            if parts:
                litellm_messages.append({"role": "system", "content": "\n".join(parts)})

    # Convert each message
    for msg in req.messages:
        m = {"role": msg.role}

        if msg.content is None:
            m["content"] = ""
        elif isinstance(msg.content, str):
            m["content"] = msg.content
        elif isinstance(msg.content, list):
            # Anthropic content blocks
            if msg.role == "user":
                texts = []
                for block in msg.content:
                    if not isinstance(block, dict):
                        continue
                    t = block.get("type")
                    if t == "text":
                        texts.append(block.get("text", ""))
                    elif t == "tool_result":
                        c = block.get("content", "")
                        if isinstance(c, list):
                            for b in c:
                                if isinstance(b, dict) and b.get("type") == "text":
                                    texts.append(b.get("text", ""))
                        else:
                            texts.append(str(c))
                m["content"] = "\n".join(texts) if texts else ""
            elif msg.role == "assistant":
                texts = []
                tool_calls = []
                for block in msg.content:
                    if not isinstance(block, dict):
                        continue
                    t = block.get("type")
                    if t == "text":
                        texts.append(block.get("text", ""))
                    elif t == "tool_use":
                        tool_calls.append({
                            "id": block.get("id", ""),
                            "type": "function",
                            "function": {
                                "name": block.get("name", ""),
                                "arguments": json.dumps(block.get("input", {})),
                            },
                        })
                m["content"] = "\n".join(texts) if texts else ""
                if tool_calls:
                    m["tool_calls"] = tool_calls
        else:
            m["content"] = str(msg.content)

        if msg.role == "tool" and msg.tool_call_id:
            m["tool_call_id"] = msg.tool_call_id

        litellm_messages.append(m)

    kwargs = {
        "model": f"openai/{NIM_MODEL}",
        "messages": litellm_messages,
        "api_base": NIM_BASE,
        "api_key": NIM_KEY,
        "max_tokens": req.max_tokens,
    }
    if req.temperature is not None:
        kwargs["temperature"] = req.temperature
    if req.tools:
        kwargs["tools"] = req.tools
    if req.tool_choice:
        kwargs["tool_choice"] = req.tool_choice
    if req.stop_sequences:
        kwargs["stop"] = req.stop_sequences

    try:
        if req.stream:
            kwargs["stream"] = True
            response = await litellm.acompletion(**kwargs)
            return StreamingResponse(
                stream_anthropic(response),
                media_type="text/event-stream",
            )
        else:
            response = await litellm.acompletion(**kwargs)
            return JSONResponse(convert_to_anthropic(response))
    except Exception as e:
        import traceback
        tb = traceback.format_exc()
        raise HTTPException(status_code=500, detail=f"{e}\n{tb}")


async def stream_anthropic(response):
    msg_id = f"msg_{uuid.uuid4().hex[:24]}"
    yield f"event: message_start\ndata: {json.dumps({'type':'message_start','message':{'id':msg_id,'type':'message','role':'assistant','model':NIM_MODEL,'content':[],'stop_reason':None,'stop_sequence':None,'usage':{'input_tokens':0,'output_tokens':0}}})}\n\n"
    yield f"event: content_block_start\ndata: {json.dumps({'type':'content_block_start','index':0,'content_block':{'type':'text','text':''}})}\n\n"

    full_text = ""
    finish_reason = "end_turn"
    tool_call_blocks = {}  # index -> {id, name, args}

    async for chunk in response:
        if not chunk.choices:
            continue
        delta = chunk.choices[0].delta
        if delta:
            if getattr(delta, "content", None):
                full_text += delta.content
                yield f"event: content_block_delta\ndata: {json.dumps({'type':'content_block_delta','index':0,'delta':{'type':'text_delta','text':delta.content}})}\n\n"
            if getattr(delta, "tool_calls", None):
                for tc in delta.tool_calls:
                    idx = tc.index
                    if idx not in tool_call_blocks:
                        tool_call_blocks[idx] = {"id": "", "name": "", "args": ""}
                    if tc.id:
                        tool_call_blocks[idx]["id"] = tc.id
                    if tc.function:
                        if tc.function.name:
                            tool_call_blocks[idx]["name"] = tc.function.name
                        if tc.function.arguments:
                            tool_call_blocks[idx]["args"] += tc.function.arguments
        if chunk.choices[0].finish_reason:
            fr = chunk.choices[0].finish_reason
            if fr == "stop":
                finish_reason = "end_turn"
            elif fr == "tool_calls":
                finish_reason = "tool_use"
            elif fr == "length":
                finish_reason = "max_tokens"

    yield f"event: content_block_stop\ndata: {json.dumps({'type':'content_block_stop','index':0})}\n\n"

    # Send tool_use blocks if any
    for idx in sorted(tool_call_blocks.keys()):
        tc = tool_call_blocks[idx]
        try:
            args = json.loads(tc["args"]) if tc["args"] else {}
        except:
            args = {"raw": tc["args"]}
        block = {"type": "tool_use", "id": tc["id"], "name": tc["name"], "input": args}
        yield f"event: content_block_start\ndata: {json.dumps({'type':'content_block_start','index':idx+1,'content_block':block})}\n\n"
        yield f"event: content_block_stop\ndata: {json.dumps({'type':'content_block_stop','index':idx+1})}\n\n"

    yield f"event: message_delta\ndata: {json.dumps({'type':'message_delta','delta':{'stop_reason':finish_reason,'stop_sequence':None},'usage':{'output_tokens':len(full_text)}})}\n\n"
    yield f"event: message_stop\ndata: {json.dumps({'type':'message_stop'})}\n\n"
    yield "data: [DONE]\n\n"


def convert_to_anthropic(response):
    choice = response.choices[0]
    msg = choice.message

    content = []
    if getattr(msg, "content", None):
        content.append({"type": "text", "text": msg.content})

    if getattr(msg, "tool_calls", None):
        for tc in msg.tool_calls:
            try:
                args = json.loads(tc.function.arguments) if tc.function.arguments else {}
            except:
                args = {"raw": tc.function.arguments}
            content.append({
                "type": "tool_use",
                "id": tc.id,
                "name": tc.function.name,
                "input": args,
            })

    stop_reason = "end_turn"
    if choice.finish_reason == "tool_calls":
        stop_reason = "tool_use"
    elif choice.finish_reason == "length":
        stop_reason = "max_tokens"

    usage = response.usage
    return {
        "id": f"msg_{uuid.uuid4().hex[:24]}",
        "type": "message",
        "role": "assistant",
        "model": NIM_MODEL,
        "content": content,
        "stop_reason": stop_reason,
        "stop_sequence": None,
        "usage": {
            "input_tokens": usage.prompt_tokens if usage else 0,
            "output_tokens": usage.completion_tokens if usage else 0,
        },
    }


port = 8082
while True:
    with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as s:
        if s.connect_ex(("127.0.0.1", port)) != 0:
            break
    port += 1
with open(os.path.join(os.environ.get("TEMP", "/tmp"), "liz_port.txt"), "w") as f:
    f.write(str(port))

if __name__ == "__main__":
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
//  LAUNCHER
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

	// 2. Ensure litellm is installed (we no longer need claude-code-proxy)
	fmt.Printf("%s  [2/4] Proxy deps...%s    ", cGray, cReset)
	if err := ensureDeps(pyCmd); err != nil {
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

	port, err := waitForProxy(30 * time.Second)
	if err != nil {
		return fmt.Errorf("proxy not ready: %w", err)
	}
	fmt.Printf("%sOK%s (port %d)\n\n", cGreen, cReset, port)

	// Build clean env for Claude Code — ONLY set AUTH_TOKEN, never API_KEY
	cleanEnv := []string{}
	for _, e := range os.Environ() {
		// Strip all ANTHROPIC_* vars to avoid conflicts
		if strings.HasPrefix(e, "ANTHROPIC_") {
			continue
		}
		cleanEnv = append(cleanEnv, e)
	}
	// Add only our auth config
	cleanEnv = append(cleanEnv,
		"ANTHROPIC_BASE_URL=http://127.0.0.1:"+fmt.Sprintf("%d", port),
		"ANTHROPIC_AUTH_TOKEN=liz-local",
	)

	cmd := exec.Command(claudePath, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = cleanEnv

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

func ensureDeps(pyCmd string) error {
	// Check if litellm + fastapi + uvicorn are installed
	cmd := exec.Command(pyCmd, "-c", "import litellm, fastapi, uvicorn")
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Run(); err == nil {
		return nil
	}

	fmt.Printf("\n%s  Installing dependencies (litellm, fastapi, uvicorn)...%s\n", cGray, cReset)
	installCmd := exec.Command(pyCmd, "-m", "pip", "install", "--quiet",
		"litellm", "fastapi", "uvicorn", "httpx")
	installCmd.Stdout = os.Stdout
	installCmd.Stderr = os.Stderr
	if err := installCmd.Run(); err != nil {
		return fmt.Errorf("install dependencies: %w", err)
	}
	return nil
}

func ensureClaude() (string, error) {
	if path, err := exec.LookPath("claude"); err == nil {
		return path, nil
	}

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

	for time.Now().Before(deadline) {
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
