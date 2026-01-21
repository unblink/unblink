import { FiArrowUp, FiLoader } from "solid-icons/fi";
import { inputValue, setInputValue, isLoading } from "../../signals/chatSignals";
import { useChat } from "../../hooks/useChat";

export default function HeroSection() {
  const { sendMessage } = useChat();

  const handleKeyPress = (e: KeyboardEvent) => {
    if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault();
      sendMessage();
    }
  };

  return (
    <div class="w-full h-full overflow-y-auto">
      <div class="flex flex-col items-center justify-center min-h-full px-4 py-12 max-w-4xl mx-auto">
        <h1 class="text-4xl md:text-6xl font-bold text-neu-100 mb-16 tracking-tight text-center">Chat</h1>

        <div class="w-full max-w-2xl mb-32">
          {/* Big Input Box */}
          <div class="w-full bg-neu-850 border border-neu-800 rounded-3xl p-4 transition-all duration-150 focus-within:border-neu-750 focus-within:bg-neu-800">
            <textarea
              value={inputValue()}
              onInput={(e) => {
                setInputValue(e.currentTarget.value);
                e.currentTarget.style.height = 'auto';
                e.currentTarget.style.height = e.currentTarget.scrollHeight + 'px';
              }}
              onKeyPress={handleKeyPress}
              placeholder="Let's start chatting..."
              rows={1}
              class="w-full px-4 py-2 bg-transparent text-neu-100 placeholder:text-neu-500 focus:outline-none resize-none max-h-48 overflow-y-auto text-lg"
            />
            <div class="flex justify-end items-end mt-4 px-1">
              <button
                onClick={() => sendMessage()}
                disabled={!inputValue().trim() || isLoading()}
                class="p-2.5 rounded-full bg-neu-200 text-neu-900 disabled:opacity-50 disabled:cursor-not-allowed hover:bg-white hover:shadow-xl hover:scale-105 transition-all duration-150 shadow-lg"
              >
                {isLoading() ? <FiLoader size={20} class="animate-spin" /> : <FiArrowUp size={20} />}
              </button>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
