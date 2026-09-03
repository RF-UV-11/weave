// grpc-web clients (docs/architecture/ARCHITECTURE.md §4): the browser
// never speaks raw gRPC, only grpc-web through Envoy
// (infra/envoy/envoy.yaml), which Connect-ES's grpcWebTransport speaks
// natively — no translation layer needed on our side.
import { createClient } from "@connectrpc/connect";
import { createGrpcWebTransport } from "@connectrpc/connect-web";

import { AuthService } from "@/gen/core/data_access/v1/auth_pb";
import { TenantService } from "@/gen/core/data_access/v1/tenant_pb";
import { BotProfileService } from "@/gen/core/data_access/v1/bot_profile_pb";
import { HttpToolService } from "@/gen/core/data_access/v1/http_tool_pb";
import { ConnectorService } from "@/gen/core/data_access/v1/connector_pb";
import { ChatService } from "@/gen/orchestrator/v1/chat_pb";

const ENVOY_URL = process.env.NEXT_PUBLIC_ENVOY_URL ?? "http://localhost:8090";

const transport = createGrpcWebTransport({ baseUrl: ENVOY_URL });

export const authClient = createClient(AuthService, transport);
export const tenantClient = createClient(TenantService, transport);
export const botProfileClient = createClient(BotProfileService, transport);
export const httpToolClient = createClient(HttpToolService, transport);
export const connectorClient = createClient(ConnectorService, transport);
export const chatClient = createClient(ChatService, transport);

/** Connect-ES call option carrying the bearer token, same shape every
 * authenticated core/orchestrator RPC expects (grpc metadata, not a
 * request field — never trust a client-supplied tenant_id either,
 * docs/architecture/SECURITY.md §2). */
export function authHeaders(token: string) {
  return { headers: { authorization: `Bearer ${token}` } };
}
