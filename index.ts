import { statSync } from "fs";
import { decode } from "cbor-x";
import { randomUUID } from "crypto";
import { RECORDINGS_DIR, RUNTIME_DIR } from "./backend/appdir";
import { table_media } from "./backend/database";
import { logger } from "./backend/logger";

import type { ServerWebSocket } from "bun";
import { WsClient } from "./backend/WsClient";
import { spawn_worker } from "./backend/worker_connect/shared";
import { start_stream, start_stream_file, start_streams, stop_stream } from "./backend/worker_connect/worker_stream_connector";
import homepage from "./index.html";
import type { ClientToServerMessage, RecordingsResponse } from "./shared";
import { createForwardFunction } from "./backend/forward";
import { MediaInput } from "node-av";


logger.info(`Using runtime directory: ${RUNTIME_DIR}`);
const server = Bun.serve({
    port: 3000,
    routes: {
        "/": homepage,
        "/test": async (req) => {
            return new Response("Test endpoint working");
        },
        '/media/:id': {
            PUT: async ({ params, body }: { params: { id: string }, body: any }) => {
                const { id } = params;
                const data = await new Response(body).json();
                const { name, uri, labels, saveToDisk, saveDir } = data;
                if (!name || !uri) {
                    return new Response('Missing name or uri', { status: 400 });
                }
                const updated_at = new Date().toISOString();
                await table_media.mergeInsert("id")
                    .whenMatchedUpdateAll()
                    .execute([{ id, name, uri, labels: labels ?? [], updated_at, saveToDisk: saveToDisk ?? false, saveDir: saveDir ?? '' }]);
                return Response.json({ success: true });
            },
            DELETE: async ({ params }: { params: { id: string } }) => {
                const { id } = params;
                await table_media.delete(`id = '${id}'`);
                return Response.json({ success: true });
            }
        },
        '/media': {
            GET: async () => {
                const media = await table_media.query().toArray();
                // @ts-ignore
                media.sort((a, b) => b.updated_at.localeCompare(a.updated_at));
                return Response.json(media);
            },
            POST: async (req: Request) => {
                const body = await req.json();
                const { name, uri, labels, saveToDisk, saveDir } = body;
                if (!name || !uri) {
                    return new Response('Missing name or uri', { status: 400 });
                }
                const id = randomUUID();
                const updated_at = new Date().toISOString();
                await table_media.add([{ id, name, uri, labels: labels ?? [], updated_at, saveToDisk: saveToDisk ?? false, saveDir: saveDir ?? '' }]);
                return Response.json({ success: true, id });
            },
        },
        '/recordings': {
            GET: async () => {
                const recordingsByStream: RecordingsResponse = {};
                const glob = new Bun.Glob("*/*.mkv");
                for await (const file of glob.scan(RECORDINGS_DIR)) {
                    const parts = file.split("/");
                    if (parts.length < 2) {
                        continue;
                    }
                    const streamId = parts[0]!;
                    // from_1762122447803_ms.mkv
                    const file_name = parts[1]!;

                    const from_ms = file_name.match(/from_(\d+)_ms\.mkv/)?.[1];
                    const to_ms = file_name.match(/_to_(\d+)_ms\.mkv/)?.[1];

                    const fromDate = from_ms ? new Date(parseInt(from_ms)) : null;
                    const toDate = to_ms ? new Date(parseInt(to_ms)) : null;

                    if (!recordingsByStream[streamId]) {
                        recordingsByStream[streamId] = [];
                    }

                    recordingsByStream[streamId].push({
                        file_name: file_name,
                        from_ms: fromDate?.getTime(),
                        to_ms: toDate?.getTime(),
                    });
                }
                return Response.json(recordingsByStream);
            }
        },
    },
    websocket: {
        open(ws) {
            logger.info("WebSocket connection opened");
            clients.set(ws, new WsClient(ws));
        },
        close(ws, code, reason) {
            logger.info(`WebSocket connection closed: ${code} - ${reason}`);
            const client = clients.get(ws);
            if (client) {
                // Mark the client as closed to prevent further processing
                // Just in case other functions are still referencing it
                client.destroy();
            }
            clients.delete(ws);
        },
        message(ws, message) {
            try {
                const decoded = decode(message as Buffer) as ClientToServerMessage;
                if (decoded.type === 'set_subscription') {
                    const client = clients.get(ws);
                    if (client) {
                        const oldFileStreams = client.subscription?.streams.filter(s => s.file_name) || [];

                        client.updateSubscription(decoded.subscription);
                        logger.info(`Client subscription updated for ${ws.remoteAddress}: ${JSON.stringify(client.subscription)}`);

                        const newFileStreams = decoded.subscription?.streams.filter(s => s.file_name) || [];
                        logger.info(`Client file subscriptions for ${ws.remoteAddress}: ${JSON.stringify(newFileStreams)}`);

                        const removedOldFileStreams = oldFileStreams.filter(oldStream =>
                            !newFileStreams.find(newStream => newStream.id === oldStream.id && newStream.file_name === oldStream.file_name)
                        );

                        for (const stream of removedOldFileStreams) {
                            logger.info(`Client unsubscribed from file stream ${stream.id} (file: ${stream.file_name})`);
                            // Notify the worker about the removed file stream
                            stop_stream({
                                worker: worker_stream,
                                stream_id: stream.id,
                                file_name: stream.file_name,
                            });
                        }

                        const addedNewFileStreams = newFileStreams.filter(newStream =>
                            !oldFileStreams.find(oldStream => oldStream.id === newStream.id && oldStream.file_name === newStream.file_name)
                        );

                        for (const stream of addedNewFileStreams) {
                            logger.info(`Client subscribed to file stream ${stream.id} (file: ${stream.file_name})`);
                            // Notify the worker about the added file stream
                            start_stream_file({
                                worker: worker_stream,
                                stream_id: stream.id,
                                file_name: stream.file_name!,
                            });
                        }

                    }
                }
            } catch (error) {
                logger.error(error, 'Error parsing websocket message');
            }
        },
    },

    async fetch(req, server) {
        const url = new URL(req.url);

        // WebSocket upgrade
        if (url.pathname === "/ws") {
            if (server.upgrade(req)) {
                return; // do not return a Response
            } else {
                return new Response("Cannot upgrade to WebSocket", { status: 400 });
            }
        }

        // API Proxying
        if (url.pathname.startsWith("/api/v1/")) {
            const targetUrl = new URL(req.url);
            targetUrl.host = "backend.zapdoslabs.com";
            targetUrl.protocol = "https:";

            const headers = new Headers(req.headers);
            // if (appConfig.store.auth_token) {
            //     headers.set("authorization", `Bearer ${appConfig.store.auth_token}`);
            // }

            try {
                const response = await fetch(targetUrl.toString(), {
                    method: req.method,
                    headers: headers,
                    body: req.body,
                    redirect: "manual",
                });
                return response;
            } catch (error) {
                logger.error(error, "Proxy error:");
                return new Response(JSON.stringify({ error: "Proxy error occurred" }), {
                    status: 500,
                    headers: { "Content-Type": "application/json" },
                });
            }
        }

        return new Response("Not found", { status: 404 });
    },

    development: process.env.NODE_ENV === "development",
});

logger.info("Server running on http://localhost:3000");



const clients = new Map<ServerWebSocket, WsClient>();
const forward = createForwardFunction({
    clients,
    worker_object_detection: () => worker_object_detection
})

const worker_stream = spawn_worker('worker_stream.js', forward);
const worker_object_detection = spawn_worker('worker_object_detection.js', forward);

// Start all streams from the database
start_streams({
    worker_stream,
});