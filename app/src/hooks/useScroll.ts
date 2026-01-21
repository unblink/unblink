import { createEffect, on } from "solid-js";
import { chatInputState, activeConversationId, uiBlocks } from "../signals/chatSignals";

export function useScroll(scrollContainerRef: () => HTMLDivElement | undefined) {
  // Scroll to bottom instantly ONLY when conversation changes (not on every message)
  createEffect(on(activeConversationId, (convId) => {
    console.log('[useScroll] activeConversationId changed:', convId);
    const blocks = uiBlocks();
    const container = scrollContainerRef();

    console.log('[useScroll] container:', !!container, 'uiBlocks.length:', blocks.length, 'convId:', convId);

    if (container && blocks.length > 0 && convId) {
      // Use requestAnimationFrame to ensure DOM is updated
      requestAnimationFrame(() => {
        console.log('[useScroll] scrolling to bottom, scrollHeight:', container.scrollHeight);
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
    console.log('[useScroll] chatInputState changed:', chatInputStateValue);
    if (chatInputStateValue === 'idle') return;

    if (chatInputStateValue === 'user_sent') {
      setTimeout(() => {
        const container = scrollContainerRef();
        if (container) {
          console.log('[useScroll] user_sent: scrolling to bottom');
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
          console.log('[useScroll] first_chunk_arrived: scrolling up by 60vh');
          container.scrollBy({
            top: window.innerHeight * 0.60,
            behavior: "smooth"
          });
        }
      }, 100);
    }
  });
}
