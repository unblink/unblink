import { Switch, Match } from "solid-js";
import type { UIMessage } from "../../signals/chatSignals";
import { ProseText } from "./ProseText";

interface MessageItemProps {
  message: UIMessage;
  showLoading?: boolean;
  isLast?: boolean;
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
