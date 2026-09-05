"use client";

import { useEffect, useState } from "react";
import { create } from "@bufbuild/protobuf";
import { motion } from "framer-motion";
import { Loader2, BarChart3, ShieldCheck } from "lucide-react";

import { Badge } from "@/components/ui/badge";
import { Card, CardContent } from "@/components/ui/card";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { useAuth, friendlyError } from "@/lib/auth-context";
import { httpToolClient, authHeaders } from "@/lib/connect";
import { ListHttpToolsRequestSchema } from "@/gen/core/data_access/v1/http_tool_pb";
import type { HttpTool } from "@/gen/database/v1/http_tool_pb";

export default function ToolsPage() {
  const { session } = useAuth();
  const [tools, setTools] = useState<HttpTool[] | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!session) return;
    httpToolClient
      .listHttpTools(create(ListHttpToolsRequestSchema, { tenantId: session.tenantId }), authHeaders(session.token))
      .then((resp) => setTools(resp.httpTools))
      .catch((err) => setError(friendlyError(err)));
  }, [session]);

  if (!session) return null;

  return (
    <div className="flex flex-col gap-6">
      <div>
        <h1 className="font-heading text-xl font-semibold tracking-tight">Tools</h1>
        <p className="text-sm text-muted-foreground">
          Every HTTP API this tenant has registered via the <code className="rounded bg-muted px-1 py-0.5 font-mono text-cool">weave</code>{" "}
          SDK, and who can see each one.
        </p>
      </div>

      {error && <p className="text-sm text-destructive">{error}</p>}

      <Card className="glass border-0">
        <CardContent className="p-0">
          {tools === null ? (
            <div className="flex justify-center py-10">
              <Loader2 className="size-5 animate-spin text-cool" />
            </div>
          ) : tools.length === 0 ? (
            <p className="p-6 text-center text-sm text-muted-foreground">
              No tools registered yet — use the weave SDK&apos;s <code>add_tool()</code> to add one.
            </p>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Name</TableHead>
                  <TableHead>Description</TableHead>
                  <TableHead>Method</TableHead>
                  <TableHead>Visibility</TableHead>
                  <TableHead>Category</TableHead>
                  <TableHead>Auth</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {tools.map((t, i) => (
                  <motion.tr
                    key={t.Id}
                    data-slot="table-row"
                    className="border-b transition-colors hover:bg-muted/50"
                    initial={{ opacity: 0 }}
                    animate={{ opacity: 1 }}
                    transition={{ delay: i * 0.03 }}
                  >
                    <TableCell className="font-medium">{t.name}</TableCell>
                    <TableCell className="max-w-xs truncate text-muted-foreground">{t.description}</TableCell>
                    <TableCell>
                      <Badge variant="outline" className="font-mono text-[11px]">
                        {t.httpMethod}
                      </Badge>
                    </TableCell>
                    <TableCell>
                      <Badge
                        variant="outline"
                        className={
                          t.visibility === "external"
                            ? "border-warm/40 bg-warm/12 text-warm"
                            : "border-cool/40 bg-cool/12 text-cool"
                        }
                      >
                        {t.visibility}
                      </Badge>
                    </TableCell>
                    <TableCell>
                      {t.category === "analytics" ? (
                        <span className="inline-flex items-center gap-1 text-sm">
                          <BarChart3 className="size-3.5 text-cool" />
                          analytics
                        </span>
                      ) : (
                        <span className="text-sm text-muted-foreground">general</span>
                      )}
                    </TableCell>
                    <TableCell>
                      {t.authMode === "user_token" ? (
                        <span className="inline-flex items-center gap-1 text-sm text-success">
                          <ShieldCheck className="size-3.5" />
                          per-user
                        </span>
                      ) : (
                        <span className="text-sm text-muted-foreground">shared</span>
                      )}
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
