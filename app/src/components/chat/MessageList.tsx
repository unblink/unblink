import { For } from "solid-js";
import { isLoading, messages } from "../../signals/chatSignals";
import MessageItem from "./MessageItem";
import { useScroll } from "../../hooks/useScroll";

export default function MessageList() {
  let scrollContainerRef: HTMLDivElement | undefined;
  useScroll(() => scrollContainerRef);

  return (
    <div
      ref={(el) => scrollContainerRef = el}
      class="flex-1 overflow-y-auto p-4 space-y-4"
      style={{ "overflow-anchor": "none" }}
    >
      <For each={messages()}>
        {(message, index) => (
          <MessageItem
            message={message}
            showLoading={isLoading() && index() === messages().length - 1 && message.type === "model"}
            isLast={index() === messages().length - 1 && message.type === "model"}
          />
        )}
      </For>
      {isLoading() && (messages().length === 0 || messages()[messages().length - 1]?.type !== "model") && (
        <div class="w-full max-w-4xl mx-auto px-2 py-4">
          <div class="flex gap-1">
            <div class="w-2 h-2 bg-neu-500 rounded-full animate-pulse"></div>
            <div class="w-2 h-2 bg-neu-500 rounded-full animate-pulse" style="animation-delay: 0.2s"></div>
            <div class="w-2 h-2 bg-neu-500 rounded-full animate-pulse" style="animation-delay: 0.4s"></div>
          </div>
        </div>
      )}
    </div>
  );
}
