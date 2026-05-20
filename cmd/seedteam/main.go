//go:build sqlite
// +build sqlite

package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/nextlevelbuilder/goclaw/internal/providers"
	"github.com/nextlevelbuilder/goclaw/internal/store"
	"github.com/nextlevelbuilder/goclaw/internal/store/sqlitestore"
)

type linkDef struct{ from, to string }

func main() {
	dataDir := os.Getenv("GOCLAW_DATA_DIR")
	if dataDir == "" {
		home, _ := os.UserHomeDir()
		dataDir = filepath.Join(home, ".goclaw", "data")
	}
	cfg := store.StoreConfig{SQLitePath: filepath.Join(dataDir, "goclaw.db"), StorageBackend: "sqlite"}
	stores, err := sqlitestore.NewSQLiteStores(cfg)
	if err != nil {
		log.Fatalf("open: %v", err)
	}
	ctx := store.WithTenantID(context.Background(), providers.MasterTenantID)

	createLink := func(from, to string) bool {
		src, err := stores.Agents.GetByKey(ctx, from)
		if err != nil {
			return false
		}
		dst, err := stores.Agents.GetByKey(ctx, to)
		if err != nil {
			return false
		}
		// Check if link already exists
		existing, _ := stores.AgentLinks.GetLinkBetween(ctx, src.ID, dst.ID)
		if existing != nil {
			return false // already exists
		}
		return stores.AgentLinks.CreateLink(ctx, &store.AgentLinkData{
			SourceAgentID: src.ID, TargetAgentID: dst.ID,
			Direction: "outbound",
			Description: fmt.Sprintf("%s → %s", from, to),
			Status: "active", MaxConcurrent: 5,
		}) == nil
	}

	// ═══ CEO → ALL DEPARTMENT LEADS ═══
	fmt.Println("=== CEO → Department Leads ===")
	ceoLeads := []string{
		"cto", "cpo", "ciso", "cdo", "cfo", "coo",
		"vp-eng", "vp-ai", "vp-infra", "vp-product",
		"dir-platform", "dir-product", "dir-ai", "dir-infra",
		"dir-mobile", "dir-security", "dir-data",
		"dir-backend", "dir-frontend", "dir-qa", "dir-devrel",
		"fintech-lead", "saas-lead", "github-lead", "modern-stack-lead",
		"dist-eng",
	}
	n := 0
	for _, lead := range ceoLeads {
		if createLink("ceo", lead) {
			n++
		}
	}
	fmt.Printf("CEO → %d department leads linked\n\n", n)

	// ═══ DEPARTMENT LEADS → TEAM MEMBERS ═══
	fmt.Println("=== Department Leads → Members ===")
	links := []linkDef{
		// C-Level
		{"cto", "vp-eng"}, {"cto", "vp-ai"}, {"cto", "vp-infra"},
		{"cpo", "vp-product"}, {"cpo", "dir-product"}, {"cpo", "sr-pm"}, {"cpo", "jr-pm"}, {"cpo", "ux-designer"}, {"cpo", "tpm"}, {"cpo", "tech-lead"},
		{"ciso", "dir-security"}, {"ciso", "security-lead"}, {"ciso", "security-analyst"}, {"ciso", "sr-security"}, {"ciso", "mid-security"},
		{"cdo", "dir-data"}, {"cdo", "data-sci"}, {"cdo", "sr-data-eng"}, {"cdo", "data-engineer"}, {"cdo", "bi-engineer"}, {"cdo", "data-analyst"},
		// VPs
		{"vp-ai", "dir-ai"}, {"vp-ai", "research-sci"}, {"vp-ai", "ml-engineer"}, {"vp-ai", "nn-engineer"}, {"vp-ai", "jr-ml"},
		{"vp-infra", "dir-infra"}, {"vp-infra", "cloud-arch"}, {"vp-infra", "net-eng"}, {"vp-infra", "db-admin"}, {"vp-infra", "sys-admin"}, {"vp-infra", "sre"}, {"vp-infra", "jr-cloud"},
		{"vp-product", "dir-product"}, {"vp-product", "sr-pm"}, {"vp-product", "jr-pm"},
		// Directors
		{"dir-platform", "sr-backend"}, {"dir-platform", "sr-frontend"}, {"dir-platform", "mid-api"}, {"dir-platform", "mid-fe"},
		{"dir-platform", "jr-backend"}, {"dir-platform", "jr-frontend"}, {"dir-platform", "devops"},
		{"dir-platform", "platform-eng"}, {"dir-platform", "principal-eng"}, {"dir-platform", "solutions-arch"},
		{"dir-platform", "staff-arch"}, {"dir-platform", "infra-director"}, {"dir-platform", "staff-swe"},
		{"dir-product", "ux-designer"}, {"dir-product", "tech-lead"}, {"dir-product", "scrum-master"}, {"dir-product", "solutions-arch"}, {"dir-product", "sr-pm"},
		{"dir-backend", "sr-backend"}, {"dir-backend", "mid-api"}, {"dir-backend", "jr-backend"}, {"dir-backend", "api-architect"}, {"dir-backend", "db-admin"},
		{"dir-frontend", "sr-frontend"}, {"dir-frontend", "mid-fe"}, {"dir-frontend", "jr-frontend"}, {"dir-frontend", "web-perf-eng"}, {"dir-frontend", "a11y-eng"},
		{"dir-mobile", "mobile-eng"}, {"dir-mobile", "sr-mobile"}, {"dir-mobile", "perf-eng"},
		{"dir-qa", "qa-architect"}, {"dir-qa", "reviewer"}, {"dir-qa", "qa-engineer"}, {"dir-qa", "test-automation"},
		{"dir-devrel", "dev-advocate"}, {"dir-devrel", "tech-writer"}, {"dir-devrel", "intern-docs"}, {"dir-devrel", "scrum-master"},
		// Specialized leads
		{"backend-lead", "sr-backend"}, {"backend-lead", "mid-api"}, {"backend-lead", "jr-backend"}, {"backend-lead", "api-architect"}, {"backend-lead", "db-admin"},
		{"frontend-lead", "sr-frontend"}, {"frontend-lead", "mid-fe"}, {"frontend-lead", "jr-frontend"}, {"frontend-lead", "web-perf-eng"}, {"frontend-lead", "a11y-eng"},
		{"fintech-lead", "ccxt-eng"}, {"fintech-lead", "crypto-trader"}, {"fintech-lead", "quant-analyst"},
		{"fintech-lead", "market-analyst"}, {"fintech-lead", "bot-trader"}, {"fintech-lead", "defi-eng"},
		{"fintech-lead", "risk-analyst"}, {"fintech-lead", "backtest-eng"},
		{"fintech-lead", "python-eng"}, {"fintech-lead", "go-eng"}, {"fintech-lead", "rust-eng"},
		{"saas-lead", "saas-architect"}, {"saas-lead", "billing-eng"}, {"saas-lead", "subscription-eng"},
		{"saas-lead", "auth-eng"}, {"saas-lead", "sso-eng"}, {"saas-lead", "tenant-eng"},
		{"github-lead", "actions-eng"}, {"github-lead", "workflow-eng"}, {"github-lead", "github-api-eng"},
		{"github-lead", "github-app-dev"}, {"github-lead", "github-bot-dev"},
		{"github-lead", "codeql-eng"}, {"github-lead", "dependabot-eng"}, {"github-lead", "secret-scan-eng"},
		{"github-lead", "github-enterprise"}, {"github-lead", "copilot-specialist"},
		{"github-lead", "oss-maintainer"}, {"github-lead", "community-mgr"},
		{"github-lead", "github-pages-eng"}, {"github-lead", "codespaces-eng"}, {"github-lead", "gh-cli-eng"},
		{"modern-stack-lead", "astro-eng"}, {"modern-stack-lead", "nextjs-eng"}, {"modern-stack-lead", "svelte-eng"},
		{"modern-stack-lead", "remix-eng"}, {"modern-stack-lead", "nuxt-eng"},
		{"modern-stack-lead", "vite-eng"}, {"modern-stack-lead", "turbo-eng"}, {"modern-stack-lead", "frontend-lead"}, {"modern-stack-lead", "sr-frontend"},
		{"security-lead", "security-analyst"}, {"security-lead", "sr-security"}, {"security-lead", "mid-security"}, {"security-lead", "infra-director"},
		{"sre", "mid-sre"}, {"sre", "sr-sre"}, {"sre", "devops"}, {"sre", "sys-admin"}, {"sre", "net-eng"}, {"sre", "cloud-arch"},
		{"qa-architect", "sdet"}, {"qa-architect", "test-automation"}, {"qa-architect", "reviewer"}, {"qa-architect", "qa-engineer"}, {"qa-architect", "qa-analyst"},
	}

	total := 0
	for _, l := range links {
		if createLink(l.from, l.to) {
			total++
		}
	}
	fmt.Printf("Department leads → members: %d new links\n", total)

	fmt.Println("\nDONE. Restart goclaw to activate all delegation chains.")
}
