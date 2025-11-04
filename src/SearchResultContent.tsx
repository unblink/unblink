import { FaSolidArrowLeft } from "solid-icons/fa";
import { createEffect, createSignal, For, Show } from "solid-js";


import type { MediaUnit } from "~/shared";
import IslandRow from "./search/IslandRow";
import LoadingSkeleton from "./search/LoadingSkeleton";
import SearchBar from "./SearchBar";
import { tab } from "./shared";

export type Island = (MediaUnit & { _distance: number })[]

// State is updated to include a nullable summary field
type SearchState = {
    type: "idle"
} | {
    type: "searching"
    query: string,
} | {
    type: "error",
} | {
    type: "result",
    query: string,
    // results: (MediaUnit & { _distance: number })[],
    // clusters: Record<string, { description: string, items: { id: string, media_id: string, at_time: string }[] }>,
    islands: Island[],
    summary: string | null; // Summary can be null while streaming
}

export default function SearchResultContent() {
    const [searchState, setSearchState] = createSignal<SearchState>({
        type: "idle",
    });

    const q = () => null;

    // const q = () => {
    //     const t = tabId();
    //     if (t.type === "search-result") {
    //         return t.query;
    //     }
    //     return null;
    // }

    createEffect(async () => {
        const query = q();
        if (!query) return;

        // setLocalStorage$('recent_searches', (old) => {
        //     const newSearches = [query, ...old.filter((s) => s !== query)];
        //     return newSearches.slice(0, 5);
        // });

        setSearchState({ type: "searching", query });
        try {
            const response = await fetch(`/api/v1/search`, {
                method: "POST",
                headers: { "Content-Type": "application/json" },
                body: JSON.stringify({ query }),
            });

            if (!response.ok || !response.body) {
                throw new Error("Search request failed");
            }

            // NEW: Consume the response as an NDJSON stream
            const reader = response.body.getReader();
            const decoder = new TextDecoder();
            let buffer = '';

            while (true) {
                const { value, done } = await reader.read();
                console.log({ value, done });
                if (done) break;

                buffer += decoder.decode(value, { stream: true });
                const lines = buffer.split('\n');

                // Keep the last (potentially incomplete) line in the buffer
                buffer = lines.pop() || '';

                for (const line of lines) {
                    if (line.trim() === '') continue; // Skip empty lines

                    const data = JSON.parse(line);

                    // Check the content of the parsed JSON object
                    if (data.type == "items") {
                        // // This is the first chunk: set the items and initialize summary to null
                        // setSearchState({
                        //     type: "result",
                        //     query,
                        //     results: data.items,
                        //     summary: null,
                        // });
                    } else if (data.type == "summary") {
                        // This is the second chunk: update the existing state with the summary
                        setSearchState(prev => {
                            if (prev.type === 'result') {
                                return { ...prev, summary: data.summary };
                            }
                            return prev;
                        });
                    } else if (data.type == "clustering") {
                        // setSearchState({
                        //     type: "result",
                        //     query,
                        //     // clusters: data.clusters,
                        //     summary: null, // Summary will be updated when its chunk arrives
                        // });
                    } else if (data.type == "islands") {
                        setSearchState({
                            type: "result",
                            query,
                            islands: data.islands,
                            summary: null, // Summary will be updated when its chunk arrives
                        });
                    }

                    else if (data.error) {
                        throw new Error(data.error);
                    }
                }
            }
        } catch (error) {
            console.error("Failed to fetch search results:", error);
            setSearchState({ type: "error" });
        }
    });

    const result = () => {
        const s = searchState()
        if (s.type === "result") return s
    }

    // The JSX/render part remains exactly the same as the previous solution.
    // Its logic already handles the progressive rendering correctly.
    return <div class="h-screen flex flex-col border-l border-neutral-800 bg-neutral-900 overflow-hidden ">
        <div class="overflow-auto h-full flex flex-col">
            <div class="px-4 py-8 mx-auto w-full xl:w-[50vw] flex flex-col ">
                <div class="flex items-center space-x-4 mb-4 flex-none">
                    <button class="btn-tertiary">
                        <FaSolidArrowLeft class="w-4 h-4" />
                        <div class="font-bold text-sm">Back</div>
                    </button>
                </div>

                <div data-scheme="lighter" class="group relative h-24 flex-none">
                    <SearchBar variant="xl" placeholder={q} />
                </div>

                <div class="text-neutral-400 flex-1 ">
                    <Show when={searchState().type === "searching"}>
                        <LoadingSkeleton />
                    </Show>
                    <Show when={searchState().type === "error"}>
                        <div class="text-red-500">Error occurred while searching. Please try again.</div>
                    </Show>
                    <Show when={result()} >
                        {s => {
                            const rows = () => s().islands
                            return <div>
                                {/* Summary Section: Shows skeleton while summary is loading */}
                                <div class="mb-6">
                                    <Show when={s().summary} fallback={<LoadingSkeleton />}>
                                        <div class="p-4 bg-neutral-800 rounded-3xl animate-fade-in">
                                            <h2 class="font-bold text-lg mb-2 text-neutral-200">Summary</h2>
                                            <p class="text-neutral-300">{s().summary}</p>
                                        </div>
                                    </Show>
                                </div>




                                <Show when={rows().length > 0} fallback={
                                    <div class="py-2">No results found for "{s().query}".</div>
                                }>
                                    <div class="space-y-4">
                                        <For each={rows()}>

                                            {
                                                island => <IslandRow island={island} selectItem={() => { }} />
                                            }

                                        </For>
                                    </div>
                                </Show>
                            </div>
                        }}
                    </Show>
                </div >
            </div >
        </div >
    </div >
}