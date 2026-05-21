import { memo } from "react";
import { useTranslation } from "react-i18next";
import { Plus, Hash, Lock } from "lucide-react";
import { Button } from "@/components/ui/button";
import { SessionSwitcher } from "@/components/chat/session-switcher";
import { cn } from "@/lib/utils";
import type { SessionInfo } from "@/types/session";

export interface ChannelInfo {
  id: string;
  name: string;
  type: "general" | "team";
  description?: string;
}

const CHANNELS: ChannelInfo[] = [
  { id: "general", name: "general", type: "general", description: "Company-wide — all agents" },
  { id: "executive", name: "executive", type: "team", description: "C-Level strategy" },
  { id: "platform", name: "platform", type: "team", description: "Platform engineering" },
  { id: "backend", name: "backend", type: "team", description: "Backend engineering" },
  { id: "frontend", name: "frontend", type: "team", description: "Frontend engineering" },
  { id: "ai-research", name: "ai-research", type: "team", description: "AI & Research" },
  { id: "cloud-infra", name: "cloud-infra", type: "team", description: "Cloud & Infrastructure" },
  { id: "security", name: "security", type: "team", description: "Security & Compliance" },
  { id: "data-intel", name: "data-intel", type: "team", description: "Data & Intelligence" },
  { id: "product-design", name: "product-design", type: "team", description: "Product & Design" },
  { id: "fintech-crypto", name: "fintech-crypto", type: "team", description: "Fintech & Crypto" },
  { id: "qa-release", name: "qa-release", type: "team", description: "QA & Release" },
  { id: "devrel", name: "devrel", type: "team", description: "Developer Relations" },
  { id: "saas-core", name: "saas-core", type: "team", description: "SaaS Platform" },
  { id: "modern-web", name: "modern-web", type: "team", description: "Modern Web Stack" },
  { id: "github-cicd", name: "github-cicd", type: "team", description: "GitHub CI/CD" },
];

interface ChatSidebarProps {
  sessions: SessionInfo[];
  sessionsLoading: boolean;
  activeSessionKey: string;
  onSessionSelect: (key: string) => void;
  onDeleteSession?: (key: string) => void;
  onNewChat: () => void;
  activeChannel?: string;
  onChannelSelect?: (channelId: string) => void;
}

export const ChatSidebar = memo(function ChatSidebar({
  sessions, sessionsLoading, activeSessionKey,
  onSessionSelect, onDeleteSession, onNewChat,
  activeChannel, onChannelSelect,
}: ChatSidebarProps) {
  const { t } = useTranslation("chat");
  return (
    <div className="flex h-full w-72 max-w-[85vw] flex-col border-r bg-background">
      {/* Channels */}
      <div className="border-b">
        <div className="px-3 py-2 text-xs font-semibold text-muted-foreground uppercase tracking-wider">
          {t("channels", "Channels")}
        </div>
        <div className="px-2 pb-2 space-y-0.5">
          {CHANNELS.map((ch) => {
            const Icon = ch.type === "general" ? Hash : Lock;
            const isActive = activeChannel === ch.id;
            return (
              <button
                key={ch.id}
                type="button"
                onClick={() => onChannelSelect?.(ch.id)}
                className={cn(
                  "w-full flex items-center gap-2 rounded-md px-2 py-1.5 text-sm transition-colors text-left",
                  isActive
                    ? "bg-primary/10 text-primary font-medium"
                    : "text-muted-foreground hover:bg-muted/50 hover:text-foreground",
                )}
                title={ch.description}
              >
                <Icon className="h-3.5 w-3.5 shrink-0" />
                <span className="truncate">#{ch.name}</span>
              </button>
            );
          })}
        </div>
      </div>

      {/* New chat button */}
      <div className="p-3">
        <Button
          variant="outline"
          className="w-full justify-start gap-2"
          onClick={onNewChat}
        >
          <Plus className="h-4 w-4" />
          {t("newChat")}
        </Button>
      </div>

      {/* Session list */}
      <div className="flex-1 overflow-y-auto">
        <SessionSwitcher
          sessions={sessions}
          activeKey={activeSessionKey}
          onSelect={onSessionSelect}
          onDelete={onDeleteSession}
          loading={sessionsLoading}
        />
      </div>
    </div>
  );
});
