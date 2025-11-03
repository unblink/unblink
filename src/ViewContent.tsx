import { For, Show, createEffect, onCleanup } from "solid-js";
import CanvasVideo from "./CanvasVideo";
import { setSubscription, viewedMedias } from "./shared";

const GAP_SIZE = '8px';

const chunk = <T,>(arr: T[]): T[][] => {
    const n = viewedMedias().length;
    const size = n === 0 ? 1 : Math.ceil(Math.sqrt(n));
    if (size <= 0) {
        return arr.length ? [arr] : [];
    }
    return Array.from({ length: Math.ceil(arr.length / size) }, (v, i) =>
        arr.slice(i * size, i * size + size)
    );
}

export default function ViewContent() {


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
    const rowsOfMedias = () => chunk(viewedMedias())



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
                    <div class="flex flex-col h-full w-full" style={{ gap: GAP_SIZE }}>
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
                                                <CanvasVideo stream_id={media.stream_id} file_name={media.file_name} />
                                            </div>
                                        }}
                                    </For>
                                </div>
                            )}
                        </For>
                    </div>
                </Show>
            </div>
        </div>
    );
}