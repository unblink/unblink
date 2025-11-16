import path from 'path';
import { WORKER_DIR } from "~/definition";
import { logger } from '../logger';
import fs from 'fs/promises';

async function fileExists(filePath: string): Promise<boolean> {
    const res = await fs.stat(filePath).catch(() => null);
    return res !== null;
}

export async function spawn_worker(filename: string, onWorkerMessage: (event: MessageEvent) => void) {
    let workerPath = path.join(WORKER_DIR, filename);

    // Check if the worker file exists
    try {
        // Get the same name, ending with .js. Do not use replace
        const withoutExt = path.parse(workerPath).name;
        const jsWorkerPath = path.join(path.dirname(workerPath), withoutExt + '.js');
        const tsWorkerPath = path.join(path.dirname(workerPath), withoutExt + '.ts');
        let whicheverExists = null;
        if (await fileExists(jsWorkerPath)) {
            whicheverExists = jsWorkerPath;
        } else if (await fileExists(tsWorkerPath)) {
            whicheverExists = tsWorkerPath;
        }
        if (whicheverExists) {
            logger.info(`Using worker file: ${whicheverExists}`);
            workerPath = whicheverExists;
        } else {
            throw new Error(`Worker files ${jsWorkerPath} and ${tsWorkerPath} do not exist.`);
        }
    } catch (error) {
        logger.error(error);
        const files = await fs.readdir(path.dirname(workerPath));
        logger.info({ files }, `Files in worker directory:`);
        throw new Error(`Worker file ${workerPath} does not exist.`);
    }

    const worker = new Worker(workerPath);

    worker.addEventListener("message", onWorkerMessage);
    worker.addEventListener("error", (error) => {
        logger.info({ error }, `Error in worker ${filename}:`);
    });
    worker.addEventListener("exit", (event) => {
        logger.info({ event }, `Worker ${filename} exited`);
    });
    worker.addEventListener("close", (event) => {
        logger.info({ event }, `Worker ${filename} closed`);
    });
    worker.addEventListener("messageerror", (error) => {
        logger.info({ error }, `Message error in worker ${filename}:`);
    });

    // Prevents the worker from keeping the Node.js event loop active
    (worker as any).unref();

    return worker;
}
