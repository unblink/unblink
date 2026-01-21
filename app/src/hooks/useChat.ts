import { createSignal } from "solid-js";
import { chatClient } from "../lib/rpc";
import {
  type Conversation,
} from "../../gen/unblink/chat/v1/chat_pb";
import {
  uiBlocks,
  setUIBlocks,
  inputValue,
  setInputValue,
  isLoading,
  setIsLoading,
  activeConversationId,
  setActiveConversationId,
  conversations,
  setConversations,
  setChatInputState,
  type UIBlock,
  type UIBlockData,
  type UserData,
  type ModelData,
  type ToolData,
  type UIRole,
} from "../signals/chatSignals";

let abortController: AbortController | null = null;
let firstChunkReceived = false;

// Track UI blocks by ID for replacement (same ID = replace, different ID = new)
const blockMap = new Map<string, UIBlock>();

// Helper function to upsert a block (update if exists, insert if new)
const upsertBlock = (block: UIBlock) => {
  blockMap.set(block.id, block);
  // Return sorted blocks by created_at
  const sorted = Array.from(blockMap.values()).sort((a, b) =>
    (a.createdAt || 0) - (b.createdAt || 0)
  );
  setUIBlocks(sorted);
};

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
      // Clear block map for new conversation
      blockMap.clear();
    }

    setInputValue("");
    setIsLoading(true);
    setStreamingContent("");
    setChatInputState('user_sent');
    firstChunkReceived = false;

    // Track accumulated model content for display
    let accumulatedModelContent = "";
    let tempModelBlockId = `temp_model_${Date.now()}`;

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
          accumulatedModelContent += delta;

          // Update temporary model block for immediate display
          const tempModelBlock: UIBlock = {
            id: tempModelBlockId,
            conversationId: currentId,
            role: "model",
            data: { content: accumulatedModelContent } as ModelData,
            createdAt: Date.now(),
          };
          upsertBlock(tempModelBlock);

          setStreamingContent((prev) => prev + delta);
        } else if (response.event.case === "uiBlock") {
          const uiBlockEvent = response.event.value;
          console.log("UI block event:", uiBlockEvent);

          // Parse the data_json from the event
          let data: UIBlockData;
          try {
            const dataJson = JSON.parse(uiBlockEvent.dataJson);
            data = dataJson;
          } catch (e) {
            console.error("Failed to parse UI block data:", e);
            continue;
          }

          // Create UI block from event
          const block: UIBlock = {
            id: uiBlockEvent.id,
            conversationId: uiBlockEvent.conversationId,
            role: uiBlockEvent.role as UIRole,
            data,
            createdAt: uiBlockEvent.createdAt ? Number(uiBlockEvent.createdAt.seconds) * 1000 : Date.now(),
          };

          // If this is a model block, it replaces the temporary accumulated block
          if (block.role === "model") {
            // Remove the temporary block if it exists
            blockMap.delete(tempModelBlockId);
          }

          // Upsert the block (same ID = replace, different ID = new)
          upsertBlock(block);
        }
      }
    } catch (error) {
      console.error("Error sending message:", error);
      // Remove the temporary model block on error
      blockMap.delete(tempModelBlockId);
      setUIBlocks(Array.from(blockMap.values()).sort((a, b) =>
        (a.createdAt || 0) - (b.createdAt || 0)
      ));
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
      // Clear block map for new conversation
      blockMap.clear();

      const response = await chatClient.listUIBlocks({
        conversationId,
      });

      const blocks: UIBlock[] = response.uiBlocks.map((b) => {
        let data: UIBlockData;
        try {
          const dataJson = JSON.parse(b.dataJson);
          data = dataJson;
        } catch (e) {
          console.error("Failed to parse UI block data:", e);
          // Default to empty model data
          data = { content: "" } as ModelData;
        }

        return {
          id: b.id,
          conversationId: b.conversationId,
          role: b.role as UIRole,
          data,
          createdAt: b.createdAt ? Number(b.createdAt.seconds) * 1000 : Date.now(),
        };
      });

      // Populate block map and set UI blocks
      blocks.forEach((block) => blockMap.set(block.id, block));
      setUIBlocks(blocks);
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
        blockMap.clear();
        setUIBlocks([]);
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
    blockMap.clear();
    setUIBlocks([]); // Clear while loading
    loadConversation(id);
  };

  const handleNewChat = () => {
    setActiveConversationId(null);
    blockMap.clear();
    setUIBlocks([]);
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
