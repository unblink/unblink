
import { createEffect, onMount, Show } from 'solid-js';
import HistoryContent from './HistoryContent';
import HomeContent from './HomeContent';
import MomentsContent from './MomentsContent';
import SearchContent from './SearchContent';
import { conn, fetchCameras, fetchSettings, setConn, subscription, tab } from './shared';
import SideBar from './SideBar';
import { connectWebSocket } from './video/connection';
import ViewContent from './ViewContent';
import SettingsContent from './SettingsContent';
import ArkToast from './ark/ArkToast';

export default function App() {
    onMount(() => {
        const conn = connectWebSocket();
        setConn(conn);
        fetchSettings();
        fetchCameras();
    })

    createEffect(() => {
        const c = conn();
        const _subscription = subscription();
        if (!c) return;
        c.send({ type: 'set_subscription', subscription: _subscription });

    })

    return <div class="h-screen flex items-start bg-neu-925 text-white space-x-2">
        <ArkToast />
        <SideBar />
        <div class="flex-1">
            <Show when={tab().type === 'home'}>
                <HomeContent />
            </Show>
            <Show when={tab().type === 'search'}>
                <SearchContent />
            </Show>
            <Show when={tab().type === 'moments'}>
                <MomentsContent />
            </Show>
            <Show when={tab().type === 'history'}>
                <HistoryContent />
            </Show>
            <Show when={tab().type === 'view'}>
                <ViewContent />
            </Show>
            <Show when={tab().type === 'settings'}>
                <SettingsContent />
            </Show>
        </div>

    </div>;
}