"use client";

import { useCallback, useRef, useState } from "react";
import { create } from "@bufbuild/protobuf";

import { chatClient, authHeaders } from "@/lib/connect";
import { ChatStreamRequestSchema } from "@/gen/orchestrator/v1/chat_pb";
import { friendlyError } from "@/lib/auth-context";

export type ChatMessage = {
  id: string;
  role: "user" | "assistant";
  content: string;
  toolUsed?: string;
  connectorUsed?: string;
  pending?: boolean;
};

let nextId = 0;
function newId(): string {
  nextId += 1;
  return `m${nextId}`;
}

/** Drives one ChatStream conversation against orchestrator (through
 * Envoy's grpc-web translation, docs/architecture/ARCHITECTURE.md §4).
 * Owns the session_id across turns — the first turn sends none and
 * stores whatever orchestrator hands back, every turn after reuses it,
 * same contract server/session_memory.py's resolve_session() expects. */
export function useChat({ token, channel }: { token: string; channel: string }) {
  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const [sending, setSending] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const sessionIdRef = useRef<string>("");

  const send = useCallback(
    async (text: string) => {
      const trimmed = text.trim();
      if (!trimmed || sending) return;

      setError(null);
      const userMsg: ChatMessage = { id: newId(), role: "user", content: trimmed };
      const assistantId = newId();
      setMessages((prev) => [...prev, userMsg, { id: assistantId, role: "assistant", content: "", pending: true }]);
      setSending(true);

      try {
        const req = create(ChatStreamRequestSchema, { channel, message: trimmed, sessionId: sessionIdRef.current });
        let accumulated = "";
        let toolUsed = "";
        let connectorUsed = "";
        for await (const resp of chatClient.chatStream(req, authHeaders(token))) {
          if (resp.sessionId) sessionIdRef.current = resp.sessionId;
          if (resp.toolUsed) toolUsed = resp.toolUsed;
          if (resp.connectorUsed) connectorUsed = resp.connectorUsed;
          accumulated += resp.token;
          setMessages((prev) =>
            prev.map((m) =>
              m.id === assistantId ? { ...m, content: accumulated, toolUsed, connectorUsed, pending: !resp.done } : m
            )
          );
        }
      } catch (err) {
        setError(friendlyError(err));
        setMessages((prev) => prev.filter((m) => m.id !== assistantId || m.content));
      } finally {
        setSending(false);
      }
    },
    [token, channel, sending]
  );

  const reset = useCallback(() => {
    sessionIdRef.current = "";
    setMessages([]);
    setError(null);
  }, []);

  return { messages, send, sending, error, reset };
}
