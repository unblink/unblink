import { createSignal } from "solid-js";

export type ToolCallState = "invoked" | "completed" | "error";

export interface ToolCall {
  toolName: string;
  state: ToolCallState;
  error?: string;
}

export interface UIMessage {
  id: string;
  type: "user" | "model";
  content: string;
  timestamp: number;
  conversationId: string;
  // Optional: full OpenAI message body as JSON
  body?: string;
  // Tool calls during this message
  toolCalls?: ToolCall[];
}

export interface ConversationSummary {
  id: string;
  title: string;
  lastUpdated: number;
}

// Messages in current conversation
export const [messages, setMessages] = createSignal<UIMessage[]>([]);

// Current input value
export const [inputValue, setInputValue] = createSignal("");

// Loading state for streaming
export const [isLoading, setIsLoading] = createSignal(false);

// Active conversation ID
export const [activeConversationId, setActiveConversationId] = createSignal<string | null>(null);

// List of conversations
export const [conversations, setConversations] = createSignal<ConversationSummary[]>([]);

// Whether we're showing conversation list (history menu)
export const [showHistory, setShowHistory] = createSignal(false);

// Chat input state for scroll effects
export type ChatInputState = 'idle' | 'user_sent' | 'first_chunk_arrived';
export const [chatInputState, setChatInputState] = createSignal<ChatInputState>('idle');
