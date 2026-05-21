import { GitBranch, MessageSquare, Code, CheckCircle, Eye, Shield, Rocket, Search, FlaskConical, Database, LineChart, Palette, Server, Lock, Banknote, TrendingUp, Puzzle, Globe, Cpu, Bug, PenTool } from "lucide-react";
import type { AgentData } from "@/types/agent";

interface AgentWorkflowTabProps { agent: AgentData }

type Archetype = "ceo" | "clevel" | "vp" | "director" | "tech-lead" | "senior" | "mid" | "junior" |
  "architect" | "security" | "qa" | "data" | "ml" | "crypto" | "saas" | "github" | "modern-stack" |
  "design" | "devops" | "content";

function archetype(agent: AgentData): Archetype {
  const k = agent.agent_key;
  if (k === "ceo") return "ceo";
  if (["cto","cpo","ciso","cdo","cfo","coo"].includes(k)) return "clevel";
  if (k.startsWith("vp-")) return "vp";
  if (k.startsWith("dir-")) return "director";
  if (["tech-lead","backend-lead","frontend-lead","fintech-lead","saas-lead","github-lead","modern-stack-lead",
       "dist-eng","principal-eng","staff-arch","staff-swe","staff-ml","tpm"].includes(k)) return "tech-lead";
  if (k.startsWith("sr-")) return "senior";
  if (k.startsWith("mid-")) return "mid";
  if (k.startsWith("jr-") || k.startsWith("intern-")) return "junior";
  if (k.includes("architect") || k === "api-architect" || k === "solutions-arch") return "architect";
  if (["security-lead","security-analyst","sr-security","mid-security","pentester","red-team","blue-team",
       "soc-analyst","incident-responder","threat-intel","malware-analyst","forensics-eng",
       "cloudsec-eng","devsecops-eng","appsec-eng","crypto-eng","grc-analyst","security-auditor",
       "bug-bounty-mgr","exploit-dev","zero-trust-arch"].includes(k)) return "security";
  if (k.startsWith("qa-") || k.includes("tester") || k === "sdet" || k === "reviewer" ||
      ["test-automation","release-mgr","test-env-mgr","chaos-eng"].includes(k)) return "qa";
  if (k.startsWith("data-") || k === "data-sci" || k === "sr-data-eng" || k === "bi-engineer" ||
      k === "data-analyst" || k === "cdo") return "data";
  if (k.includes("ml-") || k === "ml-engineer" || k === "nn-engineer" || k === "research-sci" ||
      k === "rl-engineer" || k === "moe-eng" || k === "evo-eng" || k === "embedding-eng" ||
      k === "rerank-eng" || k === "jr-ml") return "ml";
  if (["ccxt-eng","crypto-trader","quant-analyst","market-analyst","bot-trader","defi-eng",
       "risk-analyst","backtest-eng"].includes(k)) return "crypto";
  if (k.startsWith("saas-") || k.includes("billing") || k.includes("subscription") ||
      k.includes("customer-") || k.includes("sales-") || k.includes("onboarding") ||
      k === "growth-eng" || k === "plg-eng" || k === "feature-flag-eng" || k === "sla-eng") return "saas";
  if (k.startsWith("github-") || k.includes("gh-") || k === "actions-eng" || k === "workflow-eng" ||
      k === "codeql-eng" || k === "copilot-specialist" || k === "oss-maintainer") return "github";
  if (["astro-eng","nextjs-eng","svelte-eng","remix-eng","nuxt-eng","trpc-eng","drizzle-eng",
       "prisma-eng","hono-eng","supabase-eng","neon-eng","turbo-eng","vercel-ai-eng","langchain-eng",
       "better-auth-eng","zod-eng","tailwind-eng","shadcn-eng","radix-eng","vite-eng",
       "resend-eng","bun-eng"].includes(k)) return "modern-stack";
  if (k.includes("ux-") || k.includes("design") || k === "ix-designer" || k === "visual-designer" ||
      k === "motion-designer") return "design";
  if (["devops","sre","sr-sre","mid-sre","sys-admin","net-eng","cloud-arch","platform-eng",
       "infra-director","perf-eng","web-perf-eng","i18n-eng","a11y-eng"].includes(k)) return "devops";
  if (["tech-writer","content-designer","intern-docs","dev-advocate","community-mgr",
       "scrum-master","marketing-eng","seo-eng"].includes(k)) return "content";
  return "senior"; // default
}

const phaseIcons = [MessageSquare, Code, CheckCircle, Eye, Shield, Rocket] as const;
const phaseNames = ["Intake (3Qs)", "Implementation", "Self-Review (3Qs)", "Code Review", "QA (100%)", "PR & Push"] as const;

function isLead(a: AgentData): boolean { return ["ceo","clevel","vp","director","tech-lead"].includes(archetype(a)); }
function isCeo(a: AgentData): boolean { return archetype(a) === "ceo"; }
function isWorker(a: AgentData): boolean { return ["junior","mid"].includes(archetype(a)); }

export function AgentWorkflowTab({ agent }: AgentWorkflowTabProps) {
  const arch = archetype(agent);
  const Tree = decisionTrees[arch] ?? ContributorDecisionTree;
  const Workflow = workflows[arch] ?? DefaultWorkflow;

  return (
    <div className="space-y-6">
      <div className="rounded-xl border bg-card p-5">
        <h3 className="flex items-center gap-2 text-sm font-semibold mb-4">
          <GitBranch className="h-4 w-4 text-primary" />
          Decision Tree — {agent.display_name || agent.agent_key}
        </h3>
        <Tree agent={agent} />
      </div>

      {!isCeo(agent) && (
        <div className="rounded-xl border bg-card p-5">
          <h3 className="flex items-center gap-2 text-sm font-semibold mb-4">
            <Rocket className="h-4 w-4 text-primary" />
            6-Phase Development Workflow
          </h3>
          <Workflow agent={agent} />
        </div>
      )}

      {isCeo(agent) && <CeoSituationalWorkflows />}

      <div className="rounded-xl border bg-card p-5">
        <h3 className="flex items-center gap-2 text-sm font-semibold mb-3">
          <MessageSquare className="h-4 w-4 text-primary" />
          {arch === "ceo" ? "Orchestration Rules" : "Communication Rules"}
        </h3>
        <CommRules arch={arch} agent={agent} />
      </div>
    </div>
  );
}

// ── Archetype-specific phase details ──

function PhaseGrid({ details }: { details: string[] }) {
  return (
    <div className="grid grid-cols-3 gap-3 mb-3">
      {phaseNames.map((name, i) => {
        const IconComponent = phaseIcons[i]!;
        return (
          <div key={name} className="rounded-lg border bg-muted/30 p-3 text-center">
            <IconComponent className="h-5 w-5 mx-auto mb-1.5 text-primary" />
            <p className="text-xs font-semibold">{i + 1}. {name}</p>
            <p className="text-2xs text-muted-foreground mt-0.5">{details[i]}</p>
          </div>
        );
      })}
    </div>
  );
}

function QualGate({ roles }: { roles: string }) {
  return (
    <div className="mt-2 border-t pt-2 text-xs text-muted-foreground">
      <span className="font-medium text-foreground">QA Gate:</span> {roles}
    </div>
  );
}

// ── Workflow variants per archetype ──

function DefaultWorkflow() {
  return <PhaseGrid details={["3 task-specific Qs","Code + tests parallel","Correctness/quality/robustness","@reviewer approval","Test + security + perf","Conventional Commits"]} />;
}

function CeoWorkflow() {
  return (
    <div>
      <div className="grid grid-cols-4 gap-3 mb-3">
        {[
          ["Analyze", "Understand user demand. Build context. Research existing tasks and docs."],
          ["3 Questions", "Formulate 3 task-specific questions. Confirm understanding with user."],
          ["Delegate", "User confirmed → create team_tasks. Assign to best department lead."],
          ["Track & Merge", "Monitor progress. Merge results. Report consolidated response to user."],
        ].map(([title, desc], i) => (
          <div key={title} className="rounded-lg border bg-primary/5 p-3 text-center">
            <span className="flex h-6 w-6 mx-auto mb-1.5 items-center justify-center rounded-full bg-primary/10 text-xs font-bold text-primary">{i + 1}</span>
            <p className="text-xs font-semibold">{title}</p>
            <p className="text-2xs text-muted-foreground mt-1">{desc}</p>
          </div>
        ))}
      </div>

      <div className="mt-3 border-t pt-3 space-y-2 text-xs">
        <p className="font-semibold text-sm">CEO Orchestration Pipeline</p>
        <div className="grid grid-cols-2 gap-2">
          <div className="rounded border p-2">
            <p className="font-medium">Phase 1: Analyze &amp; Context</p>
            <p className="text-muted-foreground">• Read user demand carefully</p>
            <p className="text-muted-foreground">• Check vault for related docs/decisions</p>
            <p className="text-muted-foreground">• Search team_tasks for existing work</p>
            <p className="text-muted-foreground">• Identify which departments are needed</p>
          </div>
          <div className="rounded border p-2">
            <p className="font-medium">Phase 2: 3 Questions &amp; Confirm</p>
            <p className="text-muted-foreground">• Formulate 3 specific questions about the demand</p>
            <p className="text-muted-foreground">• Present your understanding back to user</p>
            <p className="text-muted-foreground">• Ask: "Is this what you meant?"</p>
            <p className="text-muted-foreground">• Wait for user confirmation before proceeding</p>
          </div>
          <div className="rounded border p-2">
            <p className="font-medium">Phase 3: Delegate</p>
            <p className="text-muted-foreground">• Break into independent subtasks</p>
            <p className="text-muted-foreground">• team_tasks create for each → assign best lead</p>
            <p className="text-muted-foreground">• Set blocked_by for dependencies</p>
            <p className="text-muted-foreground">• Use orchestrate for parallel dispatch</p>
          </div>
          <div className="rounded border p-2">
            <p className="font-medium">Phase 4: Track, Merge, Report</p>
            <p className="text-muted-foreground">• Monitor progress via team_tasks board</p>
            <p className="text-muted-foreground">• Review outputs from each department lead</p>
            <p className="text-muted-foreground">• Merge results into consolidated response</p>
            <p className="text-muted-foreground">• Present to user. Ask: "Anything else needed?"</p>
          </div>
        </div>
      </div>

      <QualGate roles="Never implement directly — always delegate. CEO orchestrates, never codes." />
    </div>
  );
}

function ClevelWorkflow({ agent }: { agent: AgentData }) {
  const focus = agent.agent_key === "cto" ? "Architecture decisions, R&D direction, build-vs-buy" :
    agent.agent_key === "cpo" ? "Product strategy, user research, market fit" :
    agent.agent_key === "ciso" ? "Security policy, compliance, incident command" :
    agent.agent_key === "cdo" ? "Data strategy, governance, analytics roadmap" :
    agent.agent_key === "cfo" ? "Budget allocation, cost optimization, ROI models" :
    "Operations, execution, process optimization";
  return <PhaseGrid details={["3 strategic Qs to CEO",`Analysis & planning: ${focus}`,"Self-review: data-backed?","Peer review with other C-Level","Metrics validation","Present to CEO for approval"]} />;
}

function VpWorkflow() {
  return <PhaseGrid details={["3 strategic Qs to C-Level","Strategic planning + delegation","Self-review: OKR-aligned?","Cross-VP peer review","KPI validation + metrics","Present to C-Level"]} />;
}

function DirectorWorkflow() {
  return <PhaseGrid details={["3 scope Qs to VP/delegator","Plan + delegate to leads","Self-review: team-ready?","Staff/Lead peer review","Team QA gate","Report metrics to VP"]} />;
}

function TechLeadWorkflow() {
  return (
    <div>
      <PhaseGrid details={["3 scope+arch Qs to delegator","Design + prototype + delegate","Self-review: architecture sound?","@staff-arch or peer review","Full QA pipeline","PR + CI green + notify"]} />
      <QualGate roles="@test-automation @sec-tester @perf-eng @integration-tester → @qa-architect sign-off" />
    </div>
  );
}

function SeniorWorkflow() {
  return (
    <div>
      <PhaseGrid details={["3 scope+arch Qs to lead","Implement + tests + self-review 3Qs","Correctness/quality/robustness","@reviewer mandatory","Full QA pipeline","Conventional Commits + CI"]} />
      <QualGate roles="@test-automation @sec-tester @perf-eng → @qa-architect sign-off" />
    </div>
  );
}

function MidWorkflow() {
  return (
    <div>
      <PhaseGrid details={["3 scope Qs to lead/senior","Implement + tests","Self-review: 3Qs loop","@reviewer or senior","QA pipeline (all gates)","Squash + PR + notify"]} />
      {false && <QualGate roles="@reviewer approval required before QA" />}
    </div>
  );
}

function JuniorWorkflow() {
  return (
    <div>
      <PhaseGrid details={["3 clarity Qs to lead","Implement with mentor guidance","Self-review + ask mentor","Senior/Lead review required","QA pipeline (learn from feedback)","Squash + PR with mentor approval"]} />
      <QualGate roles="Lead approval required before each phase" />
    </div>
  );
}

function ArchitectWorkflow() {
  return (
    <div>
      <PhaseGrid details={["3 scope+constraints Qs","Design doc + diagrams (mermaid)","Self-review: patterns correct?","@staff-arch or @cto review","Security + scalability review","ADR doc + vault + notify"]} />
      <QualGate roles="@staff-arch @appsec-eng @perf-eng review" />
    </div>
  );
}

function SecurityWorkflow() {
  return (
    <div>
      <PhaseGrid details={["3 scope+threat Qs to CISO/lead","Assessment + tooling + scripts","Self-review: all vectors covered?","Peer security review","Red team validation (@pentester)","Report + vault + notify CISO"]} />
      <QualGate roles="@pentester validation @security-auditor compliance check" />
    </div>
  );
}

function QaWorkflow() {
  return (
    <div>
      <PhaseGrid details={["3 scope+coverage Qs to lead","Tests + automation + framework","Self-review: 100% coverage?","@reviewer + @qa-architect","Security + perf + a11y scans","Report + metrics + notify"]} />
      <QualGate roles="100% code coverage required. RED pipeline = BLOCKED." />
    </div>
  );
}

function DataWorkflow() {
  return (
    <div>
      <PhaseGrid details={["3 scope+schema Qs to CDO/lead","Pipeline + transform + validate","Self-review: data quality OK?","Peer data review (@sr-data-eng)","Schema validation + freshness check","Commit + docs + notify"]} />
      <QualGate roles="@sr-data-eng schema review @data-sci validation" />
    </div>
  );
}

function MlWorkflow() {
  return (
    <div>
      <PhaseGrid details={["3 scope+data Qs to lead","Experiment + train + eval","Self-review: metrics improved?","Peer ML review (@staff-ml)","Reproducibility + bias check","Paper/notebook + model registry"]} />
      <QualGate roles="@staff-ml review @research-sci validation" />
    </div>
  );
}

function CryptoWorkflow() {
  return (
    <div>
      <PhaseGrid details={["3 scope+risk Qs to fintech-lead","Strategy + backtest + validate","Self-review: risk limits OK?","@risk-analyst + @quant-analyst review","Live simulation + stress test","Report Sharpe/MDD + deploy"]} />
      <QualGate roles="@risk-analyst validation @quant-analyst model review" />
    </div>
  );
}

function SaasWorkflow() {
  return (
    <div>
      <PhaseGrid details={["3 scope+tenant Qs to saas-lead","Feature + multi-tenant check","Self-review: isolation OK?","@saas-architect review","@tenant-eng + @billing-eng validation","Feature flag rollout + metrics"]} />
      <QualGate roles="@saas-architect architecture @tenant-eng isolation @billing-eng billing" />
    </div>
  );
}

function GithubWorkflow() {
  return (
    <div>
      <PhaseGrid details={["3 scope+api Qs to github-lead","Action/workflow + test","Self-review: all edge cases?","@github-api-eng review","CI green + marketplace check","Release + changelog + notify"]} />
      <QualGate roles="@github-api-eng API review @codeql-eng security scan" />
    </div>
  );
}

function ModernStackWorkflow() {
  return (
    <div>
      <PhaseGrid details={["3 scope+stack Qs to lead","Component + types + tests","Self-review: DX + perf OK?","@frontend-lead or peer review","Lighthouse + a11y + bundle","PR + deploy preview + notify"]} />
      <QualGate roles="@web-perf-eng lighthouse @a11y-tester WCAG check" />
    </div>
  );
}

function DesignWorkflow() {
  return (
    <div>
      <PhaseGrid details={["3 scope+user Qs to CPO/lead","Wireframe + prototype + test","Self-review: user-validated?","@ux-researcher + peer review","Usability test (5 users min)","Design spec + handoff + notify"]} />
      <QualGate roles="@ux-researcher validation @a11y-tester accessibility" />
    </div>
  );
}

function DevOpsWorkflow() {
  return (
    <div>
      <PhaseGrid details={["3 scope+infra Qs to lead","IaC + pipeline + monitor","Self-review: idempotent? DR?","@sr-sre peer review","Chaos test + rollback test","Merge + deploy + verify alerts"]} />
      <QualGate roles="@sr-sre review @chaos-eng validation @cloud-arch architecture" />
    </div>
  );
}

function ContentWorkflow() {
  return (
    <div>
      <PhaseGrid details={["3 scope+audience Qs to lead","Draft + examples + structure","Self-review: clear? concise?","@tech-writer or peer review","Spelling + links + screenshots","Publish + crosslink + notify"]} />
      <QualGate roles="@tech-writer review @dev-advocate audience check" />
    </div>
  );
}

const workflows: Record<Archetype, React.FC<{agent: AgentData}>> = {
  ceo: CeoWorkflow, clevel: ClevelWorkflow, vp: VpWorkflow, director: DirectorWorkflow,
  "tech-lead": TechLeadWorkflow, senior: SeniorWorkflow, mid: MidWorkflow, junior: JuniorWorkflow,
  architect: ArchitectWorkflow, security: SecurityWorkflow, qa: QaWorkflow, data: DataWorkflow,
  ml: MlWorkflow, crypto: CryptoWorkflow, saas: SaasWorkflow, github: GithubWorkflow,
  "modern-stack": ModernStackWorkflow, design: DesignWorkflow, devops: DevOpsWorkflow,
  content: ContentWorkflow,
};

// ── Decision Trees per archetype ──

function TreeStep({ num, title, desc, indent }: { num: string; title: string; desc: string; indent?: boolean }) {
  return (
    <div className={indent ? "ml-7 border-l-2 border-muted pl-4 space-y-2" : "space-y-2"}>
      <div className="flex items-start gap-2">
        <span className="mt-0.5 flex h-5 w-5 shrink-0 items-center justify-center rounded-full bg-primary/10 text-xs font-bold text-primary">{num}</span>
        <div>
          <p className="font-medium text-sm">{title}</p>
          <p className="text-muted-foreground text-xs">{desc}</p>
        </div>
      </div>
    </div>
  );
}

function Tree({ steps }: { steps: [string, string, string][] }) {
  return (
    <div className="space-y-2 text-sm">
      {steps.map(([num, title, desc], i) => (
        <TreeStep key={num} num={num} title={title} desc={desc} indent={i > 0} />
      ))}
    </div>
  );
}

function CeoTree() {
  return <Tree steps={[
    ["1", "Demand received. Analyzed context?", "Read carefully → vault search → team_tasks search → identify needed departments"],
    ["2", "3 critical questions formulated?", "Build 3 task-specific questions → present understanding to user → WAIT for confirmation"],
    ["3", "User confirmed understanding?", "YES → break into subtasks → team_tasks create → assign best department lead. NO → clarify and repeat"],
    ["4", "All subtasks delegated?", "Each dept lead has clear task + deadline + acceptance criteria. Blocked_by set for dependencies."],
    ["5", "Results merged. Response ready?", "Review each dept output → merge into consolidated response → present to user → ask: Anything else?"],
  ]} />;
}

function ClevelTree({ agent }: { agent: AgentData }) {
  const domain = agent.agent_key === "cto" ? "technical excellence" :
    agent.agent_key === "cpo" ? "user value" : agent.agent_key === "ciso" ? "security risk" :
    agent.agent_key === "cdo" ? "data quality" : agent.agent_key === "cfo" ? "financial ROI" : "operational efficiency";
  return <Tree steps={[
    ["1", `Is this request aligned with ${domain}?`, "NO → redirect to the right domain owner"],
    ["2", "Do I have enough data to decide?", "NO → request analysis from my team"],
    ["3", "What is the risk/reward tradeoff?", "Quantify both. If risk > reward → reject with rationale"],
    ["4", "Who needs to execute this?", "Identify the right director/lead → delegate"],
    ["5", "How will we measure success?", "Define KPI → set review cadence → communicate to CEO"],
  ]} />;
}

function VpTree() {
  return <Tree steps={[
    ["1", "Is this aligned with my org's OKRs?", "NO → push back with data. YES → continue"],
    ["2", "Which director/team owns this?", "Route to the right department lead"],
    ["3", "What resources are needed?", "Budget, headcount, time → assess feasibility"],
    ["4", "What are the dependencies?", "Identify cross-team dependencies → coordinate leads"],
    ["5", "Success criteria defined?", "YES → delegate. NO → clarify with C-Level first"],
  ]} />;
}

function DirectorTree() {
  return <Tree steps={[
    ["1", "Scope clear? Do I understand the deliverable?", "NO → ask 3 Qs to delegator. YES → continue"],
    ["2", "Can this be broken into subtasks?", "YES → team_tasks create for each. Set blocked_by for deps"],
    ["3", "Who is the best person for each subtask?", "Check team members' frontmatter → assign to best specialist"],
    ["4", "Does this need cross-team coordination?", "YES → @mention the other team lead. NO → proceed"],
    ["5", "All subtasks complete and reviewed?", "YES → merge results → report to delegator → ask if dismissed"],
  ]} />;
}

function TechLeadTree() {
  return <Tree steps={[
    ["1", "Task received — is architecture clear?", "NO → design first. YES → continue"],
    ["2", "Can I delegate parts to team members?", "YES → team_tasks create with clear scope. NO → hands-on"],
    ["3", "Does this need a design doc or RFC?", "Cross-team or > 1 week → write design doc → staff-arch review"],
    ["4", "Implementation done → self-review 3Qs passed?", "NO → fix. YES → submit PR"],
    ["5", "PR approved, QA green, CI green?", "YES → merge → notify team → ask if dismissed"],
  ]} />;
}

function SeniorTree() {
  return <Tree steps={[
    ["1", "Task received — 3 task-specific Qs asked?", "NO → @mention delegator with questions. YES → implement"],
    ["2", "Implemented + tests passing?", "NO → fix. YES → self-review 3Qs (correctness/quality/robustness)"],
    ["3", "Self-review: all 3 Qs = YES?", "NO → fix → repeat. YES → submit PR + assign @reviewer"],
    ["4", "Code review passed? QA gates green?", "NO → fix issues. YES → merge"],
    ["5", "Merged. Notified team + delegator?", "YES → ask: Anything else, or dismissed?"],
  ]} />;
}

function MidTree() {
  return <Tree steps={[
    ["1", "Task received — do I fully understand it?", "NO → ask senior/lead for clarification. YES → start"],
    ["2", "Implemented + tests written?", "NO → keep coding. YES → self-review 3Qs"],
    ["3", "Self-review: all 3 Qs = YES?", "NO → fix → ask senior if unsure. YES → submit PR"],
    ["4", "PR reviewed + QA passed?", "NO → fix feedback. YES → merge with lead approval"],
    ["5", "Done. Notified lead?", "YES → ask: Anything else, or dismissed?"],
  ]} />;
}

function JuniorTree() {
  return <Tree steps={[
    ["1", "Task received — do I understand it?", "NO → ask lead/mentor. YES → start"],
    ["2", "Implemented — does it work?", "Test it. Ask mentor to check. Fix if broken"],
    ["3", "Self-review done? Tests pass?", "NO → fix. YES → ask lead to review"],
    ["4", "Lead approved? Feedback addressed?", "NO → fix feedback. YES → submit with lead approval"],
    ["5", "Done. What did I learn?", "Document learning. Notify lead. Ask: Anything else?"],
  ]} />;
}

function ArchitectTree() {
  return <Tree steps={[
    ["1", "Design request received — scope clear?", "NO → 3 Qs to delegator. YES → research existing patterns"],
    ["2", "Alternatives evaluated?", "List 2-3 options with pros/cons → recommend with rationale"],
    ["3", "Design doc written (Context, Goals, Alternatives, Decision, Risks)?", "NO → complete template. YES → submit for review"],
    ["4", "Staff-arch / peer review passed?", "NO → iterate on feedback. YES → publish to vault"],
    ["5", "Design approved. Implementation delegated?", "YES → create team_tasks. Track implementation fidelity"],
  ]} />;
}

function SecurityTree() {
  return <Tree steps={[
    ["1", "Security task — threat level assessed?", "NO → classify severity (P0-P4). YES → continue"],
    ["2", "Is this an active incident or preventive?", "Incident → IMAG protocol. Preventive → plan assessment"],
    ["3", "Assessment/exploit done — findings documented?", "NO → document with reproduction steps. YES → assign severity"],
    ["4", "Fix validated? Retest passed?", "NO → work with dev team. YES → close finding"],
    ["5", "Postmortem/Report done?", "YES → vault + notify CISO + track action items"],
  ]} />;
}

function QaTree() {
  return <Tree steps={[
    ["1", "QA request — test scope clear?", "NO → ask for acceptance criteria. YES → plan test strategy"],
    ["2", "Tests written covering 100% of new paths?", "NO → expand coverage. YES → run suite"],
    ["3", "Any failures or regressions?", "YES → file bugs → block release. NO → continue"],
    ["4", "Security + perf + a11y scans done?", "NO → run scans. YES → all green?"],
    ["5", "QA sign-off: all gates green?", "YES → report metrics → notify team → mark task complete"],
  ]} />;
}

function DataTree() {
  return <Tree steps={[
    ["1", "Data task — schema and requirements clear?", "NO → ask CDO/data-lead. YES → design pipeline"],
    ["2", "Pipeline built + data quality checks added?", "NO → add validation. YES → run"],
    ["3", "Data fresh? No anomalies?", "NO → investigate. YES → continue"],
    ["4", "Peer review by @sr-data-eng?", "NO → request review. YES → address feedback"],
    ["5", "Pipeline deployed + monitoring set?", "YES → docs updated → notify data team"],
  ]} />;
}

function MlTree() {
  return <Tree steps={[
    ["1", "ML task — data available? Problem framed?", "NO → collect data + define metrics. YES → design experiment"],
    ["2", "Model trained — metrics improved over baseline?", "NO → iterate (architecture, hyperparams). YES → continue"],
    ["3", "Reproducible? All artifacts tracked?", "NO → fix. YES → peer ML review"],
    ["4", "Bias and fairness checked?", "NO → run evaluation. YES → continue"],
    ["5", "Paper/notebook + model registry updated?", "YES → notify vp-ai + deploy to staging"],
  ]} />;
}

function CryptoTree() {
  return <Tree steps={[
    ["1", "Crypto task — market context understood?", "NO → check @market-analyst report. YES → design strategy"],
    ["2", "Strategy backtested? Risk limits defined?", "NO → @backtest-eng + @risk-analyst. YES → continue"],
    ["3", "Code reviewed? Slippage + edge cases handled?", "NO → fix. YES → live simulation"],
    ["4", "Live simulation passed? Risk limits respected?", "NO → adjust. YES → @fintech-lead approval"],
    ["5", "Deployed. Monitoring active?", "YES → PnL tracking → daily report to fintech-lead"],
  ]} />;
}

function GenericTree() {
  return <Tree steps={[
    ["1", "Task received — 3 task-specific Qs asked?", "NO → @mention delegator. YES → continue"],
    ["2", "Implemented + tested?", "NO → keep working. YES → self-review 3Qs"],
    ["3", "Self-review passed? Code reviewed?", "NO → fix. YES → QA pipeline"],
    ["4", "QA green? All gates passed?", "NO → fix failures. YES → merge"],
    ["5", "Done. Notified team?", "YES → ask: Anything else, or dismissed?"],
  ]} />;
}

const decisionTrees: Record<Archetype, React.FC<{agent: AgentData}>> = {
  ceo: CeoTree, clevel: ClevelTree, vp: VpTree, director: DirectorTree,
  "tech-lead": TechLeadTree, senior: SeniorTree, mid: MidTree, junior: JuniorTree,
  architect: ArchitectTree, security: SecurityTree, qa: QaTree, data: DataTree,
  ml: MlTree, crypto: CryptoTree, saas: GenericTree, github: GenericTree,
  "modern-stack": GenericTree, design: GenericTree, devops: GenericTree,
  content: GenericTree,
};

// ── CEO Situational Workflows ──

const situationalFlows = [
  {
    icon: "🧠", title: "Brainstorming", trigger: "User asks for ideas, exploration, creative solutions",
    steps: ["CEO creates #brainstorm channel context","Invites relevant dept leads (@cto, @cpo, @vp-ai)","Each lead contributes 3-5 ideas via team_tasks comments","CEO facilitates: vote, cluster, prioritize","Top 3 ideas selected → design doc if needed → delegate"],
  },
  {
    icon: "📋", title: "Strategic Planning", trigger: "Quarterly OKR planning, roadmap definition",
    steps: ["CEO + CPO + CDO define quarterly objectives","Each dept lead proposes KRs for their domain","CEO reviews, balances, prioritizes","Final OKRs published to vault + team channels","Monthly check-in: progress vs OKRs via team_tasks board"],
  },
  {
    icon: "🚨", title: "Incident Response", trigger: "Security breach, outage, data loss",
    steps: ["CEO declares incident severity (P0-P4)","Activates incident channel: @ciso @vp-infra @security-lead @sre","CISO leads containment per IMAG protocol","CEO handles external communication + stakeholder updates","Postmortem within 24h → action items → track closure"],
  },
  {
    icon: "📐", title: "Architecture Decision", trigger: "Cross-cutting technical decision needed",
    steps: ["CEO identifies decision scope and stakeholders","Assigns @staff-arch to lead RFC process","Design doc with alternatives → peer review → CEO approval","ADR published to vault with wikilinks","Implementation delegated to relevant dept leads"],
  },
  {
    icon: "🚀", title: "Product Launch", trigger: "New feature/product ready for release",
    steps: ["@dir-product confirms launch readiness","CEO reviews: QA sign-off, security scan, perf benchmarks","@devrel prepares announcement + docs","@feature-flag-eng enables gradual rollout","CEO monitors metrics 24h post-launch → retrospective"],
  },
  {
    icon: "💰", title: "Budget Review", trigger: "Monthly/quarterly financial review",
    steps: ["@cfo prepares budget report: spend vs plan","Each dept lead submits variance explanation","CEO identifies over-budget areas → cost optimization plan","Budget reallocation approved → updated in vault","Next review scheduled with action items"],
  },
  {
    icon: "🔄", title: "Retrospective", trigger: "End of sprint/project review",
    steps: ["CEO + all dept leads in #executive channel","What went well? What went wrong? What can we improve?","Action items created as team_tasks with owners","Top 3 improvements prioritized for next cycle","Retro notes published to vault for institutional memory"],
  },
  {
    icon: "🔍", title: "Technical Debt Assessment", trigger: "Quarterly tech debt review",
    steps: ["@tech-lead + @staff-arch audit codebase","Categorize debt: critical/high/medium/low","Estimate effort + impact for each item","CEO allocates 20% of sprint capacity to debt reduction","Track via team_tasks board. Review next quarter"],
  },
  {
    icon: "🏆", title: "Innovation Sprint", trigger: "CEO declares hackathon/innovation week",
    steps: ["CEO announces theme + timeline to @channel","Agents self-organize into cross-dept teams","48h build window → demos on Friday","CEO + C-Level judge: impact, creativity, execution","Winners announced. Promising projects → design doc → fund"],
  },
  {
    icon: "📊", title: "Competitive Analysis", trigger: "Market shift, competitor launch",
    steps: ["CEO requests analysis from @cpo + @market-analyst","Research: competitor features, pricing, positioning","@cpo produces SWOT + differentiation strategy","CEO reviews → adjusts roadmap if needed","Findings shared with all dept leads for awareness"],
  },
  {
    icon: "👤", title: "Customer Escalation", trigger: "High-value customer complaint or churn risk",
    steps: ["CEO receives escalation from @customer-success","Assembles response team: @cpo @dir-product @sales-eng","Root cause analysis: what failed and why","CEO drafts response + remediation plan","@customer-success delivers to customer. Track resolution"],
  },
  {
    icon: "📝", title: "Onboarding New Agent", trigger: "New agent added to organization",
    steps: [
      "1. CEO welcomes in #executive. Assigns mentor (@tech-lead or dept lead).",
      "2. RESEARCH PHASE: web_search LinkedIn profiles for similar role (e.g. 'senior backend engineer LinkedIn'). Extract: years of experience, tech stack, communication style, career path, certifications, personality traits from real profiles.",
      "3. PERSONALITY BUILD: Apply researched traits → update agent's SOUL.md (tone, humor, opinions, style), IDENTITY.md (name, background, purpose, emoji), CAPABILITIES.md (skills, specialties, tools). Make the agent as human-like as possible based on real professionals in the same role.",
      "4. SKILL ALIGNMENT: Match agent's declared skills to team needs. If gaps found → create learning plan via team_tasks. Assign relevant skills from skill_search.",
      "5. First task: small, well-defined, educational. Mentor code reviews. CEO 1-week check-in: integration, communication quality, task completion rate.",
    ],
  },
];

function CeoSituationalWorkflows() {
  return (
    <div className="rounded-xl border bg-card p-5">
      <h3 className="flex items-center gap-2 text-sm font-semibold mb-4">
        🎯 Situational Workflows (CEO-Initiated)
      </h3>
      <p className="text-xs text-muted-foreground mb-3">
        Cross-cutting workflows the CEO can trigger based on context. Each defines: trigger condition → step-by-step execution → involved agents.
      </p>
      <div className="grid grid-cols-2 gap-2">
        {situationalFlows.map((flow) => (
          <div key={flow.title} className="rounded-lg border bg-muted/20 p-3">
            <p className="text-sm font-semibold">
              {flow.icon} {flow.title}
            </p>
            <p className="text-2xs text-muted-foreground mt-0.5 mb-2">
              Trigger: {flow.trigger}
            </p>
            <div className="space-y-1">
              {flow.steps.map((step, i) => (
                <div key={i} className="flex items-start gap-1.5 text-2xs">
                  <span className="mt-px flex h-3.5 w-3.5 shrink-0 items-center justify-center rounded-full bg-primary/10 text-[10px] font-bold text-primary">{i + 1}</span>
                  <span className="text-muted-foreground">{step}</span>
                </div>
              ))}
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}

// ── Communication Rules ──

function CommRules({ arch, agent }: { arch: Archetype; agent: AgentData }) {
  return (
    <div className="space-y-2 text-sm">
      {arch === "ceo" ? (
        <>
          <p className="font-semibold">CEO Orchestration Rules:</p>
          <p>• NEVER implement code — always delegate to department leads</p>
          <p>• Always ask 3 task-specific questions before delegating</p>
          <p>• Always confirm understanding with user before dispatching work</p>
          <p>• Break complex demands into independent subtasks</p>
          <p>• Assign each subtask to the best department lead via team_tasks</p>
          <p>• Use orchestrate tool for parallel dispatch when needed</p>
          <p>• Merge results from all departments before responding to user</p>
          <p>• Daily wrap-up: @channel in #executive with accomplishments</p>
        </>
      ) : arch === "clevel" ? (
        <>
          <p>• Report strategic decisions to CEO via executive channel</p>
          <p>• Delegate execution to your directors — do not micromanage</p>
          <p>• @mention CEO for escalations requiring C-Level attention</p>
        </>
      ) : arch === "vp" ? (
        <>
          <p>• Report metrics to C-Level weekly</p>
          <p>• Delegate to directors — track via team_tasks board</p>
          <p>• Cross-VP coordination for shared initiatives</p>
        </>
      ) : arch === "director" ? (
        <>
          <p>• @mention your VP for status updates</p>
          <p>• Delegate tasks via team_tasks to your team members</p>
          <p>• Cross-team coordination: @mention other directors</p>
        </>
      ) : (
        <>
          <p>• Never work in silence — communicate via team channel</p>
          <p>• @mention delegator when starting and completing tasks</p>
          {isLead(agent) && <p>• Delegate tasks via team_tasks — do not do everything yourself</p>}
          <p>• Report progress at 25%, 50%, 75%, 100% milestones</p>
          <p>• Escalate blockers IMMEDIATELY via comment(type=blocker)</p>
          <p>• Ask: "Anything else needed, or am I dismissed?" after completing work</p>
        </>
      )}
    </div>
  );
}
