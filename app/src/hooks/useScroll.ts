import { createEffect, on } from "solid-js";
import { chatInputState, activeConversationId, messages } from "../signals/chatSignals";

export function useScroll(scrollContainerRef: () => HTMLDivElement | undefined) {
  // Scroll to bottom instantly ONLY when conversation changes (not on every message)
  createEffect(on(activeConversationId, (convId) => {
    const messageList = messages();
    const container = scrollContainerRef();

    if (container && messageList.length > 0 && convId) {
      // Use requestAnimationFrame to ensure DOM is updated
      requestAnimationFrame(() => {
        container.scrollTo({
          top: container.scrollHeight,
          behavior: "instant"
        });
      });
    }
  }));

  createEffect(() => {
    const cnt = scrollContainerRef();
    if (!cnt) return;
    const chatInputStateValue = chatInputState();
    if (chatInputStateValue === 'idle') return;

    if (chatInputStateValue === 'user_sent') {
      setTimeout(() => {
        const container = scrollContainerRef();
        if (container) {
          container.scrollTo({
            top: container.scrollHeight,
            behavior: "smooth"
          });
        }
      }, 100);
      return;
    }

    if (chatInputStateValue === 'first_chunk_arrived') {
      setTimeout(() => {
        const container = scrollContainerRef();
        if (container) {
          container.scrollBy({
            top: window.innerHeight * 0.60,
            behavior: "smooth"
          });
        }
      }, 100);
    }
  });
}
