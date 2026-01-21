import { Switch, Match } from "solid-js";
import { FaSolidSpinner, FaSolidCircleCheck } from "solid-icons/fa";
import { BsX } from "solid-icons/bs";
import type { UIMessage, ToolCall } from "../../signals/chatSignals";
import { ProseText } from "./ProseText";

interface MessageItemProps {
  message: UIMessage;
  showLoading?: boolean;
  isLast?: boolean;
}

function ToolCallItem(props: { toolCall: ToolCall }) {
  return (
    <div class="flex items-center gap-2 text-sm text-white">
      <Switch>
        <Match when={props.toolCall.state === "invoked"}>
          <FaSolidSpinner class="animate-spin" size={14} />
        </Match>
        <Match when={props.toolCall.state === "completed"}>
          <FaSolidCircleCheck size={14} />
        </Match>
        <Match when={props.toolCall.state === "error"}>
          <BsX size={16} />
        </Match>
      </Switch>
      <span class="capitalize">{props.toolCall.toolName}</span>
      {props.toolCall.error && (
        <span class="text-xs ml-1">{props.toolCall.error}</span>
      )}
    </div>
  );
}

export default function MessageItem(props: MessageItemProps) {
  return (
    <div class={`w-full max-w-4xl mx-auto ${props.isLast ? 'min-h-[60vh]' : ''}`}>
      <Switch>
        <Match when={props.message.type === "user"}>
          <div class="flex justify-start">
            <div class="bg-neu-800 text-neu-100 px-5 py-2.5 rounded-2xl max-w-md shadow-sm">
              <p class="text-sm font-medium leading-relaxed">{props.message.content}</p>
            </div>
          </div>
        </Match>

        <Match when={props.message.type === "model"}>
          <div class="px-4 py-4 flex flex-col">
            <ProseText content={props.message.content} />

            {/* Tool calls */}
            {props.message.toolCalls && props.message.toolCalls.length > 0 && (
              <div class="mt-3 flex flex-wrap gap-2">
                {props.message.toolCalls.map((toolCall) => (
                  <ToolCallItem toolCall={toolCall} />
                ))}
              </div>
            )}

            {props.showLoading && (
              <div class="mt-4 flex gap-1">
                <div class="w-2 h-2 bg-neu-500 rounded-full animate-pulse"></div>
                <div class="w-2 h-2 bg-neu-500 rounded-full animate-pulse" style="animation-delay: 0.2s"></div>
                <div class="w-2 h-2 bg-neu-500 rounded-full animate-pulse" style="animation-delay: 0.4s"></div>
              </div>
            )}
          </div>
        </Match>
      </Switch>
    </div>
  );
}
