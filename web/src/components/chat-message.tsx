import { Bot, User, Wrench } from "lucide-react";

import { Avatar, AvatarFallback } from "@/components/ui/avatar";
import { Badge } from "@/components/ui/badge";
import { cn } from "@/lib/utils";
import type { ChatMessage } from "@/lib/use-chat";

export function ChatMessageBubble({ message }: { message: ChatMessage }) {
  const isUser = message.role === "user";

  return (
    <div className={cn("flex w-full gap-3", isUser && "flex-row-reverse")}>
      <Avatar className="mt-0.5 size-8 shrink-0">
        <AvatarFallback className={cn(isUser ? "bg-secondary" : "bg-primary text-primary-foreground")}>
          {isUser ? <User className="size-4" /> : <Bot className="size-4" />}
        </AvatarFallback>
      </Avatar>

      <div className={cn("flex max-w-[75%] flex-col gap-1.5", isUser && "items-end")}>
        <div
          className={cn(
            "rounded-2xl px-4 py-2.5 text-sm leading-relaxed whitespace-pre-wrap",
            isUser ? "bg-primary text-primary-foreground rounded-tr-sm" : "bg-muted rounded-tl-sm"
          )}
        >
          {message.content || (message.pending ? <TypingDots /> : "")}
        </div>
        {message.toolUsed && (
          <Badge variant="outline" className="gap-1 text-[11px] text-muted-foreground">
            <Wrench className="size-3" />
            {message.toolUsed}
            {message.connectorUsed ? ` · ${message.connectorUsed}` : ""}
          </Badge>
        )}
      </div>
    </div>
  );
}

function TypingDots() {
  return (
    <span className="flex items-center gap-1 py-1">
      {[0, 1, 2].map((i) => (
        <span
          key={i}
          className="size-1.5 animate-bounce rounded-full bg-current opacity-60"
          style={{ animationDelay: `${i * 120}ms` }}
        />
      ))}
    </span>
  );
}
