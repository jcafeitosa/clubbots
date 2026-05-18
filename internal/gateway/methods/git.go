package methods

import (
	"context"
	"encoding/json"
	"os/exec"
	"strings"

	"github.com/nextlevelbuilder/goclaw/internal/gateway"
	"github.com/nextlevelbuilder/goclaw/internal/i18n"
	"github.com/nextlevelbuilder/goclaw/internal/store"
	"github.com/nextlevelbuilder/goclaw/pkg/protocol"
)

// GitMethods handles git.commit_message and git.code_review.
type GitMethods struct{}

func NewGitMethods() *GitMethods { return &GitMethods{} }

func (m *GitMethods) Register(router *gateway.MethodRouter) {
	router.Register(protocol.MethodGitCommitMessage, m.handleCommitMessage)
	router.Register(protocol.MethodGitCodeReview, m.handleCodeReview)
	router.Register(protocol.MethodGitSecurityReview, m.handleSecurityReview)
}

type gitParams struct {
	Path string `json:"path,omitempty"` // repo path (default: cwd)
}

func (m *GitMethods) handleCommitMessage(ctx context.Context, client *gateway.Client, req *protocol.RequestFrame) {
	locale := store.LocaleFromContext(ctx)
	var params gitParams
	if req.Params != nil {
		json.Unmarshal(req.Params, &params)
	}

	diff, err := runGitCmd(params.Path, "diff", "--staged")
	if err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInternal, i18n.T(locale, i18n.MsgInvalidRequest, "git diff failed: "+err.Error())))
		return
	}
	log, _ := runGitCmd(params.Path, "log", "--oneline", "-5")

	client.SendResponse(protocol.NewOKResponse(req.ID, map[string]any{
		"diff":        diff,
		"recentLog":   log,
		"stagedFiles": countStagedFiles(diff),
	}))
}

func (m *GitMethods) handleCodeReview(ctx context.Context, client *gateway.Client, req *protocol.RequestFrame) {
	locale := store.LocaleFromContext(ctx)
	var params gitParams
	if req.Params != nil {
		json.Unmarshal(req.Params, &params)
	}

	diff, err := runGitCmd(params.Path, "diff")
	if err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInternal, i18n.T(locale, i18n.MsgInvalidRequest, "git diff failed: "+err.Error())))
		return
	}
	stagedDiff, _ := runGitCmd(params.Path, "diff", "--staged")

	client.SendResponse(protocol.NewOKResponse(req.ID, map[string]any{
		"unstagedDiff": diff,
		"stagedDiff":   stagedDiff,
	}))
}

func runGitCmd(dir string, args ...string) (string, error) {
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

func (m *GitMethods) handleSecurityReview(ctx context.Context, client *gateway.Client, req *protocol.RequestFrame) {
	locale := store.LocaleFromContext(ctx)
	var params gitParams
	if req.Params != nil {
		json.Unmarshal(req.Params, &params)
	}

	diff, err := runGitCmd(params.Path, "diff", "--staged")
	if err != nil {
		client.SendResponse(protocol.NewErrorResponse(req.ID, protocol.ErrInternal, i18n.T(locale, i18n.MsgInvalidRequest, "git diff failed: "+err.Error())))
		return
	}
	if diff == "" {
		diff, _ = runGitCmd(params.Path, "diff") // fallback to unstaged
	}

	client.SendResponse(protocol.NewOKResponse(req.ID, map[string]any{
		"diff": diff,
		"prompt": `Review this diff for security issues. Check for:
1. Secrets or credentials in code (API keys, tokens, passwords)
2. SQL/command injection vulnerabilities
3. Path traversal risks
4. XSS vulnerabilities
5. Insecure cryptography or hashing
6. Missing input validation
7. Authentication/authorization bypasses

Report each finding with: severity (critical/high/medium/low), file, line context, description, and fix suggestion.`,
	}))
}
