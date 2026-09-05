"use client";

import { motion } from "framer-motion";
import { Check, Cpu, Radio, Wrench } from "lucide-react";
import { cn } from "@/lib/utils";

/** DESIGN.md §4's signature component: a trace of a message's actual
 * path. Segments internal to Weave's own reasoning render in the cool
 * thread; a segment that touched the tenant's own connector renders in
 * the warm thread — a user should always be able to tell, at a glance,
 * when the assistant reached into *their* system versus reasoned on its
 * own (the same auditability principle SECURITY.md §7/§9 describe made
 * visible). Every step is already resolved by the time a turn finishes
 * streaming (chat_service.py's ChatStreamResponse doesn't yet emit
 * intermediate per-hop events — see PLAN.md's honest-gap notes), so this
 * renders as a completed trace with a staggered draw-in, not a live
 * step-by-step animation; the moment orchestrator streams real
 * intermediate hop events, this same component becomes the live version
 * with no markup change, just earlier `done` timing per segment. */

type WeaveStep = {
  label: string;
  kind: "cool" | "warm";
};

function buildSteps(toolUsed: string, connectorUsed: string): WeaveStep[] {
  const steps: WeaveStep[] = [
    { label: "Channel", kind: "cool" },
    { label: "Planner", kind: "cool" },
  ];
  if (toolUsed) {
    steps.push({ label: connectorUsed || "Connector", kind: "warm" });
  }
  steps.push({ label: "Response", kind: "cool" });
  return steps;
}

const ICONS: Record<string, typeof Radio> = {
  Channel: Radio,
  Planner: Cpu,
  Response: Check,
};

export function WeaveLine({ toolUsed, connectorUsed }: { toolUsed: string; connectorUsed: string }) {
  const steps = buildSteps(toolUsed, connectorUsed);

  return (
    <div className="flex items-center gap-0.5" role="img" aria-label={`Request path: ${steps.map((s) => s.label).join(" → ")}`}>
      {steps.map((step, i) => {
        const Icon = ICONS[step.label] ?? Wrench;
        const isLast = i === steps.length - 1;
        return (
          <div key={`${step.label}-${i}`} className="flex items-center">
            <motion.div
              className={cn(
                "flex items-center gap-1 rounded-full px-2 py-0.5 text-[10px] font-medium",
                step.kind === "cool" ? "bg-cool/12 text-cool" : "bg-warm/15 text-warm"
              )}
              initial={{ opacity: 0, scale: 0.7, y: 4 }}
              animate={{ opacity: 1, scale: 1, y: 0 }}
              transition={{ delay: i * 0.08, type: "spring", stiffness: 350, damping: 20 }}
            >
              <Icon className="size-2.5" />
              {step.label}
            </motion.div>
            {!isLast && (
              <motion.span
                className={cn(
                  "block h-px w-3 origin-left",
                  step.kind === "cool" ? "bg-cool/40" : "bg-warm/40"
                )}
                initial={{ scaleX: 0 }}
                animate={{ scaleX: 1 }}
                transition={{ delay: i * 0.08 + 0.05, duration: 0.2 }}
              />
            )}
          </div>
        );
      })}
    </div>
  );
}
