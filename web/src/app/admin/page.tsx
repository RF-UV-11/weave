"use client";

import { useEffect, useState } from "react";
import { create } from "@bufbuild/protobuf";
import { Bot, Wrench, Plug, Building2 } from "lucide-react";

import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { useAuth, friendlyError } from "@/lib/auth-context";
import { botProfileClient, httpToolClient, connectorClient, authHeaders } from "@/lib/connect";
import { ListBotProfilesRequestSchema } from "@/gen/core/data_access/v1/bot_profile_pb";
import { ListHttpToolsRequestSchema } from "@/gen/core/data_access/v1/http_tool_pb";
import { ListConnectorsRequestSchema } from "@/gen/core/data_access/v1/connector_pb";

export default function AdminOverviewPage() {
  const { session } = useAuth();
  const [counts, setCounts] = useState<{ profiles: number; tools: number; connectors: number } | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!session) return;
    const { tenantId, token } = session;
    const opts = authHeaders(token);
    Promise.all([
      botProfileClient.listBotProfiles(create(ListBotProfilesRequestSchema, { tenantId }), opts),
      httpToolClient.listHttpTools(create(ListHttpToolsRequestSchema, { tenantId }), opts),
      connectorClient.listConnectors(create(ListConnectorsRequestSchema, { tenantId }), opts),
    ])
      .then(([profiles, tools, connectors]) =>
        setCounts({
          profiles: profiles.botProfiles.length,
          tools: tools.httpTools.length,
          connectors: connectors.connectors.length,
        })
      )
      .catch((err) => setError(friendlyError(err)));
  }, [session]);

  if (!session) return null;

  return (
    <div className="flex flex-col gap-6">
      <div>
        <h1 className="text-xl font-semibold tracking-tight">Overview</h1>
        <p className="text-sm text-muted-foreground">Your Weave workspace at a glance.</p>
      </div>

      {error && <p className="text-sm text-destructive">{error}</p>}

      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <StatCard icon={Building2} label="Tenant" value={session.tenantId} mono />
        <StatCard icon={Bot} label="Bot profiles" value={counts?.profiles ?? "—"} />
        <StatCard icon={Wrench} label="Registered tools" value={counts?.tools ?? "—"} />
        <StatCard icon={Plug} label="Connectors" value={counts?.connectors ?? "—"} />
      </div>

      <Card>
        <CardHeader>
          <CardTitle className="text-base">Getting started</CardTitle>
          <CardDescription>How this workspace fits together.</CardDescription>
        </CardHeader>
        <CardContent className="space-y-2 text-sm text-muted-foreground">
          <p>
            <strong className="text-foreground">Connectors</strong> group the tools a business exposes to Weave —
            every tool registered via the <code className="rounded bg-muted px-1 py-0.5">weave</code> SDK lands on
            one auto-created <code className="rounded bg-muted px-1 py-0.5">weave_managed</code> connector.
          </p>
          <p>
            <strong className="text-foreground">Bot profiles</strong> decide which tools, channels, and roles a bot
            has — an <em>external</em> profile only ever sees tools marked visibility=&quot;external&quot;, an{" "}
            <em>internal</em> profile sees everything.
          </p>
        </CardContent>
      </Card>
    </div>
  );
}

function StatCard({
  icon: Icon,
  label,
  value,
  mono,
}: {
  icon: React.ComponentType<{ className?: string }>;
  label: string;
  value: string | number;
  mono?: boolean;
}) {
  return (
    <Card>
      <CardContent className="flex items-center gap-3 py-2">
        <span className="flex size-9 shrink-0 items-center justify-center rounded-lg bg-primary/10 text-primary">
          <Icon className="size-4.5" />
        </span>
        <div className="min-w-0">
          <p className="text-xs text-muted-foreground">{label}</p>
          <p className={mono ? "truncate font-mono text-sm" : "text-lg font-semibold"}>{value}</p>
        </div>
      </CardContent>
    </Card>
  );
}
