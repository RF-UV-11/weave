"use client";

import { motion } from "framer-motion";
import { Waypoints } from "lucide-react";
import { cn } from "@/lib/utils";

export function WeaveLogo({ className }: { className?: string }) {
  return (
    <div className={cn("flex items-center gap-2 font-heading font-semibold tracking-tight", className)}>
      <motion.span
        className="relative flex size-7 items-center justify-center overflow-hidden rounded-lg text-white"
        style={{ background: "linear-gradient(135deg, var(--thread-cool), var(--thread-warm))" }}
        initial={{ rotate: -8, scale: 0.9, opacity: 0 }}
        animate={{ rotate: 0, scale: 1, opacity: 1 }}
        whileHover={{ rotate: 8, scale: 1.08 }}
        transition={{ type: "spring", stiffness: 300, damping: 18 }}
      >
        <Waypoints className="size-4" />
      </motion.span>
      <span className="text-gradient">Weave</span>
    </div>
  );
}
