import { For, Show, createEffect, onCleanup, createSignal } from "solid-js";
import CanvasVideo from "./CanvasVideo";
import { setSubscription, settings, tab, } from "./shared";
import { FaSolidObjectGroup } from "solid-icons/fa";
import ArkSwitch from "./ark/ArkSwitch";

const GAP_SIZE = '8px';

const chunk = <T,>(arr: T[]): T[][] => {
    const n = arr.length;
    const size = n === 0 ? 1 : Math.ceil(Math.sqrt(n));
    if (size <= 0) {
        return arr.length ? [arr] : [];
    }
    return Array.from({ length: Math.ceil(arr.length / size) }, (v, i) =>
        arr.slice(i * size, i * size + size)
    );
}

export default function ViewContent() {
    const [showDetections, setShowDetections] = createSignal(true);


    const viewedMedias = () => {
        const t = tab();
        return t.type === 'view' ? t.medias : [];
    }


    // Handle subscriptions
    createEffect(() => {
        const medias = viewedMedias();
        if (medias && medias.length > 0) {
            console.log('Subscribing to streams:', medias);
            const session_id = crypto.randomUUID();

            setSubscription({
                session_id,
                streams: medias.map(media => ({ id: media.stream_id, file_name: media.file_name })),
            });
        } else {
            setSubscription();
        }
    });

    const cols = () => {

        const n = viewedMedias().length;
        return n === 0 ? 1 : Math.ceil(Math.sqrt(n));
    }

    const rowsOfMedias = () => chunk(viewedMedias());



    // Cleanup subscriptions on unmount
    onCleanup(() => {
        console.log('ViewContent unmounting, clearing subscriptions');
        setSubscription();
    });

    return (
        <div class="flex flex-col h-screen">
            <div class="flex-1 mr-2 my-2">
                <Show
                    when={rowsOfMedias().length > 0}
                    fallback={<div class="flex justify-center items-center h-full">No camera selected</div>}
                >
                    <div class="h-full w-full flex flex-col space-y-2">
                        <div class="flex-none flex items-center space-x-2 py-2 px-4 bg-neu-900 rounded-2xl border border-neu-800">
                            <div class="flex-1 text-sm text-neu-400">Viewing {viewedMedias().length} streams</div>
                            <Show when={settings()['object_detection_enabled'] === 'true'}>
                                <div>
                                    <ArkSwitch
                                        label="Show detection boxes"
                                        checked={showDetections}
                                        onCheckedChange={(e) => setShowDetections(e.checked)}
                                    />
                                </div>
                            </Show>
                        </div>
                        <div class="flex-1 flex flex-col" style={{ gap: GAP_SIZE }}>
                            <For each={rowsOfMedias()}>
                                {(row, rowIndex) => (
                                    <div
                                        class="flex flex-1"
                                        style={{
                                            'justify-content': rowIndex() === rowsOfMedias().length - 1 && row.length < cols() ? 'center' : 'flex-start',
                                            gap: GAP_SIZE,
                                        }}
                                    >
                                        <For each={row}>
                                            {(media) => {
                                                return <div style={{ width: `calc((100% - (${cols() - 1} * ${GAP_SIZE})) / ${cols()})`, height: '100%' }}>
                                                    <CanvasVideo stream_id={media.stream_id} file_name={media.file_name} showDetections={showDetections} />
                                                </div>
                                            }}
                                        </For>
                                    </div>
                                )}
                            </For>
                        </div>
                    </div>
                </Show>
            </div>
        </div>
    );
}