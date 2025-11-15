import { createEffect, createSignal, For, Match, Show, Switch } from "solid-js";
import LayoutContent from "./LayoutContent";
import LoadingSkeleton from "~/src/search/LoadingSkeleton";
import { cameras, tab } from "~/src/shared";
import { format } from "date-fns";
import SearchBar from "~/src/SearchBar";
import type { MediaUnit } from "~/shared/database";



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
    result: {
        media_units: MediaUnit[]
    }
}

export default function SearchResultContent() {
    const [searchState, setSearchState] = createSignal<SearchState>({
        type: "idle",
    });

    const q = () => {
        const t = tab();
        if (t.type === "search_result") {
            return t.query;
        }
        return null;
    }
    createEffect(async () => {
        const query = q();
        if (!query) return;

        setSearchState({ type: "searching", query });
        try {

            const response = await fetch(`/search`, {
                method: "POST",
                headers: { "Content-Type": "application/json" },
                body: JSON.stringify({
                    query
                }),
            });

            if (!response.ok || !response.body) {
                throw new Error("Search request failed");
            }

            const data = await response.json();
            setSearchState({ type: "result", query, result: data });

        } catch (error) {
            console.error("Failed to fetch search results:", error);
            setSearchState({ type: "error" });
        }
    });

    // Typescript: guard for search result
    const searchState_Result = () => {
        const state = searchState();
        return state.type === "result" ? state : undefined;
    }

    return <LayoutContent
        title="Search Results"
        hide_head
    >

        <div class="space-y-2 overflow-y-auto h-full">
            <div class="relative h-18 mt-4">
                <SearchBar variant="lg" placeholder={q} />
            </div>

            <Switch>
                <Match when={searchState().type === "searching"}>
                    <LoadingSkeleton />
                </Match>
                <Match when={searchState().type === "error"}>
                    <div class="p-4 text-red-500">
                        An error occurred while searching. Please try again.
                    </div>
                </Match>
                <Match when={searchState_Result()}>
                    {s => <Show when={s().result.media_units.length > 0} fallback={
                        <div class="p-4">
                            No results found for "{s().query}"
                        </div>
                    }>
                        <div class="space-y-2 p-4">
                            <For each={s().result.media_units}>
                                {(mu) => {
                                    const name = () => cameras().find(c => c.id === mu.media_id)?.name || 'Unknown Camera';
                                    return <div class="animate-push-down p-4 bg-neu-850 rounded-2xl space-y-2">
                                        <div class="font-semibold">{name()}</div>
                                        <div class="text-neu-400 text-sm">{format(mu.at_time, 'PPpp')}</div>
                                        <div class="py-1">{mu.description}</div>
                                        <img src={`/files?path=${mu.path}`} class="rounded-lg" />
                                    </div>
                                }}
                            </For>
                        </div>
                    </Show>}
                </Match>


            </Switch>
        </div>

    </LayoutContent>
}