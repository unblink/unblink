import { createClient } from "@connectrpc/connect";
import { createConnectTransport } from "@connectrpc/connect-web";
import { AuthService } from "../../gen/unblink/auth/v1/auth_pb";
import { NodeService } from "../../gen/unblink/node/v1/node_pb";
import { ServiceService } from "../../gen/unblink/service/v1/service_pb";
import { AgentService } from "../../gen/unblink/agent/v1/agent_pb";
import { WebRTCService } from "../../gen/unblink/webrtc/v1/webrtc_pb";
import { ConfigService } from "../../gen/unblink/config/v1/config_pb";
import { ChatService } from "../../gen/unblink/chat/v1/chat_pb";
import { StorageService } from "../../gen/unblink/storage/v1/storage_pb";


const BASE_URL = import.meta.env.VITE_RELAY_API_URL;

// Helper to get auth token
function getToken() {
    return localStorage.getItem('auth_token');
}

// Transport with auth interceptor
const authInterceptor = (next: any) => async (req: any) => {
    const token = getToken();
    if (token) {
        req.header.set('Authorization', `Bearer ${token}`);
    }
    return next(req);
};

// Transport with auth interceptor
const transport = createConnectTransport({
    baseUrl: BASE_URL,
    interceptors: [authInterceptor],
});

// Transport without auth interceptor (for public endpoints like GetFlags)
const publicTransport = createConnectTransport({
    baseUrl: BASE_URL,
});

// Export typed clients
export const authClient = createClient(AuthService, transport);
export const nodeClient = createClient(NodeService, transport);
export const serviceClient = createClient(ServiceService, transport);
export const agentClient = createClient(AgentService, transport);
export const webrtcClient = createClient(WebRTCService, transport);
export const configClient = createClient(ConfigService, publicTransport);
export const chatClient = createClient(ChatService, transport);
export const storageClient = createClient(StorageService, transport);
