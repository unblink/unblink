import { isLoading, uiBlocks, chatInputState } from "../../signals/chatSignals";
import UIBlockList from "./UIBlockList";
import LoadingDots from "./LoadingDots";
import { useScroll } from "../../hooks/useScroll";

export default function MessageList() {
  let scrollContainerRef: HTMLDivElement | undefined;
  useScroll(() => scrollContainerRef);

  // Show loading dots only when user sent message but first chunk hasn't arrived yet
  const showLoadingDots = () =>
    isLoading() && (chatInputState() === 'user_sent' || uiBlocks().length === 0);

  return (
    <div
      ref={(el) => scrollContainerRef = el}
      class="flex-1 overflow-y-auto p-4 space-y-4"
      style={{ "overflow-anchor": "none" }}
    >
      <UIBlockList
        blocks={uiBlocks()}
        showLoading={isLoading() && uiBlocks().length > 0 && uiBlocks()[uiBlocks().length - 1]?.role === "model"}
      />
      {showLoadingDots() && (
        <div class="w-full max-w-4xl mx-auto px-2 py-4">
          <LoadingDots />
        </div>
      )}
    </div>
  );
}
