"use client";

import { useEffect, useRef, useState, type KeyboardEvent } from "react";
import { AnimatePresence, motion } from "framer-motion";
import { MessageSquarePlus, Send, Sparkles } from "lucide-react";

import { RequireAuth } from "@/components/require-auth";
import { ChatMessageBubble } from "@/components/chat-message";
import { Button } from "@/components/ui/button";
import { Textarea } from "@/components/ui/textarea";
import { ScrollArea } from "@/components/ui/scroll-area";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { useChat } from "@/lib/use-chat";

const CHANNELS = [
  { value: "web-widget", label: "Web widget (customer)" },
  { value: "slack", label: "Slack (staff)" },
];

const SUGGESTIONS = [
  "What's the status of order ORD-1001?",
  "What's the warranty status on order ORD-1001?",
  "Tell me about the Acme Wave Headphones.",
];

export default function ChatPage() {
  return <RequireAuth>{(session) => <ChatWindow token={session.token} />}</RequireAuth>;
}

function ChatWindow({ token }: { token: string }) {
  const [channel, setChannel] = useState(CHANNELS[0].value);
  const { messages, send, sending, error, reset } = useChat({ token, channel });
  const [draft, setDraft] = useState("");
  const scrollRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    scrollRef.current?.scrollIntoView({ behavior: "smooth", block: "end" });
  }, [messages]);

  function onKeyDown(e: KeyboardEvent<HTMLTextAreaElement>) {
    if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault();
      submit();
    }
  }

  function submit() {
    if (!draft.trim() || sending) return;
    send(draft);
    setDraft("");
  }

  return (
    <div className="mx-auto flex w-full max-w-3xl flex-1 flex-col px-4">
      <div className="flex items-center justify-between gap-3 py-4">
        <Select value={channel} onValueChange={(v) => { if (v) { setChannel(v); reset(); } }}>
          <SelectTrigger className="glass w-[220px] border-0">
            <SelectValue>{(v: string) => CHANNELS.find((c) => c.value === v)?.label ?? v}</SelectValue>
          </SelectTrigger>
          <SelectContent>
            {CHANNELS.map((c) => (
              <SelectItem key={c.value} value={c.value}>
                {c.label}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        <Button variant="outline" size="sm" className="gap-1.5" onClick={reset}>
          <MessageSquarePlus className="size-4" />
          New chat
        </Button>
      </div>

      <ScrollArea className="flex-1">
        <div className="flex flex-col gap-6 pb-4">
          {messages.length === 0 ? (
            <EmptyState onPick={(text) => send(text)} />
          ) : (
            <AnimatePresence initial={false}>
              {messages.map((m) => (
                <ChatMessageBubble key={m.id} message={m} />
              ))}
            </AnimatePresence>
          )}
          <div ref={scrollRef} />
        </div>
      </ScrollArea>

      {error && <p className="pb-2 text-sm text-destructive">{error}</p>}

      <div className="sticky bottom-0 flex items-end gap-2 border-t border-border/60 bg-background/70 py-4 backdrop-blur-md">
        <Textarea
          value={draft}
          onChange={(e) => setDraft(e.target.value)}
          onKeyDown={onKeyDown}
          placeholder="Message this bot…"
          rows={1}
          className="max-h-40 min-h-11 resize-none rounded-2xl border-border/60 bg-card/60"
        />
        <motion.div whileTap={{ scale: 0.92 }} whileHover={{ scale: 1.05 }}>
          <Button
            size="icon"
            onClick={submit}
            disabled={sending || !draft.trim()}
            aria-label="Send"
            className="glow-cool rounded-full"
          >
            <Send className="size-4" />
          </Button>
        </motion.div>
      </div>
    </div>
  );
}

function EmptyState({ onPick }: { onPick: (text: string) => void }) {
  return (
    <motion.div
      className="flex flex-1 flex-col items-center justify-center gap-4 py-24 text-center"
      initial={{ opacity: 0, y: 8 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ duration: 0.4 }}
    >
      <motion.span
        className="glow-cool flex size-14 items-center justify-center rounded-2xl text-white"
        style={{ background: "linear-gradient(135deg, var(--thread-cool), var(--thread-warm))" }}
        animate={{ y: [0, -6, 0] }}
        transition={{ duration: 3, repeat: Infinity, ease: "easeInOut" }}
      >
        <Sparkles className="size-7" />
      </motion.span>
      <div className="space-y-1">
        <p className="font-heading font-medium">How can I help?</p>
        <p className="text-sm text-muted-foreground">Ask about an order, a product, or anything else.</p>
      </div>
      <div className="flex flex-wrap justify-center gap-2">
        {SUGGESTIONS.map((s, i) => (
          <motion.div
            key={s}
            initial={{ opacity: 0, y: 6 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ delay: 0.15 + i * 0.08 }}
          >
            <Button variant="outline" size="sm" className="glass border-0" onClick={() => onPick(s)}>
              {s}
            </Button>
          </motion.div>
        ))}
      </div>
    </motion.div>
  );
}
