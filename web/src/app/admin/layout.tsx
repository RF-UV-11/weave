"use client";

import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { useEffect } from "react";
import { motion } from "framer-motion";
import { Loader2, LayoutGrid, Bot, Wrench, Plug } from "lucide-react";

import { SiteHeader } from "@/components/site-header";
import { buttonVariants } from "@/components/ui/button";
import { useAuth } from "@/lib/auth-context";
import { cn } from "@/lib/utils";

const NAV = [
  { href: "/admin", label: "Overview", icon: LayoutGrid, exact: true },
  { href: "/admin/bot-profiles", label: "Bot profiles", icon: Bot },
  { href: "/admin/tools", label: "Tools", icon: Wrench },
  { href: "/admin/connectors", label: "Connectors", icon: Plug },
];

export default function AdminLayout({ children }: LayoutProps<"/admin">) {
  const { session, loading } = useAuth();
  const router = useRouter();
  const pathname = usePathname();

  useEffect(() => {
    if (!loading && !session) router.replace("/login");
  }, [loading, session, router]);

  if (loading || !session) {
    return (
      <div className="flex flex-1 items-center justify-center">
        <Loader2 className="size-6 animate-spin text-muted-foreground" />
      </div>
    );
  }

  return (
    <div className="flex min-h-full flex-1 flex-col">
      <SiteHeader />
      <div className="mx-auto flex w-full max-w-6xl flex-1 gap-8 px-4 py-6">
        <aside className="w-52 shrink-0">
          <nav className="relative flex flex-col gap-1">
            {NAV.map(({ href, label, icon: Icon, exact }) => {
              const active = exact ? pathname === href : pathname?.startsWith(href);
              return (
                <Link
                  key={href}
                  href={href}
                  className={cn(
                    buttonVariants({ variant: "ghost", size: "sm" }),
                    "relative justify-start gap-2 hover:bg-transparent",
                    active ? "font-medium text-foreground" : "text-muted-foreground"
                  )}
                >
                  <Icon className="relative z-10 size-4" />
                  <span className="relative z-10">{label}</span>
                  {active && (
                    <motion.span
                      layoutId="admin-nav-active-pill"
                      className="absolute inset-0 rounded-md bg-secondary"
                      transition={{ type: "spring", stiffness: 400, damping: 32 }}
                    />
                  )}
                </Link>
              );
            })}
          </nav>
        </aside>
        <main className="min-w-0 flex-1">{children}</main>
      </div>
    </div>
  );
}
