package agent

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
)

// AgentLevel represents a hierarchical level in the engineering org.
type AgentLevel int

const (
	LevelIntern    AgentLevel = iota // simple isolated tasks, learning
	LevelJunior                      // well-scoped features, tests, docs
	LevelMid                         // feature ownership, independent work
	LevelSenior                      // complex features, mentoring, review
	LevelStaff                       // cross-team architecture, technical strategy
	LevelPrincipal                   // org-wide technical direction
	LevelDistinguished               // company-wide technical strategy, external representation
	LevelDirector                    // multi-team coordination, resource allocation
	LevelVP                          // strategic vision, prioritization
	LevelCTO                         // technology vision, build-vs-buy, R&D
	LevelCEO                         // top-level decisions, external communication
)

func (l AgentLevel) String() string {
	return [...]string{"intern", "junior", "mid", "senior", "staff", "principal", "distinguished", "director", "vp", "cto", "ceo"}[l]
}

// AgentRole is a specialized function within the org.
type AgentRole string

const (
	RoleEngineer      AgentRole = "engineer"
	RoleArchitect     AgentRole = "architect"
	RoleSRE           AgentRole = "sre"
	RoleSecurity      AgentRole = "security"
	RoleQA            AgentRole = "qa"
	RoleDevOps        AgentRole = "devops"
	RolePM            AgentRole = "pm"
	RoleReviewer      AgentRole = "reviewer"
	RoleDocs          AgentRole = "docs"
	RoleDataEngineer  AgentRole = "data-engineer"
	RoleML            AgentRole = "ml"
	RoleUX            AgentRole = "ux"
	RoleTechLead      AgentRole = "tech-lead"
	RoleScrumMaster   AgentRole = "scrum-master"
	RoleDataScientist AgentRole = "data-scientist"
)

// TeamAgent represents an agent in the hierarchy.
type TeamAgent struct {
	ID       string     `json:"id"`
	Name     string     `json:"name"`
	Level    AgentLevel `json:"level"`
	Role     AgentRole  `json:"role"`
	Manager  string     `json:"manager,omitempty"` // who they report to
	Reports  []string   `json:"reports,omitempty"` // who reports to them
	Provider string     `json:"provider"`
	Model    string     `json:"model"`
	Skills   []string   `json:"skills"`
}

// Hierarchy manages the agent org chart and task routing.
type Hierarchy struct {
	mu     sync.RWMutex
	agents map[string]*TeamAgent // id → agent
	router *Router
}

// NewHierarchy creates an agent hierarchy manager.
func NewHierarchy(router *Router) *Hierarchy {
	return &Hierarchy{
		agents: make(map[string]*TeamAgent),
		router: router,
	}
}

// Register adds an agent to the hierarchy.
func (h *Hierarchy) Register(agent *TeamAgent) {
	h.mu.Lock()
	h.agents[agent.ID] = agent
	h.mu.Unlock()
	slog.Info("hierarchy: registered", "id", agent.ID, "level", agent.Level.String(), "role", string(agent.Role))
}

// OrgChart returns the full hierarchy tree.
func (h *Hierarchy) OrgChart() string {
	h.mu.RLock()
	defer h.mu.RUnlock()

	var sb strings.Builder
	sb.WriteString("Engineering Org Chart\n")
	sb.WriteString("══════════════════════\n\n")

	// Find roots (no manager)
	for _, a := range h.agents {
		if a.Manager == "" {
			h.printTree(&sb, a, 0)
		}
	}
	return sb.String()
}

func (h *Hierarchy) printTree(sb *strings.Builder, agent *TeamAgent, depth int) {
	indent := strings.Repeat("  ", depth)
	icon := levelIcon(agent.Level)
	sb.WriteString(fmt.Sprintf("%s%s %s [%s/%s]\n", indent, icon, agent.Name, agent.Level.String(), string(agent.Role)))
	for _, reportID := range agent.Reports {
		h.mu.RLock()
		if report, ok := h.agents[reportID]; ok {
			h.mu.RUnlock()
			h.printTree(sb, report, depth+1)
		} else {
			h.mu.RUnlock()
		}
	}
}

func levelIcon(l AgentLevel) string {
	switch l {
	case LevelCEO, LevelCTO:
		return "👑"
	case LevelVP:
		return "💼"
	case LevelDistinguished:
		return "🏆"
	case LevelDirector:
		return "🏢"
	case LevelPrincipal, LevelStaff:
		return "⭐"
	case LevelSenior:
		return "💎"
	case LevelMid:
		return "🔧"
	case LevelJunior:
		return "🌱"
	case LevelIntern:
		return "🎓"
	}
	return "•"
}

// RouteTask assigns a task to the appropriate agent based on complexity.
func (h *Hierarchy) RouteTask(ctx context.Context, task string) (string, error) {
	complexity := assessComplexity(task)
	level := levelForComplexity(complexity)

	h.mu.RLock()
	defer h.mu.RUnlock()

	// Find available agent at this level (prefer idle)
	for _, a := range h.agents {
		if a.Level == level {
			return a.ID, nil
		}
	}
	// Fallback: junior
	for _, a := range h.agents {
		if a.Level == LevelMid {
			return a.ID, nil
		}
	}
	return "", fmt.Errorf("hierarchy: no agent available for level %s", level.String())
}

// TaskComplexity estimates how hard a task is.
type TaskComplexity int

const (
	ComplexityTrivial  TaskComplexity = iota // fix typo, update comment
	ComplexitySimple                         // add test, small bug fix
	ComplexityModerate                       // feature, refactor
	ComplexityComplex                        // cross-module, architecture change
	ComplexityCritical                       // system-wide, security, incident
)

func assessComplexity(task string) TaskComplexity {
	lower := strings.ToLower(task)
	switch {
	case strings.Contains(lower, "security") || strings.Contains(lower, "incident") ||
		strings.Contains(lower, "vulnerability") || strings.Contains(lower, "breach"):
		return ComplexityCritical
	case strings.Contains(lower, "architecture") || strings.Contains(lower, "refactor") ||
		strings.Contains(lower, "migration") || strings.Contains(lower, "cross-module"):
		return ComplexityComplex
	case strings.Contains(lower, "feature") || strings.Contains(lower, "implement") ||
		strings.Contains(lower, "build") || strings.Contains(lower, "create"):
		return ComplexityModerate
	case strings.Contains(lower, "test") || strings.Contains(lower, "fix") ||
		strings.Contains(lower, "bug") || strings.Contains(lower, "update"):
		return ComplexitySimple
	default:
		return ComplexityModerate
	}
}

func levelForComplexity(c TaskComplexity) AgentLevel {
	switch c {
	case ComplexityCritical:
		return LevelSenior
	case ComplexityComplex:
		return LevelMid
	case ComplexityModerate:
		return LevelJunior
	case ComplexitySimple, ComplexityTrivial:
		return LevelIntern
	default:
		return LevelMid
	}
}

// Escalate moves a task up the hierarchy when it can't be resolved at current level.
func (h *Hierarchy) Escalate(agentID string) string {
	h.mu.RLock()
	agent, ok := h.agents[agentID]
	h.mu.RUnlock()
	if !ok || agent.Manager == "" {
		return agentID
	}
	return agent.Manager
}

// DefaultTeam returns a standard engineering hierarchy.
func DefaultTeam() []*TeamAgent {
	return []*TeamAgent{
		{ID: "ceo", Name: "CEO Agent", Level: LevelCEO, Role: RolePM, Manager: "", Provider: "claude-cli", Model: "opus"},
		{ID: "cto", Name: "CTO Agent", Level: LevelCTO, Role: RoleArchitect, Manager: "ceo", Provider: "claude-cli", Model: "opus"},
		{ID: "vp-eng", Name: "VP Engineering", Level: LevelVP, Role: RoleArchitect, Manager: "cto", Provider: "claude-cli", Model: "opus"},
		{ID: "dir-platform", Name: "Director Platform", Level: LevelDirector, Role: RoleArchitect, Manager: "vp-eng", Provider: "claude-cli", Model: "sonnet"},
		{ID: "dir-product", Name: "Director Product", Level: LevelDirector, Role: RolePM, Manager: "vp-eng", Provider: "claude-cli", Model: "sonnet"},
		{ID: "staff-arch", Name: "Staff Architect", Level: LevelStaff, Role: RoleArchitect, Manager: "dir-platform", Provider: "claude-cli", Model: "sonnet"},
		{ID: "sr-backend", Name: "Sr Backend", Level: LevelSenior, Role: RoleEngineer, Manager: "dir-platform", Provider: "claude-cli", Model: "sonnet"},
		{ID: "sr-frontend", Name: "Sr Frontend", Level: LevelSenior, Role: RoleEngineer, Manager: "dir-platform", Provider: "claude-cli", Model: "sonnet"},
		{ID: "mid-api", Name: "Mid API", Level: LevelMid, Role: RoleEngineer, Manager: "sr-backend", Provider: "claude-cli", Model: "sonnet"},
		{ID: "mid-fe", Name: "Mid Frontend", Level: LevelMid, Role: RoleEngineer, Manager: "sr-frontend", Provider: "claude-cli", Model: "sonnet"},
		{ID: "jr-tests", Name: "Jr Tests", Level: LevelJunior, Role: RoleQA, Manager: "mid-api", Provider: "claude-cli", Model: "sonnet"},
		{ID: "intern-docs", Name: "Intern Docs", Level: LevelIntern, Role: RoleDocs, Manager: "mid-fe", Provider: "claude-cli", Model: "haiku"},
		{ID: "sre", Name: "SRE Lead", Level: LevelSenior, Role: RoleSRE, Manager: "dir-platform", Provider: "claude-cli", Model: "sonnet"},
		{ID: "security-lead", Name: "Security Lead", Level: LevelSenior, Role: RoleSecurity, Manager: "dir-platform", Provider: "claude-cli", Model: "sonnet"},
		{ID: "devops", Name: "DevOps Lead", Level: LevelMid, Role: RoleDevOps, Manager: "dir-platform", Provider: "claude-cli", Model: "sonnet"},
		{ID: "reviewer", Name: "Code Reviewer", Level: LevelSenior, Role: RoleReviewer, Manager: "dir-platform", Provider: "claude-cli", Model: "sonnet"},
		{ID: "dist-eng", Name: "Distinguished Engineer", Level: LevelDistinguished, Role: RoleArchitect, Manager: "cto", Provider: "claude-cli", Model: "opus"},
		{ID: "tech-lead", Name: "Tech Lead", Level: LevelSenior, Role: RoleTechLead, Manager: "dir-platform", Provider: "claude-cli", Model: "sonnet"},
		{ID: "ux-designer", Name: "UX Designer", Level: LevelMid, Role: RoleUX, Manager: "sr-frontend", Provider: "claude-cli", Model: "sonnet"},
		{ID: "ml-engineer", Name: "ML Engineer", Level: LevelSenior, Role: RoleML, Manager: "dir-platform", Provider: "claude-cli", Model: "sonnet"},
		{ID: "scrum-master", Name: "Scrum Master", Level: LevelMid, Role: RoleScrumMaster, Manager: "dir-product", Provider: "claude-cli", Model: "sonnet"},
		{ID: "data-scientist", Name: "Data Scientist", Level: LevelMid, Role: RoleDataScientist, Manager: "dir-platform", Provider: "claude-cli", Model: "sonnet"},
	}
}
