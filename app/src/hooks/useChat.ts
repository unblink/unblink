import { createSignal } from "solid-js";
import { chatClient } from "../lib/rpc";
import {
  type Message,
  type Conversation,
} from "../../gen/unblink/chat/v1/chat_pb";
import {
  messages,
  setMessages,
  inputValue,
  setInputValue,
  isLoading,
  setIsLoading,
  activeConversationId,
  setActiveConversationId,
  conversations,
  setConversations,
  setChatInputState,
  type UIMessage,
} from "../signals/chatSignals";

let abortController: AbortController | null = null;
let firstChunkReceived = false;

export function useChat() {
  const [streamingContent, setStreamingContent] = createSignal("");

  const sendMessage = async () => {
    const message = inputValue().trim();
    if (!message || isLoading()) return;

    // Ensure we have an active conversation
    let currentId = activeConversationId();
    if (!currentId) {
      const newConv = await createConversation();
      if (!newConv) return;
      currentId = newConv.id;
      setActiveConversationId(currentId);
    }

    // Add user message immediately
    const userMsg: UIMessage = {
      id: `user_${Date.now()}`,
      type: "user",
      content: message,
      timestamp: Date.now(),
      conversationId: currentId,
    };
    setMessages((prev) => [...prev, userMsg]);
    setInputValue("");
    setIsLoading(true);
    setStreamingContent("");
    setChatInputState('user_sent');
    firstChunkReceived = false;

    // Create placeholder for model response
    const modelMsgId = `model_${Date.now()}`;
    setMessages((prev) => [
      ...prev,
      {
        id: modelMsgId,
        type: "model",
        content: "",
        timestamp: Date.now(),
        conversationId: currentId,
      },
    ]);

    // Set up abort controller for this request
    abortController = new AbortController();

    try {
      const stream = chatClient.sendMessage(
        {
          conversationId: currentId,
          content: message,
          useWebSearch: false,
        },
        { signal: abortController.signal }
      );

      for await (const response of stream) {
        if (response.event.case === "textDelta") {
          const delta = response.event.value;
          // Set first chunk state on first delta
          if (!firstChunkReceived) {
            setChatInputState('first_chunk_arrived');
            firstChunkReceived = true;
          }

          console.log('Received delta:', delta);
          setMessages((prev) => {
            const updated = [...prev];
            try {
              const lastMsg = updated[updated.length - 1];
              if (lastMsg && lastMsg.type === "model") {
                const newContent = lastMsg.content + delta;
                updated[updated.length - 1] = {
                  ...lastMsg,
                  content: newContent,
                };
                console.log('Updated model message:', updated[updated.length - 1]);
              }
            } catch (e) {
              console.error("FUCK:", e);
            }
            return updated;
          });
          setStreamingContent((prev) => prev + delta);
        } else if (response.event.case === "toolCall") {
          const toolEvent = response.event.value;
          console.log("Tool event:", toolEvent);

          // Update the model message with tool call info
          setMessages((prev) => {
            const updated = [...prev];
            const lastMsg = updated[updated.length - 1];
            if (lastMsg && lastMsg.type === "model") {
              const existingToolCalls = lastMsg.toolCalls || [];
              // Find existing tool call or create new one
              const toolCallIndex = existingToolCalls.findIndex(tc => tc.toolName === toolEvent.toolName);

              const newToolCall: {
                toolName: string;
                state: "invoked" | "completed" | "error";
                error?: string;
              } = {
                toolName: toolEvent.toolName,
                state: toolEvent.state as "invoked" | "completed" | "error",
              };

              if (toolEvent.state === "error" && toolEvent.error) {
                newToolCall.error = toolEvent.error;
              }

              let updatedToolCalls;
              if (toolCallIndex >= 0) {
                // Update existing tool call
                updatedToolCalls = [...existingToolCalls];
                updatedToolCalls[toolCallIndex] = newToolCall;
              } else {
                // Add new tool call
                updatedToolCalls = [...existingToolCalls, newToolCall];
              }

              updated[updated.length - 1] = {
                ...lastMsg,
                toolCalls: updatedToolCalls,
              };
            }
            return updated;
          });
        }
      }
    } catch (error) {
      console.error("Error sending message:", error);
      // Remove the placeholder message on error
      setMessages((prev) => {
        const updated = prev.filter((m) => m.id !== modelMsgId);
        return updated;
      });
    } finally {
      setIsLoading(false);
      setStreamingContent("");
      setChatInputState('idle');
      abortController = null;
      // Refresh conversations to update lastUpdated time
      await listConversations();
    }
  };

  const createConversation = async (): Promise<Conversation | null> => {
    try {
      const response = await chatClient.createConversation({
        title: "",
        systemPrompt: `You are a helpful assistant named Unblink. You are created by Zapdos Labs (https://zapdoslabs.com). You can search videos to help answer user questions. Today is ${new Date().toDateString()}.`,
      });
      const conv = response.conversation;
      if (conv) {
        // Refresh the conversation list
        await listConversations();
        return conv;
      }
    } catch (error) {
      console.error("Error creating conversation:", error);
    }
    return null;
  };

  const listConversations = async () => {
    try {
      const response = await chatClient.listConversations({
        pageSize: 50,
        pageToken: "",
      });
      const convs = response.conversations;
      const summaries = convs.map((c) => ({
        id: c.id,
        title: c.title || "New Conversation",
        lastUpdated: c.updatedAt ? Number(c.updatedAt.seconds) * 1000 : Date.now(),
      }));
      setConversations(summaries);
    } catch (error) {
      console.error("Error listing conversations:", error);
    }
  };

  const loadConversation = async (conversationId: string) => {
    try {
      const response = await chatClient.listMessages({
        conversationId,
        pageSize: 100,
        pageToken: "",
      });

      const msgs: UIMessage[] = response.messages.map((m) => {
        // Parse body JSON to extract role and content
        let role = "model";
        let content = "";

        try {
          const body = JSON.parse(m.body);
          role = body.role || "model";
          content = body.content || "";
        } catch (e) {
          console.error("Failed to parse message body:", e);
        }

        return {
          id: m.id,
          type: role === "user" ? "user" : "model",
          content,
          timestamp: m.createdAt ? Number(m.createdAt.seconds) * 1000 : Date.now(),
          conversationId: m.conversationId,
          body: m.body,
        };
      });

      setMessages(msgs);
      setActiveConversationId(conversationId);
    } catch (error) {
      console.error("Error loading conversation:", error);
    }
  };

  const deleteConversation = async (conversationId: string) => {
    try {
      await chatClient.deleteConversation({ conversationId });
      // If we deleted the active conversation, clear state
      if (activeConversationId() === conversationId) {
        setActiveConversationId(null);
        setMessages([]);
      }
      await listConversations();
    } catch (error) {
      console.error("Error deleting conversation:", error);
    }
  };

  const stopGeneration = () => {
    if (abortController) {
      abortController.abort();
      abortController = null;
      setIsLoading(false);
      setStreamingContent("");
    }
  };

  const handleSelectConversation = (id: string) => {
    if (id === activeConversationId()) return;
    setActiveConversationId(id);
    setMessages([]); // Clear while loading
    loadConversation(id);
  };

  const handleNewChat = () => {
    setActiveConversationId(null);
    setMessages([]);
    setInputValue("");
    setIsLoading(false);
  };

  return {
    sendMessage,
    createConversation,
    listConversations,
    loadConversation,
    deleteConversation,
    stopGeneration,
    handleSelectConversation,
    handleNewChat,
    streamingContent,
  };
}
