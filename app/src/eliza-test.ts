import { createClient } from "@connectrpc/connect";
import { createConnectTransport } from "@connectrpc/connect-web";
import { ElizaService } from "../gen/connectrpc/eliza/v1/eliza_pb";

const BASE_URL = import.meta.env.VITE_RELAY_API_URL || "http://localhost:8020";

const transport = createConnectTransport({
  baseUrl: BASE_URL,
});

async function main() {
  const client = createClient(ElizaService, transport);

  console.log("\n--- Unary RPC (Say) ---");
  const sayRes = await client.say({ sentence: "I feel happy." });
  console.log(sayRes);

  console.log("\n--- Server Streaming (Introduce) ---");
  const introduceStream = await client.introduce({ name: "Tri" });
  for await (const res of introduceStream) {
    console.log(res);
  }

  console.log("\n--- Client Streaming (Consult) ---");
  try {
    async function* createMessages() {
      yield { sentence: "First thought" };
      yield { sentence: "Second thought" };
      yield { sentence: "Third thought" };
    }
    const consultRes = await client.consult(createMessages());
    console.log(consultRes);
  } catch (err: any) {
    if (err.message?.includes("fetch API does not support streaming request bodies")) {
      console.log("⚠️ Expected: Browser fetch API doesn't support streaming request bodies (client streaming)");
    } else {
      console.error("Unexpected error:", err);
    }
  }

  console.log("\n--- Bidi Streaming (Chat) ---");
  try {
    async function* chatMessages() {
      yield { sentence: "Hello from Chat" };
      await new Promise(r => setTimeout(r, 100));
      yield { sentence: "Another message" };
      await new Promise(r => setTimeout(r, 100));
      yield { sentence: "Goodbye" };
    }

    const chatStream = client.chat(chatMessages());
    for await (const res of chatStream) {
      console.log(res);
    }
  } catch (err: any) {
    if (err.message?.includes("fetch API does not support streaming request bodies")) {
      console.log("⚠️ Expected: Browser fetch API doesn't support streaming request bodies (bidi streaming)");
    } else {
      console.error("Unexpected error:", err);
    }
  }

  console.log("\n--- Summary ---");
  console.log("✅ Unary RPC: Works");
  console.log("✅ Server Streaming: Works");
  console.log("❌ Client Streaming: Not supported in browsers (use Node.js or WebSocket transport)");
  console.log("❌ Bidi Streaming: Not supported in browsers (use Node.js or WebSocket transport)");
}

// Export for console testing
(window as any).testEliza = main;

console.log("Eliza test loaded. Run testEliza() in console to test.");
