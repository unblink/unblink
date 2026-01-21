import { createSignal } from "solid-js";
import { configClient, nodeClient, serviceClient, agentClient } from "./lib/rpc";
import { Code } from "@connectrpc/connect";
import { ClientRealtimeEvent, AgentEvent, Agent } from "@/gen/unblink/agent/v1/agent_pb";
import { GetFlagsResponse } from "../gen/unblink/config/v1/config_pb";
import { Node } from "@/gen/unblink/node/v1/node_pb";
import { Service } from "@/gen/unblink/service/v1/service_pb";

const BASE_URL = import.meta.env.VITE_RELAY_API_URL;
if (!BASE_URL) throw new Error("VITE_RELAY_API_URL is not configured");
export const relay = (path: string) => new URL(path, BASE_URL).href

// Config state
export const [configState, setConfigState] = createSignal<GetFlagsResponse | null>(null);
export const [configLoading, setConfigLoading] = createSignal(true);

// Fetch flags from relay using RPC
export const fetchFlags = async () => {
  setConfigLoading(true);
  try {
    const response = await configClient.getFlags({});
    setConfigState(response);
    console.log('[fetchFlags] Config loaded:', response.devImpersonateEmail);
  } catch (error) {
    console.error('[fetchFlags] Failed to fetch flags:', error);
    setConfigState(null);
  } finally {
    setConfigLoading(false);
  }
};

export type SimpleTabType = 'home' | 'chat' | 'moments' | 'agents' | 'settings';

export type Tab =
  | { type: SimpleTabType }
  | {
    type: 'view';
    nodeId: string;
    serviceId: string;
    name?: string;
  };

export const [tab, setTab] = createSignal<Tab>({ type: 'home' });

// Node services state
export const [nodes, setNodes] = createSignal<Node[]>([]);
export const [nodesLoading, setNodesLoading] = createSignal(true);

export const [services, setServices] = createSignal<Service[]>([]);
export const [servicesLoading, setServicesLoading] = createSignal(true);

// Agent state
export const [agents, setAgents] = createSignal<Agent[]>([]);
export const [agentsLoading, setAgentsLoading] = createSignal(true);

export const fetchAgents = async () => {
  setAgentsLoading(true);
  try {
    const response = await agentClient.listAgents({});
    setAgents(response.agents);
  } catch (error) {
    console.error('Error fetching agents:', error);
    // Check if it's an unauthenticated error
    if (error && typeof error === 'object' && 'code' in error && (error as any).code === 'unauthenticated') {
      window.location.href = '/login';
    }
    setAgents([]);
  } finally {
    setAgentsLoading(false);
  }
};

export const fetchServices = async () => {
  setServicesLoading(true);
  try {
    const response = await serviceClient.listServices({});
    setServices(response.services);
  } catch (error) {
    console.error('Error fetching services:', error);
    // Check if it's an unauthenticated error
    if (error && typeof error === 'object' && 'code' in error && (error as any).code === 'unauthenticated') {
      window.location.href = '/login';
    }
    setServices([]);
  } finally {
    setServicesLoading(false);
  }
};

export const fetchNodes = async () => {
  setNodesLoading(true);
  try {
    const response = await nodeClient.listNodes({});
    setNodes(response.nodes);
  } catch (error) {
    console.error('Error fetching nodes:', error);
    // Check if it's an unauthenticated error
    if (error && typeof error === 'object' && 'code' in error && (error as any).code === 'unauthenticated') {
      window.location.href = '/login';
    }
    setNodes([]);
  } finally {
    setNodesLoading(false);
  }
};

export const fetchNodesAndServices = async () => {
  await Promise.all([fetchNodes(), fetchServices()]);
};

// === Client Realtime Stream (replaces WebSocket) ===

// Event types matching the oneof in protobuf

let realtimeStreamClose: (() => void) | null = null;

// Newest realtime message - signal-based pattern with proper type
export const [newestRTEvent, setNewestRTEvent] = createSignal<ClientRealtimeEvent | null>(null);

export const connectClientRealtimeStream = async () => {
  console.log('[ClientRealtime] Connecting stream...');

  const abortController = new AbortController();

  // Stream in background, don't await
  (async () => {
    try {
      const stream = agentClient.streamClientRealtimeEvents(
        {},  // empty request
        { signal: abortController.signal }
      );

      for await (const response of stream) {
        if (response.event) {
          setNewestRTEvent(response.event);
          console.log('[ClientRealtime] New event received:', response.event);
        }
      }
    } catch (error) {
      if ((error as Error).name !== 'AbortError') {
        console.error('[ClientRealtime] Stream error:', error);
        // Auto-reconnect after delay
        setTimeout(connectClientRealtimeStream, 3000);
      }
    }
  })();

  realtimeStreamClose = () => abortController.abort();
  console.log('[ClientRealtime] Stream connecting...');
};

export const disconnectClientRealtimeStream = () => {
  if (realtimeStreamClose) {
    realtimeStreamClose();
    realtimeStreamClose = null;
  }
  setNewestRTEvent(null);
};

// Historical events now use separate unary RPC

