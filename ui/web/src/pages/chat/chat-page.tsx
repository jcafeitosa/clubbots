import { useState, useCallback, useEffect, useRef, useMemo } from "react";
import { useTranslation } from "react-i18next";
import { useParams, useNavigate } from "react-router";
import { Eye, PanelLeftOpen, Hash } from "lucide-react";
import { useAuthStore } from "@/stores/use-auth-store";
import { useIsMobile } from "@/hooks/use-media-query";
import { cn } from "@/lib/utils";
import { ChatSidebar } from "./chat-sidebar";
import { ChatThread } from "./chat-thread";
import { ChatInput, type AttachedFile } from "@/components/chat/chat-input";
import { ChatTopBar } from "@/components/chat/chat-top-bar";
import { DropZone } from "@/components/chat/drop-zone";
import { useChatSessions } from "./hooks/use-chat-sessions";
import { useChatMessages } from "./hooks/use-chat-messages";
import { useChatSend } from "./hooks/use-chat-send";
import { isOwnSession, parseSessionKey } from "@/lib/session-key";
import { useVirtualKeyboard } from "@/hooks/use-virtual-keyboard";
import { AgentSelector } from "@/components/chat/agent-selector";
import { TaskPanel } from "@/components/chat/task-panel";

export function ChatPage() {
  const { t } = useTranslation("chat");
  const { sessionKey: urlSessionKey } = useParams<{ sessionKey: string }>();
  const navigate = useNavigate();
  const connected = useAuthStore((s) => s.connected);
  const userId = useAuthStore((s) => s.userId);

  const [scrollTrigger, setScrollTrigger] = useState(0);
  const [files, setFiles] = useState<AttachedFile[]>([]);

  // sessionKey derived from URL — single source of truth, no separate state
  const sessionKey = urlSessionKey ?? "";

  // CEO is the default orchestrator agent for #general channel
  const CEO_KEY = "ceo";
  const [agentIdFallback, setAgentIdFallback] = useState(CEO_KEY);
  const [activeChannel, setActiveChannel] = useState("general");

  // Channel → Agent mapping: each channel has a lead agent
  const channelAgentMap: Record<string, string> = {
    general: CEO_KEY, executive: CEO_KEY,
    platform: "dir-platform", backend: "backend-lead", frontend: "frontend-lead",
    "ai-research": "vp-ai", "cloud-infra": "vp-infra", security: "ciso",
    "data-intel": "cdo", "product-design": "cpo", "fintech-crypto": "fintech-lead",
    "qa-release": "dir-qa", devrel: "dir-devrel", "saas-core": "saas-lead",
    "modern-web": "modern-stack-lead", "github-cicd": "github-lead",
  };

  // Agent is confirmed: CEO is always available for #general
  const agentConfirmed = !!urlSessionKey || !!agentIdFallback;

  // Derive agentId from URL, fallback to channel-based agent, fallback to CEO
  const agentId = useMemo(() => {
    if (urlSessionKey) {
      const { agentId: parsed } = parseSessionKey(urlSessionKey);
      if (parsed) return parsed;
    }
    return agentIdFallback;
  }, [urlSessionKey, agentIdFallback]);

  // When channel changes, update the agent
  const handleChannelSelect = (channelId: string) => {
    setActiveChannel(channelId);
    const leadAgent = channelAgentMap[channelId] || CEO_KEY;
    setAgentIdFallback(leadAgent);
  };

  const {
    sessions,
    loading: sessionsLoading,
    refresh: refreshSessions,
    buildNewSessionKey,
    deleteSession,
  } = useChatSessions(agentId);

  // Auto-restore last session on page load (no URL session key)
  useEffect(() => {
    if (!urlSessionKey && !sessionsLoading && sessions.length > 0) {
      const existing = sessions.find((s) => {
        const { agentId: parsed } = parseSessionKey(s.key);
        return parsed === agentIdFallback;
      });
      if (existing) {
        navigate(`/chat/${encodeURIComponent(existing.key)}`, { replace: true });
      }
    }
  }, [urlSessionKey, sessionsLoading, sessions, agentIdFallback, navigate]);

  const {
    messages,
    streamText,
    thinkingText,
    toolStream,
    isRunning,
    isBusy,
    loading: messagesLoading,
    activity,
    blockReplies,
    teamTasks,
    expectRun,
    addLocalMessage,
  } = useChatMessages(sessionKey, agentId);

  // Refresh sessions when all work completes (main agent + team tasks)
  const prevIsBusyRef = useRef(false);
  useEffect(() => {
    if (prevIsBusyRef.current && !isBusy) {
      refreshSessions();
    }
    prevIsBusyRef.current = isBusy;
  }, [isBusy, refreshSessions]);

  const isOwn = !sessionKey || isOwnSession(sessionKey, userId);

  const handleMessageAdded = useCallback(
    (msg: { role: "user" | "assistant" | "tool"; content: string; timestamp?: number }, key?: string) => {
      addLocalMessage(msg, key);
    },
    [addLocalMessage],
  );

  const { send, abort, error: sendError } = useChatSend({
    agentId,
    onMessageAdded: handleMessageAdded,
    onExpectRun: expectRun,
  });

  const handleNewChat = useCallback(() => {
    navigate(`/chat/${encodeURIComponent(buildNewSessionKey())}`);
  }, [buildNewSessionKey, navigate]);

  const handleSessionSelect = useCallback(
    (key: string) => {
      const { agentId: parsed } = parseSessionKey(key);
      if (parsed) setAgentIdFallback(parsed);
      navigate(`/chat/${encodeURIComponent(key)}`);
    },
    [navigate],
  );

  const handleDeleteSession = useCallback(async (key: string) => {
    await deleteSession(key);
    if (key === sessionKey) {
      const next = sessions.find((s) => s.key !== key);
      if (next) {
        handleSessionSelect(next.key);
      } else {
        handleNewChat();
      }
    }
  }, [deleteSession, sessionKey, sessions, handleSessionSelect, handleNewChat]);

  const handleAgentChange = useCallback(
    (newAgentId: string) => {
      setAgentIdFallback(newAgentId);
      if (sessionKey) {
        navigate("/chat");
      }
    },
    [navigate],
  );

  const handleSend = useCallback(
    (message: string, sendFiles?: AttachedFile[]) => {
      let key = sessionKey;
      if (!key) {
        key = buildNewSessionKey();
        navigate(`/chat/${encodeURIComponent(key)}`, { replace: true });
      }
      send(message, key, sendFiles);
      setScrollTrigger((n) => n + 1);
    },
    [sessionKey, send, buildNewSessionKey, navigate],
  );

  const handleDropFiles = useCallback((dropped: File[]) => {
    setFiles((prev) => [...prev, ...dropped.map((f) => ({ file: f }))]);
  }, []);

  const handleAbort = useCallback(() => {
    abort(sessionKey);
  }, [abort, sessionKey]);

  const isMobile = useIsMobile();
  useVirtualKeyboard();
  const [chatSidebarOpen, setChatSidebarOpen] = useState(false);
  const [taskPanelOpen, setTaskPanelOpen] = useState(false);

  // Auto-open task panel when first task appears, auto-close when all done.
  const prevTaskCountRef = useRef(0);
  useEffect(() => {
    const prev = prevTaskCountRef.current;
    const curr = teamTasks.length;
    if (prev === 0 && curr > 0) setTaskPanelOpen(true);
    if (curr === 0 && prev > 0) setTaskPanelOpen(false);
    prevTaskCountRef.current = curr;
  }, [teamTasks.length]);

  const handleSessionSelectMobile = useCallback(
    (key: string) => {
      handleSessionSelect(key);
      setChatSidebarOpen(false);
    },
    [handleSessionSelect],
  );

  const handleNewChatMobile = useCallback(() => {
    handleNewChat();
    setChatSidebarOpen(false);
  }, [handleNewChat]);

  return (
    <div className="relative flex h-full overflow-hidden">
      {/* Chat Sidebar */}
      {isMobile ? (
        <>
          {chatSidebarOpen && (
            <div
              className="fixed inset-0 z-40 bg-black/50"
              onClick={() => setChatSidebarOpen(false)}
            />
          )}
          <div
            className={cn(
              "fixed inset-y-0 left-0 z-50 transition-transform duration-200 ease-in-out",
              chatSidebarOpen ? "translate-x-0" : "-translate-x-full",
            )}
          >
            <ChatSidebar
              sessions={sessions}
              sessionsLoading={sessionsLoading}
              activeSessionKey={sessionKey}
              onSessionSelect={handleSessionSelectMobile}
              onDeleteSession={handleDeleteSession}
              onNewChat={handleNewChatMobile}
              activeChannel={activeChannel}
              onChannelSelect={handleChannelSelect}
            />
          </div>
        </>
      ) : (
        <ChatSidebar
          agentId={agentId}
          onAgentChange={handleAgentChange}
          sessions={sessions}
          sessionsLoading={sessionsLoading}
          activeSessionKey={sessionKey}
          onSessionSelect={handleSessionSelect}
          onDeleteSession={handleDeleteSession}
          onNewChat={handleNewChat}
          activeChannel={activeChannel}
          onChannelSelect={handleChannelSelect}
        />
      )}

      {/* Main chat area */}
      <div className="flex min-w-0 flex-1 min-h-0 flex-col">
        {isMobile && (
          <div className="flex shrink-0 items-center border-b px-3 py-2 landscape-compact">
            <button
              onClick={() => setChatSidebarOpen(true)}
              className="rounded-md p-1.5 text-muted-foreground hover:bg-accent hover:text-accent-foreground"
              title={t("openSessions")}
            >
              <PanelLeftOpen className="h-4 w-4" />
            </button>
          </div>
        )}

        <div className="shrink-0">
          <ChatTopBar
            agentId={agentId}
            isRunning={isRunning}
            isBusy={isBusy}
            activity={activity}
            teamTasks={teamTasks}
            onToggleTaskPanel={() => setTaskPanelOpen((v) => !v)}
            taskPanelOpen={taskPanelOpen}
            session={sessions.find((s) => s.key === sessionKey) ?? null}
            channelName={activeChannel}
          />
        </div>

        {sendError && (
          <div className="shrink-0 border-b bg-destructive/10 px-4 py-2 text-sm text-destructive">
            {sendError}
          </div>
        )}

        <DropZone onDrop={handleDropFiles}>
          <ChatThread
            messages={messages}
            streamText={streamText}
            thinkingText={thinkingText}
            toolStream={toolStream}
            blockReplies={blockReplies}
            activity={activity}
            teamTasks={teamTasks}
            isRunning={isRunning}
            isBusy={isBusy}
            loading={messagesLoading}
            scrollTrigger={scrollTrigger}
            onToggleTaskPanel={() => setTaskPanelOpen((v) => !v)}
          />

          {!isOwn ? (
            <div className="mx-3 mb-3 flex items-center gap-2 rounded-xl border bg-muted/50 px-4 py-3 text-sm text-muted-foreground shadow-sm">
              <Eye className="h-4 w-4" />
              {t("readOnly")}
            </div>
          ) : (
            <div className="mx-3 mb-3 rounded-xl border bg-muted/30 shadow-sm">
              {/* Top bar: agent + channel + context */}
              <div className="flex items-center gap-2 px-3 py-2 border-b border-border/40">
                <span className="flex items-center gap-1.5 text-xs font-medium text-primary bg-primary/10 rounded-md px-2 py-0.5">
                  <Hash className="h-3 w-3" />{activeChannel}
                </span>
                <span className="text-xs text-muted-foreground">→</span>
                <AgentSelector value={agentId} onChange={handleAgentChange} />
              </div>
              {/* Input area */}
              <ChatInput
                onSend={handleSend}
                onAbort={handleAbort}
                isBusy={isBusy}
                disabled={!connected}
                files={files}
                onFilesChange={setFiles}
              />
            </div>
          )}
        </DropZone>
      </div>

      {/* Mobile overlay backdrop — must render before TaskPanel so panel sits above */}
      {isMobile && taskPanelOpen && (
        <div className="fixed inset-0 z-40 bg-black/50" onClick={() => setTaskPanelOpen(false)} />
      )}

      {/* Task panel — toggleable sidebar on the right */}
      <TaskPanel tasks={teamTasks} open={taskPanelOpen} onClose={() => setTaskPanelOpen(false)} />
    </div>
  );
}
