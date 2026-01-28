import { onMount, Show } from 'solid-js'
import { fetchServices, services, activeTab } from '../shared'
import ChatView from './ChatView'
import VideoTile from './VideoTile'
import SideBar from './SideBar'
import SettingsView from './SettingsView'

interface MainProps {
  nodeId: string
}

export default function Main(props: MainProps) {
  // Fetch services on mount - only runs after auth is complete
  onMount(() => {
    fetchServices(props.nodeId)
  })

  return (
    <Show
      when={props.nodeId}
      fallback={
        <div class="flex h-full items-center justify-center">
          <div class="text-center">
            <p class="text-gray-400">No node ID in URL</p>
            <p class="text-sm text-gray-500 mt-2">Navigate to /node/YOUR_NODE_ID</p>
          </div>
        </div>
      }
    >
      <div class="flex items-start h-full">
        <SideBar nodeId={props.nodeId} />

        {/* Main Content Area */}
        <div class="flex-1 h-full">
          {(() => {
            const tab = activeTab()
            if (tab.type === 'chat') return <ChatView />
            if (tab.type === 'settings') return <SettingsView nodeId={props.nodeId} />
            const service = services().find((s) => s.id === tab.serviceId)
            if (!service) return <ChatView />
            return (
              <VideoTile
                nodeId={tab.nodeId}
                serviceId={tab.serviceId}
                serviceUrl={service.serviceUrl}
                name={tab.name}
              />
            )
          })()}
        </div>
      </div>
    </Show>
  )
}
