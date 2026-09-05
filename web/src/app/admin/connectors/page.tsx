"use client";

import { useEffect, useState } from "react";
import { create } from "@bufbuild/protobuf";
import { motion } from "framer-motion";
import { Loader2 } from "lucide-react";

import { Badge } from "@/components/ui/badge";
import { Card, CardContent } from "@/components/ui/card";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { useAuth, friendlyError } from "@/lib/auth-context";
import { connectorClient, authHeaders } from "@/lib/connect";
import { ListConnectorsRequestSchema } from "@/gen/core/data_access/v1/connector_pb";
import type { Connector } from "@/gen/database/v1/connector_pb";

export default function ConnectorsPage() {
  const { session } = useAuth();
  const [connectors, setConnectors] = useState<Connector[] | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!session) return;
    connectorClient
      .listConnectors(create(ListConnectorsRequestSchema, { tenantId: session.tenantId }), authHeaders(session.token))
      .then((resp) => setConnectors(resp.connectors))
      .catch((err) => setError(friendlyError(err)));
  }, [session]);

  if (!session) return null;

  return (
    <div className="flex flex-col gap-6">
      <div>
        <h1 className="font-heading text-xl font-semibold tracking-tight">Connectors</h1>
        <p className="text-sm text-muted-foreground">
          Every MCP-reachable endpoint this tenant has registered — hand-rolled MCP servers, or the auto-created{" "}
          <code className="rounded bg-muted px-1 py-0.5 font-mono text-cool">weave_managed</code> connector every SDK-registered tool
          lands on.
        </p>
      </div>

      {error && <p className="text-sm text-destructive">{error}</p>}

      <Card className="glass border-0">
        <CardContent className="p-0">
          {connectors === null ? (
            <div className="flex justify-center py-10">
              <Loader2 className="size-5 animate-spin text-cool" />
            </div>
          ) : connectors.length === 0 ? (
            <p className="p-6 text-center text-sm text-muted-foreground">No connectors registered yet.</p>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Name</TableHead>
                  <TableHead>Transport</TableHead>
                  <TableHead>Endpoint</TableHead>
                  <TableHead>Status</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {connectors.map((c, i) => (
                  <motion.tr
                    key={c.Id}
                    data-slot="table-row"
                    className="border-b transition-colors hover:bg-muted/50"
                    initial={{ opacity: 0 }}
                    animate={{ opacity: 1 }}
                    transition={{ delay: i * 0.03 }}
                  >
                    <TableCell className="font-medium">{c.name}</TableCell>
                    <TableCell className="text-muted-foreground">{c.transport}</TableCell>
                    <TableCell className="max-w-xs truncate font-mono text-xs text-muted-foreground">
                      {c.endpoint}
                    </TableCell>
                    <TableCell>
                      <Badge
                        className={c.status === "active" ? "border-success/40 bg-success/12 text-success" : ""}
                        variant={c.status === "active" ? "outline" : "secondary"}
                      >
                        {c.status}
                      </Badge>
                    </TableCell>
                  </motion.tr>
                ))}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
