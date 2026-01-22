import { For, Switch, Match } from "solid-js";
import { FaSolidSpinner, FaSolidCircleCheck } from "solid-icons/fa";
import { BsX } from "solid-icons/bs";
import type { UIBlock } from "../../signals/chatSignals";
import { ProseText } from "./ProseText";
import LoadingDots from "./LoadingDots";

interface ToolCallItemProps {
  toolName: string;
  state: "invoked" | "completed" | "error";
  displayMessage?: string;
  error?: string;
}

function ToolCallItem(props: ToolCallItemProps) {
  const displayText = () =>
    props.displayMessage ?? props.toolName.replace(/_/g, " ");

  console.log("[ToolCallItem] props:", props);
  console.log("[ToolCallItem] displayText:", displayText());

  return (
    <div class="flex items-center gap-2 text-sm text-white">
      <Switch>
        <Match when={props.state === "invoked"}>
          <FaSolidSpinner class="animate-spin" size={14} />
        </Match>
        <Match when={props.state === "completed"}>
          <FaSolidCircleCheck size={14} />
        </Match>
        <Match when={props.state === "error"}>
          <BsX size={16} />
        </Match>
      </Switch>
      <span>{displayText()}</span>
      {props.error && (
        <span class="text-xs text-red-400 ml-1">{props.error}</span>
      )}
    </div>
  );
}

interface UIBlockListProps {
  blocks: UIBlock[];
  showLoading?: boolean;
}

export default function UIBlockList(props: UIBlockListProps) {
  const lastBlock = () => props.blocks[props.blocks.length - 1];
  const isLastModelBlock = (block: UIBlock) =>
    block === lastBlock() && block.role === "model";

  return (
    <div class="w-full max-w-4xl mx-auto">
      <For each={props.blocks}>
        {(block) => (
          // Add min-h-[60vh] to last model block for scroll behavior - ensures content
          // starts near top of viewport when streaming, giving room to scroll up
          <div class={isLastModelBlock(block) ? "min-h-[60vh]" : ""}>
            <Switch>
              <Match when={block.role === "user"}>
                <div class="flex justify-start">
                  <div class="bg-neu-800 text-neu-100 px-5 py-2.5 rounded-2xl max-w-md shadow-sm">
                    <p class="text-sm font-medium leading-relaxed">{(block.data as any).content}</p>
                  </div>
                </div>
              </Match>

              <Match when={block.role === "model"}>
                <div class="py-4 flex flex-col">
                  <ProseText content={(block.data as any).content} />
                  {props.showLoading && isLastModelBlock(block) && <LoadingDots />}
                </div>
              </Match>

              <Match when={block.role === "tool"}>
                <div class="py-2">
                  <ToolCallItem
                    toolName={(block.data as any).toolName}
                    state={(block.data as any).state}
                    displayMessage={(block.data as any).displayMessage}
                    error={(block.data as any).error}
                  />
                </div>
              </Match>

              <Match when={block.role === "system"}>
                <div class="py-2 text-neu-500 text-sm">
                  {(block.data as any).content}
                </div>
              </Match>
            </Switch>
          </div>
        )}
      </For>
    </div>
  );
}
