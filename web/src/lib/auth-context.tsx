"use client";

// Session state for the web app: tenant_id + access_token + the logged-in
// user's role, held client-side (localStorage) since this is a browser
// SPA — orchestrator/core never trust anything from here except the JWT
// itself (docs/architecture/SECURITY.md §2's "never a client-supplied
// tenant_id" rule; tenant_id below is only ever used to ask the login
// RPC to check it, never sent as a bare claim after that).

import { createContext, useCallback, useContext, useEffect, useMemo, useState } from "react";
import { create } from "@bufbuild/protobuf";
import { ConnectError } from "@connectrpc/connect";

import { authClient } from "@/lib/connect";
import { LoginRequestSchema } from "@/gen/core/data_access/v1/auth_pb";

export type Session = {
  tenantId: string;
  token: string;
  userId: string;
  email: string;
  role: string;
};

const STORAGE_KEY = "weave.session";

const ROLE_NAMES: Record<number, string> = { 1: "owner", 2: "admin", 3: "staff", 4: "customer" };

type AuthContextValue = {
  session: Session | null;
  loading: boolean;
  login: (args: { tenantId: string; email: string; password: string }) => Promise<void>;
  logout: () => void;
};

const AuthContext = createContext<AuthContextValue | null>(null);

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [session, setSession] = useState<Session | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    // One-time hydration from localStorage on mount — an external
    // system read, not a derived-state cascade; the intentional use
    // case react-hooks/set-state-in-effect otherwise warns against.
    const raw = window.localStorage.getItem(STORAGE_KEY);
    if (raw) {
      try {
        // eslint-disable-next-line react-hooks/set-state-in-effect
        setSession(JSON.parse(raw));
      } catch {
        window.localStorage.removeItem(STORAGE_KEY);
      }
    }
    setLoading(false);
  }, []);

  const login = useCallback(async ({ tenantId, email, password }: { tenantId: string; email: string; password: string }) => {
    const resp = await authClient.login(create(LoginRequestSchema, { tenantId, email, password }));
    const next: Session = {
      tenantId,
      token: resp.accessToken,
      userId: resp.user?.Id ?? "",
      email: resp.user?.email ?? email,
      role: ROLE_NAMES[resp.user?.role ?? 0] ?? "unknown",
    };
    window.localStorage.setItem(STORAGE_KEY, JSON.stringify(next));
    setSession(next);
  }, []);

  const logout = useCallback(() => {
    window.localStorage.removeItem(STORAGE_KEY);
    setSession(null);
  }, []);

  const value = useMemo(() => ({ session, loading, login, logout }), [session, loading, login, logout]);

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth(): AuthContextValue {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error("useAuth must be used within an AuthProvider");
  return ctx;
}

/** Turns a Connect-ES error into a short, user-presentable message —
 * every RPC call site in this app funnels errors through this rather
 * than showing a raw stack/ConnectError string. */
export function friendlyError(err: unknown): string {
  if (err instanceof ConnectError) return err.rawMessage || err.message;
  if (err instanceof Error) return err.message;
  return "Something went wrong.";
}
