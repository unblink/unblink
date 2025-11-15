
export type ServerToEngine =
    {
        type: "i_am_server";
        token?: string;
    } | {
        type: "frame_binary";
        workers: Partial<{
            'vlm': true,
            'object_detection': true,
            'embedding': true,
        }>
        frame_id: string;
        stream_id: string;
        frame: Uint8Array;
    }


export type DetectionObject = {
    label: string;
    confidence: number;
    box: {
        x_min: number;
        y_min: number;
        x_max: number;
        y_max: number;
    }
}

export type EngineToServer = {
    type: "frame_description";
    frame_id: string;
    stream_id: string;
    description: string;
} | {
    type: "frame_embedding";
    frame_id: string;
    stream_id: string;
    embedding: number[];
} | {
    type: "frame_object_detection";
    stream_id: string;
    frame_id: string;
    objects: DetectionObject[];
}

export type EngineReceivedMessage = ServerToEngine | WorkerToEngine;