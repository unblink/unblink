import type { ServerToWorkerStreamMessage, ServerToWorkerStreamMessage_Add_File, ServerToWorkerStreamMessage_Add_Stream } from "~/shared";
import { table_media } from "../database";
import { logger } from "../logger";

export async function start_streams(opts: {
    worker_stream: Worker
}) {
    await new Promise(resolve => setTimeout(resolve, 5000)); // stagger starts
    try {
        const allMedia = await table_media.query().toArray();
        for (const media of allMedia) {
            await new Promise(resolve => setTimeout(resolve, 1000)); // stagger starts
            if (media.id && media.uri) {
                logger.info({ media }, `Starting stream:`);
                start_stream({
                    worker: opts.worker_stream,
                    stream_id: media.id as string,
                    uri: media.uri as string,
                    saveToDisk: media.saveToDisk as boolean,
                    saveDir: media.saveDir as string,
                });
            }
        }
    } catch (error) {
        logger.error(error, "Error starting streams from database");
    }
}

export function start_stream(opts: Omit<ServerToWorkerStreamMessage_Add_Stream, 'type'> & { worker: Worker, saveToDisk: boolean, saveDir: string }) {
    const start_msg: ServerToWorkerStreamMessage = {
        type: 'start_stream',
        stream_id: opts.stream_id,
        uri: opts.uri,
        saveToDisk: opts.saveToDisk,
        saveDir: opts.saveDir,
    }

    opts.worker.postMessage(start_msg);
}

export function start_stream_file(opts: Omit<ServerToWorkerStreamMessage_Add_File, 'type'> & { worker: Worker }) {
    const start_msg: ServerToWorkerStreamMessage = {
        type: 'start_stream_file',
        stream_id: opts.stream_id,
        file_name: opts.file_name,
    }

    opts.worker.postMessage(start_msg);
}

export function stop_stream(opts: {
    worker: Worker,
    stream_id: string,
    file_name?: string,
}) {
    const stop_msg: ServerToWorkerStreamMessage = {
        type: 'stop_stream',
        stream_id: opts.stream_id,
        file_name: opts.file_name,
    }

    opts.worker.postMessage(stop_msg);
}
