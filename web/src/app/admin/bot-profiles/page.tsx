"use client";

import { useCallback, useEffect, useState } from "react";
import { create } from "@bufbuild/protobuf";
import { Loader2, Plus, ShieldCheck, ShieldOff, Globe } from "lucide-react";

import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent } from "@/components/ui/card";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Switch } from "@/components/ui/switch";
import { useAuth, friendlyError } from "@/lib/auth-context";
import { botProfileClient, authHeaders } from "@/lib/connect";
import { ListBotProfilesRequestSchema, CreateBotProfileRequestSchema } from "@/gen/core/data_access/v1/bot_profile_pb";
import type { BotProfile } from "@/gen/database/v1/bot_profile_pb";
import { Role } from "@/gen/database/v1/auth_pb";

const ROLE_OPTIONS: { value: Role; label: string }[] = [
  { value: Role.OWNER, label: "Owner" },
  { value: Role.ADMIN, label: "Admin" },
  { value: Role.STAFF, label: "Staff" },
  { value: Role.CUSTOMER, label: "Customer" },
];

export default function BotProfilesPage() {
  const { session } = useAuth();
  const [profiles, setProfiles] = useState<BotProfile[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [dialogOpen, setDialogOpen] = useState(false);

  const load = useCallback(async () => {
    if (!session) return;
    try {
      const resp = await botProfileClient.listBotProfiles(
        create(ListBotProfilesRequestSchema, { tenantId: session.tenantId }),
        authHeaders(session.token)
      );
      setProfiles(resp.botProfiles);
    } catch (err) {
      setError(friendlyError(err));
    }
  }, [session]);

  useEffect(() => {
    // Initial fetch-on-mount; load() is reused after create too (see
    // CreateBotProfileDialog's onCreated below), which is why it's a
    // separate memoized function rather than inlined here.
    // eslint-disable-next-line react-hooks/set-state-in-effect
    load();
  }, [load]);

  if (!session) return null;

  return (
    <div className="flex flex-col gap-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-xl font-semibold tracking-tight">Bot profiles</h1>
          <p className="text-sm text-muted-foreground">
            Who each bot is, which channels it answers on, and what it can see.
          </p>
        </div>
        <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
          <DialogTrigger render={<Button size="sm" className="gap-1.5" />}>
            <Plus className="size-4" />
            New bot profile
          </DialogTrigger>
          <CreateBotProfileDialog
            token={session.token}
            tenantId={session.tenantId}
            onCreated={() => {
              setDialogOpen(false);
              load();
            }}
          />
        </Dialog>
      </div>

      {error && <p className="text-sm text-destructive">{error}</p>}

      <Card>
        <CardContent className="p-0">
          {profiles === null ? (
            <div className="flex justify-center py-10">
              <Loader2 className="size-5 animate-spin text-muted-foreground" />
            </div>
          ) : profiles.length === 0 ? (
            <p className="p-6 text-center text-sm text-muted-foreground">No bot profiles yet.</p>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Name</TableHead>
                  <TableHead>Visibility</TableHead>
                  <TableHead>Channels</TableHead>
                  <TableHead>Guardrails</TableHead>
                  <TableHead>Web search</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {profiles.map((p) => (
                  <TableRow key={p.Id}>
                    <TableCell className="font-medium">{p.name}</TableCell>
                    <TableCell>
                      <Badge variant={p.visibility === "external" ? "default" : "secondary"}>{p.visibility}</Badge>
                    </TableCell>
                    <TableCell className="text-muted-foreground">{p.channels.join(", ") || "—"}</TableCell>
                    <TableCell>
                      {p.guardrails.length > 0 ? (
                        <span className="inline-flex items-center gap-1 text-sm">
                          <ShieldCheck className="size-3.5 text-primary" />
                          {p.guardrails.length}
                        </span>
                      ) : (
                        <span className="inline-flex items-center gap-1 text-sm text-muted-foreground">
                          <ShieldOff className="size-3.5" />
                          none
                        </span>
                      )}
                    </TableCell>
                    <TableCell>
                      {p.webSearchEnabled ? (
                        <span className="inline-flex items-center gap-1 text-sm">
                          <Globe className="size-3.5 text-primary" />
                          on
                        </span>
                      ) : (
                        <span className="text-sm text-muted-foreground">off</span>
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

function CreateBotProfileDialog({
  token,
  tenantId,
  onCreated,
}: {
  token: string;
  tenantId: string;
  onCreated: () => void;
}) {
  const [name, setName] = useState("");
  const [channelsInput, setChannelsInput] = useState("web-widget");
  const [visibility, setVisibility] = useState<"internal" | "external">("internal");
  const [role, setRole] = useState<Role>(Role.STAFF);
  const [webSearchEnabled, setWebSearchEnabled] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function onSubmit() {
    setError(null);
    setSubmitting(true);
    try {
      await botProfileClient.createBotProfile(
        create(CreateBotProfileRequestSchema, {
          tenantId,
          name: name.trim(),
          channels: channelsInput.split(",").map((c) => c.trim()).filter(Boolean),
          rolesAllowed: [role],
          visibility,
          webSearchEnabled,
        }),
        authHeaders(token)
      );
      onCreated();
    } catch (err) {
      setError(friendlyError(err));
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <DialogContent>
      <DialogHeader>
        <DialogTitle>New bot profile</DialogTitle>
        <DialogDescription>
          A bot profile controls which channels a bot answers on, which role can use it, and whether it&apos;s
          internal (staff, sees everything) or external (customer-facing, restricted).
        </DialogDescription>
      </DialogHeader>

      <div className="flex flex-col gap-4">
        <div className="flex flex-col gap-1.5">
          <Label htmlFor="bp-name">Name</Label>
          <Input id="bp-name" value={name} onChange={(e) => setName(e.target.value)} placeholder="external" />
        </div>
        <div className="flex flex-col gap-1.5">
          <Label htmlFor="bp-channels">Channels (comma-separated)</Label>
          <Input id="bp-channels" value={channelsInput} onChange={(e) => setChannelsInput(e.target.value)} />
        </div>
        <div className="grid grid-cols-2 gap-4">
          <div className="flex flex-col gap-1.5">
            <Label>Visibility</Label>
            <Select value={visibility} onValueChange={(v) => v && setVisibility(v as "internal" | "external")}>
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="internal">Internal (staff)</SelectItem>
                <SelectItem value="external">External (customer)</SelectItem>
              </SelectContent>
            </Select>
          </div>
          <div className="flex flex-col gap-1.5">
            <Label>Role allowed</Label>
            <Select value={String(role)} onValueChange={(v) => v && setRole(Number(v) as Role)}>
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {ROLE_OPTIONS.map((r) => (
                  <SelectItem key={r.value} value={String(r.value)}>
                    {r.label}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
        </div>
        <div className="flex items-center justify-between rounded-lg border px-3 py-2.5">
          <div>
            <p className="text-sm font-medium">Web search</p>
            <p className="text-xs text-muted-foreground">Let this bot fall back to public web search.</p>
          </div>
          <Switch checked={webSearchEnabled} onCheckedChange={setWebSearchEnabled} />
        </div>
        {error && <p className="text-sm text-destructive">{error}</p>}
      </div>

      <DialogFooter>
        <Button onClick={onSubmit} disabled={submitting || !name.trim()} className="gap-2">
          {submitting && <Loader2 className="size-4 animate-spin" />}
          Create
        </Button>
      </DialogFooter>
    </DialogContent>
  );
}
