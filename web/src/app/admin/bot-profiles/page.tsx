"use client";

import { useCallback, useEffect, useState } from "react";
import { create } from "@bufbuild/protobuf";
import { motion } from "framer-motion";
import { Loader2, Plus, ShieldCheck, ShieldOff, Globe, Sparkles } from "lucide-react";

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
import { Textarea } from "@/components/ui/textarea";
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

// "" (shown as "Default (Ollama)") is a real, distinct choice, not a
// placeholder — see BotProfile.llm_provider's proto comment.
const LLM_PROVIDER_OPTIONS: { value: string; label: string }[] = [
  { value: "", label: "Default (Ollama)" },
  { value: "ollama", label: "Ollama" },
  { value: "openai", label: "OpenAI-compatible" },
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
          <h1 className="font-heading text-xl font-semibold tracking-tight">Bot profiles</h1>
          <p className="text-sm text-muted-foreground">
            Who each bot is, which channels it answers on, and what it can see.
          </p>
        </div>
        <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
          <DialogTrigger render={<Button size="sm" className="glow-cool gap-1.5" />}>
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

      <Card className="glass border-0">
        <CardContent className="p-0">
          {profiles === null ? (
            <div className="flex justify-center py-10">
              <Loader2 className="size-5 animate-spin text-cool" />
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
                  <TableHead>Model</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {profiles.map((p, i) => (
                  <motion.tr
                    key={p.Id}
                    data-slot="table-row"
                    className="border-b transition-colors hover:bg-muted/50"
                    initial={{ opacity: 0 }}
                    animate={{ opacity: 1 }}
                    transition={{ delay: i * 0.03 }}
                  >
                    <TableCell className="font-medium">{p.name}</TableCell>
                    <TableCell>
                      <Badge
                        variant="outline"
                        className={
                          p.visibility === "external"
                            ? "border-warm/40 bg-warm/12 text-warm"
                            : "border-cool/40 bg-cool/12 text-cool"
                        }
                      >
                        {p.visibility}
                      </Badge>
                    </TableCell>
                    <TableCell className="text-muted-foreground">{p.channels.join(", ") || "—"}</TableCell>
                    <TableCell>
                      {p.guardrails.length > 0 ? (
                        <span className="inline-flex items-center gap-1 text-sm">
                          <ShieldCheck className="size-3.5 text-success" />
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
                          <Globe className="size-3.5 text-cool" />
                          on
                        </span>
                      ) : (
                        <span className="text-sm text-muted-foreground">off</span>
                      )}
                    </TableCell>
                    <TableCell>
                      <span className="font-mono text-xs text-muted-foreground">
                        {p.llmProvider || "ollama"}
                        {p.llmModel ? `:${p.llmModel}` : ""}
                      </span>
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
  const [persona, setPersona] = useState("");
  const [llmProvider, setLlmProvider] = useState("");
  const [llmModel, setLlmModel] = useState("");
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
          persona: persona.trim(),
          llmProvider,
          llmModel: llmModel.trim(),
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
    <DialogContent className="glass shadow-elevated border-0 sm:max-w-lg">
      <DialogHeader>
        <DialogTitle className="font-heading">New bot profile</DialogTitle>
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
                <SelectValue>
                  {(v: string) => (v === "external" ? "External (customer)" : "Internal (staff)")}
                </SelectValue>
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
                <SelectValue>
                  {(v: string) => ROLE_OPTIONS.find((r) => String(r.value) === v)?.label ?? v}
                </SelectValue>
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

        <div className="flex flex-col gap-1.5">
          <Label htmlFor="bp-persona" className="flex items-center gap-1.5">
            <Sparkles className="size-3.5 text-cool" />
            Persona <span className="font-normal text-muted-foreground">(optional)</span>
          </Label>
          <Textarea
            id="bp-persona"
            value={persona}
            onChange={(e) => setPersona(e.target.value)}
            placeholder="You are Acme's support assistant. Be concise, always cite the order ID you looked up…"
            rows={3}
            className="resize-none"
          />
          <p className="text-xs text-muted-foreground">
            This bot&apos;s system prompt, verbatim. Left blank, it falls back to a generic default.
          </p>
        </div>

        <div className="grid grid-cols-2 gap-4">
          <div className="flex flex-col gap-1.5">
            <Label>Model provider</Label>
            <Select value={llmProvider} onValueChange={(v) => setLlmProvider(v ?? "")}>
              <SelectTrigger>
                <SelectValue>
                  {(v: string) => LLM_PROVIDER_OPTIONS.find((o) => o.value === v)?.label ?? "Default (Ollama)"}
                </SelectValue>
              </SelectTrigger>
              <SelectContent>
                {LLM_PROVIDER_OPTIONS.map((o) => (
                  <SelectItem key={o.value || "default"} value={o.value}>
                    {o.label}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="bp-model">Model name (optional)</Label>
            <Input
              id="bp-model"
              value={llmModel}
              onChange={(e) => setLlmModel(e.target.value)}
              placeholder="llama3.2:3b"
              className="font-mono text-sm"
            />
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
        <Button onClick={onSubmit} disabled={submitting || !name.trim()} className="glow-cool gap-2">
          {submitting && <Loader2 className="size-4 animate-spin" />}
          Create
        </Button>
      </DialogFooter>
    </DialogContent>
  );
}
