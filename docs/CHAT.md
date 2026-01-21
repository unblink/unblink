# Chat-RPC Behavioral Specification (Single-AI, No Images)

## Client Operations

| #   | Operation                  | Description                                                                           |
| --- | -------------------------- | ------------------------------------------------------------------------------------- |
| 1   | Send chat message          | Client sends user text with optional flags (e.g., web search) to server               |
| 2   | Receive streaming response | Client progressively displays AI-generated text chunks and artifacts as they arrive   |
| 3   | Display artifacts          | Client renders structured outputs (search results, tool outputs) attached to messages |
| 4   | Stop generation            | Client sends signal to cancel in-progress AI response                                 |
| 5   | Regenerate response        | Client requests AI to regenerate response from a specific message with optional flags |
| 6   | Load conversation          | Client retrieves and displays full message history with artifacts for a conversation  |
| 7   | List conversations         | Client retrieves and displays summary of all user conversations                       |

---

## Server Operations

| #   | Operation            | Description                                                                                 |
| --- | -------------------- | ------------------------------------------------------------------------------------------- |
| 1   | Receive chat message | Server validates authentication and processes user message with flags                       |
| 2   | Stream AI response   | Server streams AI-generated content back to client in chunks                                |
| 3   | Execute web search   | Server searches web via API if web search flag is set                                       |
| 4   | Execute AI tools     | Server runs tools (prompt modification, function calls) when requested by the AI system     |
| 5   | Create artifacts     | Server creates and attaches structured outputs (search results, tool outputs) to messages   |
| 6   | Persist messages     | Server saves user messages, AI responses, and artifacts to database                         |
| 7   | Create conversation  | Server initializes new conversation with unique ID and metadata                             |
| 8   | Update conversation  | Server modifies conversation title, system prompt, or timestamp                             |
| 9   | Stop generation      | Server cancels active AI generation request and closes stream                               |
| 10  | Regenerate response  | Server re-processes conversation from specified message with flags and streams new response |
| 11  | Authenticate user    | Server validates JWT tokens on all incoming requests                                        |

> **Note:** There is a single, hard-coded AI system instruction for all conversations. No per-character or per-user AI customization.

---

## Data Requirements

| Entity         | Fields Required                                                        |
| -------------- | ---------------------------------------------------------------------- |
| User           | ID, authentication token                                               |
| Conversation   | ID, tenant ID, title, system prompt, created/updated timestamps        |
| Message        | ID, conversation ID, type (user/model/tool), content, timestamp, order |
| Artifact       | ID, message ID, type (search/tool), data, timestamp                    |
| Tool Execution | Message ID, tool name, start/complete status                           |

---

## Non-Functional Requirements

| Requirement             | Description                                                  |
| ----------------------- | ------------------------------------------------------------ |
| Streaming latency       | AI response chunks must appear within 100ms of generation    |
| Message persistence     | All messages must be saved before responding to client       |
| Authentication          | All requests must include a valid JWT token                  |
| Error handling          | Server returns descriptive errors for all failure modes      |
| Concurrency             | System supports multiple simultaneous conversations per user |
| Backwards compatibility | Protocol changes must support older clients                  |
| Tenant isolation        | Users can only access their own data                         |
