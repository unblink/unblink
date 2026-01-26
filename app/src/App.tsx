import { Suspense, Show, onMount } from 'solid-js'
import { Authenticated } from './components/Authenticated'
import ChatView from './components/ChatView'
import VideoTile from './components/VideoTile'
import SideBar from './components/SideBar'
import ArkToast from './ark/ArkToast'
import { fetchServices, services, activeTab } from './shared'

function App() {
  // Parse node ID from URL path: /node/{nodeId}
  const getNodeIdFromPath = () => {
    const path = window.location.pathname
    const match = path.match(/^\/node\/([^/]+)/)
    return match ? match[1] : null
  }

  const nodeId = getNodeIdFromPath()

  // Fetch services on mount
  onMount(() => {
    if (nodeId) {
      fetchServices(nodeId)
    }
  })

  return (
    <div class="h-[100dvh] bg-black text-white">
      <ArkToast />
      <Suspense fallback={<div class="flex h-[100dvh] items-center justify-center text-white">Loading...</div>}>
        <Authenticated>
          <Show
            when={nodeId}
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
              <SideBar nodeId={nodeId!} />

              {/* Main Content Area */}
              <div class="flex-1 h-full">
                {(() => {
                  const tab = activeTab()
                  if (tab.type === 'chat') return <ChatView />
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
        </Authenticated>
      </Suspense>
    </div>
  )
}

export default App
