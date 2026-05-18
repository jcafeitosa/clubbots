package methods

import (
	"context"
	"encoding/json"
	"errors"
	"os/exec"
	"strings"

	"github.com/nextlevelbuilder/goclaw/internal/gateway"
	"github.com/nextlevelbuilder/goclaw/pkg/protocol"
)

const securityReviewPrompt = `Review this diff for security issues. Check for:
1. Secrets or credentials in code (API keys, tokens, passwords)
2. SQL/command injection vulnerabilities
3. Path traversal risks
4. XSS vulnerabilities
5. Insecure cryptography or hashing
6. Missing input validation
7. Authentication/authorization bypasses

Report each finding with: severity (critical/high/medium/low), file, line context, description, and fix suggestion.`

var errNoGit = errors.New("not a git repository")

type GitMethods struct{}

func NewGitMethods() *GitMethods { return &GitMethods{} }

func (m *GitMethods) Register(router *gateway.MethodRouter) {
	router.Register(protocol.MethodGitCommitMessage, m.handleCommitMessage)
	router.Register(protocol.MethodGitCodeReview, m.handleCodeReview)
	router.Register(protocol.MethodGitSecurityReview, m.handleSecurityReview)
}

type gitParams struct {
	Path string `json:"path,omitempty"`
}

func (m *GitMethods) handleCommitMessage(ctx context.Context, client *gateway.Client, req *protocol.RequestFrame) {
	var params gitParams
	if req.Params != nil {
		json.Unmarshal(req.Params, &params)
	}

	diff, err := runGitCmd(params.Path, "diff", "--staged")
	if err != nil {
		client.SendResponse(protocol.NewOKResponse(req.ID, map[string]any{
			"diff": "", "recentLog": "", "stagedFiles": 0,
			"warning": "not a git repository",
		}))
		return
	}
	log, _ := runGitCmd(params.Path, "log", "--oneline", "-5")
	client.SendResponse(protocol.NewOKResponse(req.ID, map[string]any{
		"diff": diff, "recentLog": log, "stagedFiles": countStagedFiles(diff),
	}))
}

func (m *GitMethods) handleCodeReview(ctx context.Context, client *gateway.Client, req *protocol.RequestFrame) {
	var params gitParams
	if req.Params != nil {
		json.Unmarshal(req.Params, &params)
	}

	diff, err := runGitCmd(params.Path, "diff")
	if err != nil {
		client.SendResponse(protocol.NewOKResponse(req.ID, map[string]any{
			"unstagedDiff": "", "stagedDiff": "",
			"warning": "not a git repository",
		}))
		return
	}
	stagedDiff, _ := runGitCmd(params.Path, "diff", "--staged")
	client.SendResponse(protocol.NewOKResponse(req.ID, map[string]any{
		"unstagedDiff": diff, "stagedDiff": stagedDiff,
	}))
}

func (m *GitMethods) handleSecurityReview(ctx context.Context, client *gateway.Client, req *protocol.RequestFrame) {
	var params gitParams
	if req.Params != nil {
		json.Unmarshal(req.Params, &params)
	}

	diff, err := runGitCmd(params.Path, "diff", "--staged")
	if err != nil {
		client.SendResponse(protocol.NewOKResponse(req.ID, map[string]any{
			"diff": "", "prompt": securityReviewPrompt,
			"warning": "not a git repository",
		}))
		return
	}
	if diff == "" {
		diff, _ = runGitCmd(params.Path, "diff")
	}
	client.SendResponse(protocol.NewOKResponse(req.ID, map[string]any{
		"diff": diff, "prompt": securityReviewPrompt,
	}))
}

func runGitCmd(dir string, args ...string) (string, error) {
	if _, err := exec.LookPath("git"); err != nil {
		return "", errNoGit
	}
	check := exec.Command("git", "rev-parse", "--git-dir")
	if dir != "" {
		check.Dir = dir
	}
	if err := check.Run(); err != nil {
		return "", errNoGit
	}
	cmd := exec.Command("git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func countStagedFiles(diff string) int {
	if diff == "" {
		return 0
	}
	count := 0
	for _, line := range strings.Split(diff, "\n") {
		if strings.HasPrefix(line, "diff --git") {
			count++
		}
	}
	return count
}
