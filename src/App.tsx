
import { createEffect, onMount, Show } from 'solid-js';
import HistoryContent from './HistoryContent';
import HomeContent from './HomeContent';
import MomentsContent from './MomentsContent';
import SearchContent from './SearchContent';
import { conn, setConn, subscription, tabId } from './shared';
import SideBar from './SideBar';
import { connectWebSocket } from './video/connection';
import ViewContent from './ViewContent';

export default function App() {
    onMount(() => {
        const conn = connectWebSocket();
        setConn(conn);
    })

    createEffect(() => {
        const c = conn();
        const _subscription = subscription();
        if (!c) return;
        c.send({ type: 'set_subscription', subscription: _subscription });

    })

    return <div class="h-screen flex items-start bg-neu-925 text-white space-x-2">
        <SideBar />
        <div class="flex-1">
            <Show when={tabId() === 'home'}>
                <HomeContent />
            </Show>
            <Show when={tabId() === 'search'}>
                <SearchContent />
            </Show>
            <Show when={tabId() === 'moments'}>
                <MomentsContent />
            </Show>
            <Show when={tabId() === 'history'}>
                <HistoryContent />
            </Show>
            <Show when={tabId() === 'view'}>
                <ViewContent />
            </Show>
        </div>

    </div>;
}