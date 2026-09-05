"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { motion } from "framer-motion";
import { LogOut, MessageSquare, Shield } from "lucide-react";

import { WeaveLogo } from "@/components/weave-logo";
import { Button, buttonVariants } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { useAuth } from "@/lib/auth-context";
import { cn } from "@/lib/utils";

const NAV = [
  { href: "/chat", label: "Chat", icon: MessageSquare },
  { href: "/admin", label: "Admin", icon: Shield },
];

export function SiteHeader() {
  const pathname = usePathname();
  const { session, logout } = useAuth();

  return (
    <header className="glass sticky top-0 z-40 border-b">
      <div className="mx-auto flex h-14 max-w-6xl items-center gap-4 px-4">
        <Link href="/chat">
          <WeaveLogo />
        </Link>

        {session && (
          <nav className="relative flex items-center gap-1">
            {NAV.map(({ href, label, icon: Icon }) => {
              const active = pathname?.startsWith(href);
              return (
                <Link
                  key={href}
                  href={href}
                  className={cn(
                    buttonVariants({ variant: "ghost", size: "sm" }),
                    "relative gap-1.5 hover:bg-transparent",
                    active ? "text-foreground font-medium" : "text-muted-foreground"
                  )}
                >
                  <Icon className="relative z-10 size-4" />
                  <span className="relative z-10">{label}</span>
                  {active && (
                    <motion.span
                      layoutId="nav-active-pill"
                      className="absolute inset-0 rounded-md bg-secondary"
                      transition={{ type: "spring", stiffness: 400, damping: 32 }}
                    />
                  )}
                </Link>
              );
            })}
          </nav>
        )}

        <div className="ml-auto flex items-center gap-2">
          {session && (
            <>
              <Badge variant="outline" className="hidden gap-1 border-cool/40 text-cool sm:flex">
                {session.role}
              </Badge>
              <Button variant="ghost" size="icon" aria-label="Log out" onClick={logout} title={session.email}>
                <LogOut className="size-4" />
              </Button>
            </>
          )}
        </div>
      </div>
    </header>
  );
}
