"use client";

import { motion } from "framer-motion";
import { Bot, User } from "lucide-react";

import { Avatar, AvatarFallback } from "@/components/ui/avatar";
import { WeaveLine } from "@/components/weave-line";
import { cn } from "@/lib/utils";
import type { ChatMessage } from "@/lib/use-chat";

export function ChatMessageBubble({ message }: { message: ChatMessage }) {
  const isUser = message.role === "user";

  return (
    <motion.div
      className={cn("flex w-full gap-3", isUser && "flex-row-reverse")}
      initial={{ opacity: 0, y: 12, scale: 0.98 }}
      animate={{ opacity: 1, y: 0, scale: 1 }}
      transition={{ type: "spring", stiffness: 300, damping: 26 }}
    >
      <Avatar className={cn("mt-0.5 size-8 shrink-0", !isUser && "glow-cool")}>
        <AvatarFallback
          className={cn(
            isUser
              ? "bg-secondary text-secondary-foreground"
              : "text-white"
          )}
          style={!isUser ? { background: "linear-gradient(135deg, var(--thread-cool), var(--thread-warm))" } : undefined}
        >
          {isUser ? <User className="size-4" /> : <Bot className="size-4" />}
        </AvatarFallback>
      </Avatar>

      <div className={cn("flex max-w-[75%] flex-col gap-1.5", isUser && "items-end")}>
        <div
          className={cn(
            "rounded-2xl px-4 py-2.5 text-sm leading-relaxed whitespace-pre-wrap shadow-elevated",
            isUser
              ? "rounded-tr-sm text-primary-foreground"
              : "glass rounded-tl-sm"
          )}
          style={isUser ? { background: "linear-gradient(135deg, var(--thread-cool), color-mix(in oklch, var(--thread-cool) 70%, var(--thread-warm)))" } : undefined}
        >
          {message.content || (message.pending ? <TypingDots /> : "")}
        </div>
        {message.toolUsed && (
          <WeaveLine toolUsed={message.toolUsed} connectorUsed={message.connectorUsed ?? ""} />
        )}
      </div>
    </motion.div>
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
