import { Waypoints } from "lucide-react";
import { cn } from "@/lib/utils";

export function WeaveLogo({ className }: { className?: string }) {
  return (
    <div className={cn("flex items-center gap-2 font-semibold tracking-tight", className)}>
      <span className="flex size-7 items-center justify-center rounded-lg bg-primary text-primary-foreground">
        <Waypoints className="size-4" />
      </span>
      <span>Weave</span>
    </div>
  );
}
