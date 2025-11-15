
import { createEffect, onMount, type ValidComponent } from 'solid-js';
import { Dynamic } from 'solid-js/web';
import ArkToast from './ark/ArkToast';
import Authed from './Authed';
import HistoryContent from './content/HistoryContent';
import HomeContent from './content/HomeContent';
import MomentsContent from './content/MomentsContent';
import SearchContent from './content/SearchContent';
import SearchResultContent from './content/SearchResultContent';
import SettingsContent from './content/SettingsContent';
import { conn, fetchCameras, setAgentCards, setConn, subscription, tab, type Tab } from './shared';
import SideBar from './SideBar';
import { connectWebSocket, newMessage } from './video/connection';
import ViewContent from './ViewContent';

export default function App() {
    onMount(() => {
        const conn = connectWebSocket();
        setConn(conn);
        fetchCameras();
    })

    createEffect(() => {
        const m = newMessage();
        if (!m) return;

        if (m.type === 'agent_card') {
            // console.log('Received description for stream', m.stream_id, ':', m.description);
            setAgentCards(prev => {
                return [...prev, m.media_unit].slice(-200);
            });
        }
    })

    createEffect(() => {
        const c = conn();
        const _subscription = subscription();
        if (!c) return;
        c.send({ type: 'set_subscription', subscription: _subscription });

    })

    const components = (): Record<Tab['type'], ValidComponent> => {
        return {
            'home': HomeContent,
            'moments': MomentsContent,
            'view': ViewContent,
            'history': HistoryContent,
            'search': SearchContent,
            'search_result': SearchResultContent,
            'settings': SettingsContent,
        }

    }
    const component = () => components()[tab().type]

    return <Authed>
        <div class="h-screen flex items-start bg-neu-925 text-white space-x-2">
            <ArkToast />
            <SideBar />
            <div class="flex-1">
                <Dynamic component={component()} />
            </div>
        </div>
    </Authed>
}