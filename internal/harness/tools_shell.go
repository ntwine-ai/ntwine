package harness

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

var dangerousPatterns = []*regexp.Regexp{
	regexp.MustCompile(`rm\s+(-rf|-fr)\s+/\s*$`),
	regexp.MustCompile(`rm\s+(-rf|-fr)\s+~/?\s*$`),
	regexp.MustCompile(`rm\s+(-rf|-fr)\s+\.\s*$`),
	regexp.MustCompile(`mkfs\.`),
	regexp.MustCompile(`dd\s+.*of=/dev/`),
	regexp.MustCompile(`>\s*/dev/sd`),
	regexp.MustCompile(`:(){.*};:`),
	regexp.MustCompile(`chmod\s+-R\s+777\s+/`),
	regexp.MustCompile(`curl.*\|\s*(sudo\s+)?bash`),
	regexp.MustCompile(`wget.*\|\s*(sudo\s+)?bash`),
}

func RegisterShellTool(r *Registry, codebasePath string, timeout time.Duration) {
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	r.Register(ToolDef{
		Name:             "shell",
		Description:      "run a shell command in the project directory. use for: running tests, installing deps, git operations, build commands. do NOT use for reading or editing files.",
		ModelDescription: "run a shell command. returns stdout+stderr. 30s timeout.",
		Source:           "builtin",
		Destructive:      true,
		Parameters: params(
			prop("command", "string", "the shell command to run"),
			required("command"),
		),
	}, shellHandler(codebasePath, timeout))
}

func shellHandler(codebasePath string, timeout time.Duration) ToolHandlerFunc {
	return func(ctx context.Context, raw json.RawMessage) (ToolResult, error) {
		var args struct {
			Command string `json:"command"`
		}
		if err := json.Unmarshal(raw, &args); err != nil {
			return ToolResult{}, err
		}

		if args.Command == "" {
			return ToolResult{}, fmt.Errorf("command is required")
		}

		for _, pat := range dangerousPatterns {
			if pat.MatchString(args.Command) {
				return ToolResult{
					Output:  fmt.Sprintf("blocked: command matches dangerous pattern (%s)", pat.String()),
					IsError: true,
				}, nil
			}
		}

		cmdCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()

		cmd := exec.CommandContext(cmdCtx, "sh", "-c", args.Command)
		cmd.Dir = codebasePath

		cmd.Env = sanitizeEnv()

		out, err := cmd.CombinedOutput()
		result := string(out)

		if len(result) > 10000 {
			head := result[:4000]
			tail := result[len(result)-4000:]
			result = head + "\n\n... (truncated " + fmt.Sprintf("%d", len(result)-8000) + " chars) ...\n\n" + tail
		}

		if err != nil {
			if cmdCtx.Err() == context.DeadlineExceeded {
				return ToolResult{
					Output:  "command timed out after " + timeout.String() + "\n" + result,
					IsError: true,
				}, nil
			}
			return ToolResult{
				Output:  result + "\nexit: " + err.Error(),
				IsError: true,
			}, nil
		}

		return ToolResult{Output: result}, nil
	}
}

func sanitizeEnv() []string {
	allowed := map[string]bool{
		"PATH": true, "HOME": true, "USER": true, "SHELL": true,
		"TMPDIR": true, "TEMP": true, "TMP": true,
		"LANG": true, "LC_ALL": true, "LC_CTYPE": true,
		"GOPATH": true, "GOROOT": true, "GOBIN": true,
		"NODE_PATH": true, "NVM_DIR": true,
		"RUST_BACKTRACE": true, "CARGO_HOME": true,
		"PYTHON_PATH": true, "VIRTUAL_ENV": true,
		"TERM": true, "COLORTERM": true,
		"XDG_DATA_HOME": true, "XDG_CONFIG_HOME": true, "XDG_CACHE_HOME": true,
		"EDITOR": true, "VISUAL": true,
	}

	var env []string
	for _, e := range os.Environ() {
		parts := strings.SplitN(e, "=", 2)
		if len(parts) == 2 && allowed[parts[0]] {
			env = append(env, e)
		}
	}
	return env
}
