import { encode } from "cbor-x";
import { streamMedia, type StartStreamArg } from "../stream/index";
import { logger } from "../logger";
import fs from "fs/promises";
import type { WorkerStreamToServerMessage, ServerToWorkerStreamMessage } from "../../shared";
import { RECORDINGS_DIR } from "../appdir";
import path from "path";
declare var self: Worker;

logger.info("Worker 'stream' started");

const loops: {
    [loop_id: string]: {
        controller: AbortController;
    }
} = {};

function sendMessage(msg: WorkerStreamToServerMessage) {
    const worker_msg = encode(msg);
    self.postMessage(worker_msg, [worker_msg.buffer]);
}

async function startStream(stream: StartStreamArg, signal: AbortSignal) {
    logger.info(`Starting media stream for ${stream.id}`);

    await streamMedia(stream, (msg) => {
        const worker_msg: WorkerStreamToServerMessage = {
            ...msg,
            stream_id: stream.id,
            file_name: stream.file_name,
        }

        sendMessage(worker_msg);
    }, signal);
}

async function startFaultTolerantStream(stream: StartStreamArg, signal: AbortSignal) {
    const state = {
        hearts: 5,
    }
    let recovery_timeout: NodeJS.Timeout | null = null;
    while (true) {
        try {
            recovery_timeout = setTimeout(() => {
                logger.info(`Stream ${stream.id} has been stable for 30 seconds, full recovery.`);
                state.hearts = 5;
            }, 30000);
            await startStream(stream, signal);
        } catch (e) {
            if (recovery_timeout) clearTimeout(recovery_timeout);
            state.hearts -= 1;
            if (state.hearts <= 0) {
                logger.error(e, `Stream for ${stream.id} has failed too many times, giving up.`);
                return;
            }
            logger.error(e, `Error in streaming loop for ${stream.id}, restarting (${state.hearts} hearts remaining)...`);
            if (signal.aborted) {
                logger.info(`Abort signal received, stopping stream for ${stream.id}`);
                return;
            }
            await new Promise((resolve) => setTimeout(resolve, 5000));
        }
    }
}

self.addEventListener("message", async (event) => {
    const msg: ServerToWorkerStreamMessage = event.data;
    if (msg.type === 'start_stream') {
        logger.info(`Starting stream ${msg.stream_id} with URI ${msg.uri}`);

        if (msg.uri) {
            const abortController = new AbortController();
            const loop_id = msg.stream_id;
            loops[loop_id] = {
                controller: abortController,
            };

            startFaultTolerantStream({
                id: msg.stream_id,
                uri: msg.uri,
                write_to_file: true, // TODO: make configurable
            }, abortController.signal);
        }
    }

    if (msg.type === 'start_stream_file') {

        logger.info(`Starting file stream ${msg.stream_id} for file ${msg.file_name}`);
        const abortController = new AbortController();
        const loop_id = `${msg.stream_id}::${msg.file_name}`;
        loops[loop_id] = {
            controller: abortController,
        };
        const dir = `${RECORDINGS_DIR}/${msg.stream_id}`;
        const uri = path.join(dir, msg.file_name);
        try {
            await startStream({
                id: msg.stream_id,
                uri,
                file_name: msg.file_name,
                write_to_file: false,
            }, abortController.signal);
        } catch (error) {
            logger.error(error, `Error starting file stream for ${msg.stream_id} file ${msg.file_name}`);
        }
    }

    if (msg.type === 'stop_stream') {
        logger.info(`Stopping stream ${msg.stream_id}`);
        // Stop the stream and clean up resources
        const loop_id = msg.file_name ? `${msg.stream_id}::${msg.file_name}` : msg.stream_id;
        loops[loop_id]?.controller.abort();
    }
});
