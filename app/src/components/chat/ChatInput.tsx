import { FiArrowUp, FiSquare } from "solid-icons/fi";
import { onMount } from "solid-js";
import { inputValue, isLoading, setInputValue } from "../../signals/chatSignals";
import ChatControls from "./ChatControls";
import { useChat } from "../../hooks/useChat";

export default function ChatInput() {
  const { sendMessage, stopGeneration } = useChat();

  let textareaRef: HTMLTextAreaElement | undefined;

  onMount(() => {
    textareaRef?.focus();
  });

  const handleKeyPress = (e: KeyboardEvent) => {
    if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault();
      sendMessage();
    }
  };

  return (
    <div class="pb-4 px-4">
      <div class="max-w-4xl mx-auto bg-neu-850 border border-neu-800 rounded-3xl p-4 transition-all duration-150 focus-within:border-neu-750 focus-within:bg-neu-800 shadow-lg">
        <textarea
          ref={textareaRef}
          value={inputValue()}
          onInput={(e) => {
            setInputValue(e.currentTarget.value);
            e.currentTarget.style.height = 'auto';
            e.currentTarget.style.height = e.currentTarget.scrollHeight + 'px';
          }}
          onKeyPress={handleKeyPress}
          placeholder="Reply..."
          rows={1}
          class="w-full px-4 py-2 bg-transparent text-neu-100 placeholder:text-neu-500 focus:outline-none resize-none max-h-48 overflow-y-auto text-lg"
        />
        <div class="flex justify-between items-center mt-4 px-1">
          <ChatControls />
          <button
            onClick={() => isLoading() ? stopGeneration() : sendMessage()}
            disabled={!isLoading() && !inputValue().trim()}
            class="p-2.5 rounded-full bg-neu-200 text-neu-900 disabled:opacity-50 disabled:cursor-not-allowed hover:bg-white hover:shadow-xl hover:scale-105 transition-all duration-150 shadow-lg"
          >
            {isLoading() ? <FiSquare size={20} /> : <FiArrowUp size={20} />}
          </button>
        </div>
      </div>
    </div>
  );
}
