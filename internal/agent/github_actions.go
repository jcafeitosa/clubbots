package agent

// GitHubActionConfig generates CI configuration for running GoClaw in GitHub Actions.
// Inspired by Claude Code's GitHub Action integration pattern.
type GitHubActionConfig struct {
	TriggerPhrase string   `json:"trigger_phrase"` // "@goclaw" default
	BranchPrefix  string   `json:"branch_prefix"`  // "goclaw/" default
	AllowedUsers  []string `json:"allowed_users"`  // who can trigger
	MaxTokens     int      `json:"max_tokens"`     // per-run limit
}

// DefaultGitHubActionConfig returns sensible defaults.
func DefaultGitHubActionConfig() GitHubActionConfig {
	return GitHubActionConfig{
		TriggerPhrase: "@goclaw",
		BranchPrefix:  "goclaw/",
		MaxTokens:     100000,
	}
}

// GenerateWorkflowYAML produces a GitHub Actions workflow file.
func (c *GitHubActionConfig) GenerateWorkflowYAML() string {
	return `name: GoClaw

on:
  issue_comment:
    types: [created]
  pull_request:
    types: [opened, synchronize]
  issues:
    types: [opened]

jobs:
  goclaw:
    runs-on: ubuntu-latest
    if: |
      contains(github.event.comment.body, '` + c.TriggerPhrase + `') ||
      github.event_name == 'pull_request' ||
      github.event_name == 'issues'
    steps:
      - uses: actions/checkout@v4

      - name: Setup Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.26'

      - name: Build GoClaw
        run: go build -o goclaw .

      - name: Run GoClaw
        env:
          ANTHROPIC_API_KEY: ${{ secrets.ANTHROPIC_API_KEY }}
          GOCLAW_GATEWAY_TOKEN: ${{ secrets.GOCLAW_GATEWAY_TOKEN }}
        run: |
          EVENT="${{ github.event_name }}"
          if [ "$EVENT" = "pull_request" ]; then
            ./goclaw -p "Review PR #${{ github.event.pull_request.number }}: ${{ github.event.pull_request.title }}. Focus on bugs, security, and code quality."
          elif [ "$EVENT" = "issues" ]; then
            ./goclaw -p "Triage issue #${{ github.event.issue.number }}: ${{ github.event.issue.title }}. Suggest fix approach."
          else
            COMMENT="${{ github.event.comment.body }}"
            PROMPT=$(echo "$COMMENT" | sed 's/` + c.TriggerPhrase + ` //')
            ./goclaw -p "$PROMPT"
          fi

      - name: Post Result
        if: always()
        uses: actions/github-script@v7
        with:
          script: |
            const fs = require('fs');
            const result = fs.readFileSync('/tmp/goclaw-result.md', 'utf8');
            github.rest.issues.createComment({
              issue_number: context.issue.number,
              owner: context.repo.owner,
              repo: context.repo.repo,
              body: result
            });
`
}
