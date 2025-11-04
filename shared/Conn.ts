import { decode, encode } from "cbor-x";

// So that we can queue messages if the client is not ready
export class Conn<T, R> {
    ws: WebSocket | null = null;
    queue: (ArrayBuffer | Uint8Array)[] = [];
    closed: boolean = false;
    private url: string;
    private options: {
        onOpen?: () => void,
        onClose?: () => void,
        onError?: (event: Event) => void,
        onMessage?: (decoded: R) => void
    };
    private reconnectDelay: number = 1000; // Initial delay in milliseconds
    private maxReconnectDelay: number = 60000; // Max delay in milliseconds (60 seconds)
    private reconnectTimer: ReturnType<typeof setTimeout> | null = null;

    constructor(url: string, options: {
        onOpen?: () => void,
        onClose?: () => void,
        onError?: (event: Event) => void,
        onMessage?: (decoded: R) => void
    }) {
        this.url = url;
        this.options = options;
        this.connect();
    }



    private connect() {
        try {
            this.ws = new WebSocket(this.url);
            this.ws.binaryType = "arraybuffer";

            if (this.options.onOpen) {
                this.ws.addEventListener("open", this.options.onOpen);
            }

            if (this.options.onClose) {
                this.ws.addEventListener("close", this.options.onClose);
            }

            if (this.options.onError) {
                this.ws.addEventListener("error", this.options.onError);
            }

            const onMessage = this.options.onMessage;
            if (onMessage) {
                this.ws.addEventListener("message", (event) => {
                    const data = new Uint8Array(event.data);
                    const decoded = decode(data) as R;
                    onMessage(decoded);
                });
            }

            this.ws.addEventListener("open", () => {
                console.log(`[Conn] Connection successful to ${this.url}.`);
                // Reset reconnect delay on successful connection
                this.reconnectDelay = 1000;
                this.flush();
            });

            // on close, clear the queue and attempt to reconnect with exponential backoff
            this.ws.addEventListener("close", (event) => {
                console.log(`[Conn] Connection closed: ${event.code} ${event.reason}`);
                this.closed = true;

                // Attempt to reconnect with exponential backoff
                this.attemptReconnect();
            });
        } catch (error) {
            console.error(`[Conn] Error creating WebSocket connection:`, error);
            // Attempt to reconnect with exponential backoff
            this.attemptReconnect();
        }
    }

    private attemptReconnect() {
        console.log(`[Conn] Attempting to reconnect in ${this.reconnectDelay / 1000} seconds...`);

        if (this.reconnectTimer) {
            clearTimeout(this.reconnectTimer);
        }

        this.reconnectTimer = setTimeout(() => {
            if (!this.closed) { // Only reconnect if not explicitly closed
                this.connect();
            }
        }, this.reconnectDelay);

        // Exponential backoff: double the delay, but cap it at maxReconnectDelay
        // Add some random jitter to prevent thundering herd
        const jitter = Math.random() * 1000; // Add up to 1 second of random jitter
        this.reconnectDelay = Math.min(this.reconnectDelay * 2, this.maxReconnectDelay) + jitter;
    }

    public close() {
        this.closed = true;
        if (this.reconnectTimer) {
            clearTimeout(this.reconnectTimer);
            this.reconnectTimer = null;
        }
        if (this.ws) {
            this.ws.close();
            this.ws = null;
        }
        this.queue = [];
    }

    send(message: T) {
        if (this.closed) {
            console.warn("Attempted to send on closed connection");
            return;
        }
        const encoded = encode(message);
        if (this.ws && this.ws.readyState === WebSocket.OPEN) {
            this.ws.send(encoded);
        } else {
            this.queue.push(encoded);
        }
    }

    flush() {
        if (!this.ws || this.ws.readyState !== WebSocket.OPEN) {
            return; // Don't flush if connection is not ready
        }

        while (this.queue.length > 0) {
            const message = this.queue.shift();
            if (message) {
                this.ws.send(message);
            }
        }
    }
}
