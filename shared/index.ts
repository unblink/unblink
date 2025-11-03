export type FrameMessage = {
    type: "frame_file";
    path: string;
}

export type StreamMessage = {
    type: "codec";
    mimeType: string | null;
    videoCodec: string | null;
    audioCodec: string | null;
    codecString: string | null;
    fullCodec: string;
    width: number;
    height: number;
    hasAudio: boolean;
} | {
    type: 'frame';
    data: Uint8Array;
} | FrameMessage;

export type MediaUnit = {
    id: string;
    description: string | null;
    at_time: Date;
    embedding: number[] | null;
    media_id: string;
    path: string;
}

export type Subscription = {
    session_id: string;
    streams: {
        id: string;
        file_name?: string;
    }[];
}

export type ClientToServerMessage = {
    type: 'set_subscription';
    subscription: Subscription | undefined | null;
}

export type WorkerToServerMessage = WorkerObjectDetectionToServerMessage | WorkerStreamToServerMessage
export type ServerToClientMessage = WorkerToServerMessage & {
    session_id?: string;
}

export type WorkerStreamToServerMessage = (StreamMessage & { stream_id: string, file_name?: string }) | {
    type: "error";
    stream_id: string;
} | {
    type: "restarting";
    stream_id: string;
} | {
    type: 'starting';
    stream_id: string;
}

export type ServerToWorkerStreamMessage_Add_Stream = {
    type: 'start_stream',
    stream_id: string,
    uri: string,
}
export type ServerToWorkerStreamMessage_Add_File = {
    type: 'start_stream_file',
    stream_id: string,
    file_name: string,
}
export type ServerToWorkerStreamMessage = ServerToWorkerStreamMessage_Add_Stream | ServerToWorkerStreamMessage_Add_File | {
    type: 'stop_stream',
    stream_id: string,
    file_name?: string,
}

export type ServerToWorkerObjectDetectionMessage = {
    stream_id: string;
} & FrameMessage

export type DetectionObject = {
    label: string;
    confidence: number;
    box: {
        x1: number;
        y1: number;
        x2: number;
        y2: number;
    }
}

export type WorkerObjectDetectionToServerMessage = {
    type: 'object_detection';
    stream_id: string;
    objects: DetectionObject[];
}

export type RecordingsResponse = Record<string, {
    file_name: string;
    from_ms?: number;
    to_ms?: number;
}[]>;