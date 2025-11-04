import { createSignal, untrack } from "solid-js";
import type { ClientToServerMessage, ServerToClientMessage } from "~/shared";
import { Conn } from "~/shared/Conn";
import { subscription } from "../shared";

export const [newMessage, setNewMessage] = createSignal<ServerToClientMessage>();


export const connectWebSocket = () => {
    const conn = new Conn<ClientToServerMessage, ServerToClientMessage>(`ws://${location.host}/ws`, {
        onOpen: () => console.log("WebSocket connection opened"),
        onClose: () => console.log("WebSocket connection closed"),
        onError: (e) => console.error("WebSocket connection error:", e),
        onMessage(decoded) {
            const sub = untrack(subscription)
            const session_id = sub?.session_id
            if (decoded.session_id !== session_id) {
                console.warn(`Received message for session_id ${decoded.session_id}, but current session_id is ${session_id}. Ignoring message.`, decoded);
                return;
            }
            // console.log("WebSocket message received", decoded);
            setNewMessage(decoded);
        }
    });
    return conn;
};

