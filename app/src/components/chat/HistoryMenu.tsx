import { FiClock } from 'solid-icons/fi';
import { useChat } from '../../hooks/useChat';
import { activeConversationId } from '../../signals/chatSignals';
import type { Conversation } from '../../../gen/unblink/chat/v1/chat_pb';
import { ArkMenu, type ArkMenuItem } from '../../ark/ArkMenu';

interface HistoryMenuProps {
  class?: string;
  width?: string;
}

export const HistoryMenu = (props: HistoryMenuProps) => {
  const { handleSelectConversation, conversations } = useChat();

  const items = () => conversations().map(conv => ({
    id: conv.id,
    title: conv.title || 'Untitled Chat',
    subtitle: conv.updatedAt ? new Date(Number(conv.updatedAt.seconds) * 1000).toLocaleString() : undefined,
    icon: <FiClock />
  }));

  return (
    <ArkMenu
      items={items}
      class={props.class}
      width={props.width}
      triggerIcon={<FiClock />}
      emptyContent={<div class="text-sm text-neu-500">No recent chats</div>}
      onSelect={(id) => handleSelectConversation(id)}
      activeItemId={activeConversationId() || undefined}
      itemRender={(item: ArkMenuItem) => (
        <div class="flex-1 min-w-0">
          <div class="font-semibold text-white truncate">{item.title}</div>
          {item.subtitle && <div class="mt-0.5 text-neu-500 text-xs truncate">{item.subtitle}</div>}
        </div>
      )}
    />
  );
};

export default HistoryMenu;
