import { Bot, User } from "lucide-react";
import { MessageContent } from "./message-content";
import { ThinkingBlock } from "./thinking-block";
import { ToolCallCard } from "./tool-call-card";
import { BlockReplyBubble } from "./block-reply-bubble";
import { MediaGallery } from "./media-gallery";
import { useUiStore } from "@/stores/use-ui-store";
import { resolveTimezone } from "@/lib/format";
import type { ChatMessage } from "@/types/chat";

interface MessageBubbleProps {
  message: ChatMessage;
}

export function MessageBubble({ message }: MessageBubbleProps) {
  const timezone = useUiStore((s) => s.timezone);
  const isUser = message.role === "user";
  const isTool = message.role === "tool";

  if (isTool) return null;
  if (message.isNotification) return null;
  if (message.isBlockReply) return <BlockReplyBubble message={message} />;

  const isAssistant = message.role === "assistant";
  const hasThinking = isAssistant && !!message.thinking;
  const hasToolDetails = isAssistant && message.toolDetails && message.toolDetails.length > 0;
  const hasToolCalls = isAssistant && message.tool_calls && message.tool_calls.length > 0;
  const hasContent = !!message.content?.trim();

  if (isAssistant && !hasContent && !hasToolCalls && !hasToolDetails) return null;

  // Tool-only message (no text content) — render compact without bubble wrapper
  const isToolOnly = isAssistant && !hasContent && !hasThinking && (hasToolDetails || hasToolCalls);

  return (
    <div className={`flex gap-2 sm:gap-3 px-3 sm:px-4 ${isUser ? "flex-row-reverse" : ""}`}>
      {/* Avatar — larger on desktop, compact on mobile */}
      <div className={`flex shrink-0 items-center justify-center rounded-full border bg-background ${isUser ? "h-7 w-7 sm:h-8 sm:w-8 text-xs sm:text-sm" : "h-8 w-8 sm:h-9 sm:w-9 text-sm"}`}>
        {isUser ? (
          <User className="h-3.5 w-3.5 sm:h-4 sm:w-4" />
        ) : message.agentEmoji ? (
          <span>{message.agentEmoji}</span>
        ) : (
          <Bot className="h-3.5 w-3.5 sm:h-4 sm:w-4" />
        )}
      </div>

      {isToolOnly ? (
        /* Compact tool-only card — no bubble wrapper, full width */
        <div className="flex-1 min-w-0 max-w-full">
          {isAssistant && message.agentName && (
            <div className="flex items-center gap-1.5 mb-1">
              <span className="text-xs font-semibold">{message.agentName}</span>
              {message.agentRole && <span className="text-2xs text-muted-foreground hidden sm:inline">· {message.agentRole}</span>}
            </div>
          )}
          <div className="rounded-md border bg-muted divide-y divide-border overflow-x-auto">
          {hasThinking && (
            <div className="px-2 py-1.5">
              <ThinkingBlock text={message.thinking!} />
            </div>
          )}
          {hasToolDetails && message.toolDetails!.map((entry) => (
            <ToolCallCard key={entry.toolCallId} entry={entry} compact />
          ))}
        </div>
        </div>
      ) : (
        /* Normal message bubble — assistant left, user right */
        <div className={isUser ? "max-w-[85%] sm:max-w-[75%]" : "flex-1 min-w-0 max-w-full"}>
          {isAssistant && message.agentName && (
            <div className="flex items-center gap-1.5 mb-1 ml-1">
              <span className="text-xs font-semibold">{message.agentName}</span>
              {message.agentRole && <span className="text-2xs text-muted-foreground hidden sm:inline">· {message.agentRole}</span>}
            </div>
          )}
          <div className={`rounded-lg px-3 py-2 sm:px-4 sm:py-2 text-sm sm:text-base ${
            isUser
              ? "bg-primary/10 text-foreground border border-primary/20 shadow-sm"
              : "bg-card text-card-foreground border border-border shadow-sm border-l-2 border-l-primary/40"
          }`}>
          {hasThinking && (
            <div className="mb-2">
              <ThinkingBlock text={message.thinking!} />
            </div>
          )}
          {hasToolDetails && (
            <div className="mb-2 rounded-md border bg-muted divide-y divide-border">
              {message.toolDetails!.map((entry) => (
                <ToolCallCard key={entry.toolCallId} entry={entry} compact />
              ))}
            </div>
          )}
          <MessageContent content={message.content} role={message.role} mediaBasenames={message.mediaItems?.map((m) => m.path.split("/").pop() ?? "").filter(Boolean)} />
          {message.mediaItems && message.mediaItems.length > 0 && (
            <div className="mt-2">
              <MediaGallery items={message.mediaItems} />
            </div>
          )}
          {message.timestamp && (
            <div className="mt-1 text-2xs text-muted-foreground">
              {new Intl.DateTimeFormat([], {
                timeZone: resolveTimezone(timezone),
                hour: "numeric",
                minute: "2-digit",
              }).format(new Date(message.timestamp))}
            </div>
          )}
        </div>
        </div>
      )}
    </div>
  );
}
