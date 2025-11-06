import { format } from "date-fns";
import { createResource, For, Show } from "solid-js";
import type { RecordingsResponse } from "~/shared";
import LayoutContent from "./LayoutContent";
import { cameras, setTab } from "~/src/shared";

async function fetchRecordings() {
    const response = await fetch("/recordings");
    const recordings = await response.json();
    console.log("Fetched recordings:", recordings);
    return recordings;
}

export default function HistoryContent() {
    const [recordings] = createResource<RecordingsResponse>(fetchRecordings);
    const isEmpty = () => {
        const recs = recordings();
        if (!recs) return true;
        return Object.entries(recs).length == 0
    }

    const formatDuration = (ms: number) => {
        const totalSeconds = Math.floor(ms / 1000);
        const minutes = Math.floor(totalSeconds / 60);
        const seconds = totalSeconds % 60;
        return `${minutes}m ${seconds}s`;
    }

    return <LayoutContent title="History">
        <div class="h-full overflow-auto p-4">

            <Show when={!isEmpty()} fallback={
                <div class="h-full flex items-center justify-center text-neu-500">
                    No recordings available
                </div>
            }>
                <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4 ">
                    <For each={Object.entries(recordings() || {})}>
                        {([streamId, recordings]) => (
                            <For each={recordings}>
                                {({ file_name, from_ms, to_ms }) => {
                                    const title = () => cameras().find(camera => camera.id === streamId)?.name ?? streamId
                                    return <div
                                        onClick={() => {
                                            setTab({
                                                type: 'view',
                                                medias: [{
                                                    stream_id: streamId,
                                                    file_name,
                                                }],
                                            });
                                        }}
                                        class="bg-neu-850 p-4 rounded-lg space-y-1 cursor-pointer hover:bg-neu-800 border border-neu-800">
                                        <p class="font-semibold">{title()}</p>
                                        <div>
                                            <p class="text-sm text-neu-400">{from_ms ? format(from_ms, 'PPpp') : 'N/A'}</p>
                                            <Show when={to_ms && from_ms}>
                                                {(val) => <p class="text-sm text-neu-400">Duration: {formatDuration(to_ms! - from_ms!)}</p>}
                                            </Show>
                                        </div>
                                    </div>
                                }}
                            </For>
                        )}
                    </For>
                </div>
            </Show>
        </div>
    </LayoutContent>
}