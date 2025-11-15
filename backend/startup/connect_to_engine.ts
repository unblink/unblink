import type { ServerWebSocket } from "bun";

import type { WebhookMessage } from "~/shared/alert";
import { Conn } from "~/shared/Conn";
import { updateMediaUnit } from "../database/utils";
import { logger } from "../logger";
import type { WsClient } from "../WsClient";
import type { EngineToServer, ServerToEngine } from "~/shared/engine";

export function connect_to_engine(props: {
    ENGINE_URL: string,
    forward_to_webhook: (msg: WebhookMessage) => Promise<void>,
    clients: () => Map<ServerWebSocket, WsClient>,
}) {
    const engine_conn = new Conn<ServerToEngine, EngineToServer>(`wss://${props.ENGINE_URL}/ws`, {
        onOpen() {
            const msg: ServerToEngine = {
                type: "i_am_server",
            }
            engine_conn.send(msg);
        },
        onClose() {
            logger.info("Disconnected from Zapdos Labs engine WebSocket");
        },
        onError(event) {
            logger.error(event, "WebSocket to engine error:");
        },
        onMessage(decoded) {
            if (decoded.type === 'frame_description') {
                // Store in database
                // logger.info(`Received description for frame ${decoded.frame_id}: ${decoded.description}`);
                updateMediaUnit(decoded.frame_id, {
                    description: decoded.description,
                })

                // Forward to clients 
                for (const [id, client] of props.clients()) {
                    client.send(decoded, false);
                }

                // Also forward to webhook
                props.forward_to_webhook({
                    event: 'description',
                    data: {
                        created_at: new Date().toISOString(),
                        stream_id: decoded.stream_id,
                        frame_id: decoded.frame_id,
                        description: decoded.description,
                    }
                });
            }

            if (decoded.type === 'frame_embedding') {
                // Convert number[] to Uint8Array for database storage
                const embeddingBuffer = decoded.embedding ? new Uint8Array(new Float32Array(decoded.embedding).buffer) : null;

                // Store in database
                updateMediaUnit(decoded.frame_id, {
                    embedding: embeddingBuffer,
                })
            }

            if (decoded.type === 'frame_object_detection') {
                // // Also forward to webhook
                // props.forward_to_webhook({
                //     event: 'object_detection',
                //     data: {
                //         created_at: new Date().toISOString(),
                //         stream_id: decoded.stream_id,
                //         frame_id: decoded.frame_id,
                //         objects: decoded.objects,
                //     }
                // });

                // Forward to clients
                for (const [, client] of props.clients()) {
                    client.send(decoded);
                }
            }
        }
    });

    return engine_conn;
}