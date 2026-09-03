"use client";

import { useEffect, useState } from "react";
import { create } from "@bufbuild/protobuf";
import { Loader2, BarChart3 } from "lucide-react";

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
        <h1 className="text-xl font-semibold tracking-tight">Tools</h1>
        <p className="text-sm text-muted-foreground">
          Every HTTP API this tenant has registered via the <code className="rounded bg-muted px-1 py-0.5">weave</code>{" "}
          SDK, and who can see each one.
        </p>
      </div>

      {error && <p className="text-sm text-destructive">{error}</p>}

      <Card>
        <CardContent className="p-0">
          {tools === null ? (
            <div className="flex justify-center py-10">
              <Loader2 className="size-5 animate-spin text-muted-foreground" />
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
                </TableRow>
              </TableHeader>
              <TableBody>
                {tools.map((t) => (
                  <TableRow key={t.Id}>
                    <TableCell className="font-medium">{t.name}</TableCell>
                    <TableCell className="max-w-xs truncate text-muted-foreground">{t.description}</TableCell>
                    <TableCell>
                      <Badge variant="outline" className="font-mono text-[11px]">
                        {t.httpMethod}
                      </Badge>
                    </TableCell>
                    <TableCell>
                      <Badge variant={t.visibility === "external" ? "default" : "secondary"}>{t.visibility}</Badge>
                    </TableCell>
                    <TableCell>
                      {t.category === "analytics" ? (
                        <span className="inline-flex items-center gap-1 text-sm">
                          <BarChart3 className="size-3.5 text-primary" />
                          analytics
                        </span>
                      ) : (
                        <span className="text-sm text-muted-foreground">general</span>
                      )}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
