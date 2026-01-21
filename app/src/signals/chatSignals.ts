import { createSignal } from "solid-js";

export type UIRole = "user" | "model" | "tool" | "system";
export type ToolCallState = "invoked" | "completed" | "error";

// Data structures for each UI block type
export interface UserData {
  content: string;
}

export interface ModelData {
  content: string;
}

export interface ToolData {
  toolName: string;
  state: ToolCallState;
  error?: string;
  content?: string;
}

export interface SystemData {
  content: string;
}

export type UIBlockData = UserData | ModelData | ToolData | SystemData;

export interface UIBlock {
  id: string;
  conversationId: string;
  role: UIRole;
  data: UIBlockData;
  createdAt?: number;
}

// For backward compatibility during transition
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

// UI blocks in current conversation (replaces messages)
export const [uiBlocks, setUIBlocks] = createSignal<UIBlock[]>([]);

// For backward compatibility - alias to uiBlocks
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
