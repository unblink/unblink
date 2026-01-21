import { onMount, onCleanup } from 'solid-js';
import { uiBlocks } from '../../signals/chatSignals';
import { useChat } from '../../hooks/useChat';
import HeroSection from '../chat/HeroSection';
import MessageList from '../chat/MessageList';
import ChatInput from '../chat/ChatInput';

export default function TabChat() {
  const { handleNewChat, listConversations } = useChat();

  // Load conversations on mount
  onMount(() => {
    listConversations();
  });

  // Keyboard shortcut: Escape to go back to hero/new chat
  onMount(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') handleNewChat();
    };
    window.addEventListener('keydown', onKey);
    onCleanup(() => window.removeEventListener('keydown', onKey));
  });

  return (
    <div class="flex h-full relative">
      {/* Main Content Area */}
      <div class="flex-1 flex flex-col h-full transition-all duration-300 w-full relative">
        {uiBlocks().length === 0 ? (
          // HERO / EMPTY STATE LAYOUT
          <HeroSection />
        ) : (
          // EXISTING CHAT LAYOUT
          <>
            {/* Messages Area */}
            <MessageList />

            {/* Input Area */}
            <ChatInput />
          </>
        )}
      </div>
    </div>
  );
}
