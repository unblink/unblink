import type { ServerWebSocket } from "bun";
import { encode } from "cbor-x";
import type { ServerToClientMessage, Subscription } from "~/shared";

export class WsClient {
    private _subscription: Subscription | null | undefined;
    private destroyed: boolean = false;
    constructor(
        public ws: ServerWebSocket,
    ) {

    }

    updateSubscription(subscription: Subscription | null | undefined) {
        this._subscription = subscription;
    }

    get subscription() {
        return this.destroyed ? null : this._subscription;
    }

    destroy() {
        this.destroyed = true;
    }

    send(
        msg: ServerToClientMessage,

    ) {
        if (this.destroyed) return;

        if (!this._subscription) return;

        // Check matches subscription before sending

        if (msg.type === 'codec' || msg.type === 'frame') {
            const is_subscribed = this._subscription.streams.some(s => {
                return s.id === msg.stream_id && s.file_name === msg.file_name;
            });
            if (!is_subscribed) {
                // Not subscribed to this stream
                return;
            }
        }

        const encoded = encode({
            session_id: this._subscription.session_id,
            ...msg,
        })

        this.ws.send(encoded);
    }
}
