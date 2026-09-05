"use client";

import Link from "next/link";
import { motion } from "framer-motion";
import { ArrowRight, Blocks, Bot, Rocket } from "lucide-react";

import { WeaveLogo } from "@/components/weave-logo";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";

// Deliberately no useAuth()/session check here — this is Weave's own
// public landing page, not a per-tenant workspace. It reads the same for
// every visitor regardless of whether they're signed in anywhere, and it
// never auto-redirects based on session state; "Sign in" is an explicit
// action a visitor takes, not something decided for them.

const STEPS = [
  {
    icon: Blocks,
    title: "Connect",
    body: "Register one or more MCP servers exposing your tools and data — write your own, or scaffold one from a connector template.",
  },
  {
    icon: Bot,
    title: "Configure",
    body: "Define a bot profile: persona, which connectors/tools it can use, which channels it's reachable on, which roles can reach it.",
  },
  {
    icon: Rocket,
    title: "Deploy",
    body: "Drop it behind your web app, an embeddable widget, WhatsApp, Slack, or call the chat API directly.",
  },
];

export default function LandingPage() {
  return (
    <div className="flex flex-1 flex-col">
      <header className="glass sticky top-0 z-40 border-b">
        <div className="mx-auto flex h-14 max-w-5xl items-center justify-between px-4">
          <WeaveLogo />
          <Link href="/login" className="inline-flex">
            <Button size="sm" className="glow-cool gap-1.5">
              Sign in
              <ArrowRight className="size-3.5" />
            </Button>
          </Link>
        </div>
      </header>

      <main className="mx-auto flex w-full max-w-5xl flex-1 flex-col gap-24 px-4 py-20">
        <motion.section
          className="flex flex-col items-center gap-6 text-center"
          initial={{ opacity: 0, y: 16 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ type: "spring", stiffness: 260, damping: 24 }}
        >
          <h1 className="text-gradient font-heading text-4xl font-semibold tracking-tight sm:text-5xl">
            Weave your systems into one AI assistant.
          </h1>
          <p className="max-w-2xl text-balance text-muted-foreground sm:text-lg">
            Weave turns whatever a business or individual already runs — a CRM, a helpdesk, a calendar, an inventory
            system, a personal inbox — into a conversational AI assistant that can actually <em>act</em> on it. No
            data migration, no forking a codebase, no rebuilding your workflows inside someone else&apos;s system.
          </p>
          <div className="flex flex-wrap items-center justify-center gap-3">
            <Link href="/login">
              <Button size="lg" className="glow-cool gap-2">
                Sign in to your workspace
                <ArrowRight className="size-4" />
              </Button>
            </Link>
          </div>
        </motion.section>

        <section className="flex flex-col gap-8">
          <div className="text-center">
            <h2 className="font-heading text-2xl font-semibold tracking-tight">How it works</h2>
            <p className="text-sm text-muted-foreground">
              Weave resolves the tenant and active bot profile per request, assembles the available tools
              dynamically, and routes through a planner → agent → tools loop — every hop traced end-to-end.
            </p>
          </div>
          <div className="grid gap-4 sm:grid-cols-3">
            {STEPS.map(({ icon: Icon, title, body }, i) => (
              <motion.div
                key={title}
                initial={{ opacity: 0, y: 12 }}
                animate={{ opacity: 1, y: 0 }}
                transition={{ delay: 0.1 + i * 0.1, type: "spring", stiffness: 280, damping: 24 }}
              >
                <Card className="glass shadow-elevated h-full border-0">
                  <CardHeader>
                    <span
                      className={
                        "mb-2 flex size-10 items-center justify-center rounded-lg " +
                        (i === 1 ? "bg-warm/12 text-warm" : "bg-cool/12 text-cool")
                      }
                    >
                      <Icon className="size-5" />
                    </span>
                    <CardTitle className="font-heading text-base">
                      {i + 1}. {title}
                    </CardTitle>
                    <CardDescription>{body}</CardDescription>
                  </CardHeader>
                  <CardContent />
                </Card>
              </motion.div>
            ))}
          </div>
        </section>

        <motion.section
          className="glass shadow-elevated rounded-2xl border-0 p-8 text-center"
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          transition={{ delay: 0.3 }}
        >
          <h2 className="font-heading text-xl font-semibold tracking-tight">Not another database for your data</h2>
          <p className="mx-auto mt-2 max-w-2xl text-sm text-muted-foreground">
            Most AI-assistant products want your data inside their database. Weave inverts that: it&apos;s the{" "}
            <em>connective layer</em>, not the system of record. A tenant in Weave is just an identity plus a set of
            registered connectors plus a bot profile — the same core serves a company and an individual with a
            personal assistant, without special-casing either.
          </p>
        </motion.section>
      </main>

      <footer className="border-t border-border/60 py-6 text-center text-xs text-muted-foreground">
        Weave — pre-alpha.
      </footer>
    </div>
  );
}
