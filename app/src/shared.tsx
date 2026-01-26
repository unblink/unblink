import { createSignal } from 'solid-js'
import { toaster } from './ark/ArkToast'
import { serviceClient } from './lib/rpc'

export interface Service {
  id: string
  name: string
  nodeId: string
  serviceUrl: string
  description?: string
}

export type Tab =
  | { type: 'chat' }
  | { type: 'view'; nodeId: string; serviceId: string; name: string }

// Services state
export const [services, setServices] = createSignal<Service[]>([])

// Active tab state - default to chat
export const [activeTab, setActiveTab] = createSignal<Tab>({ type: 'chat' })

// Fetch services from server
export async function fetchServices(nodeId: string) {
  try {
    const res = await serviceClient.listServicesByNodeId({ nodeId })
    if (res.services) {
      const loadedServices: Service[] = res.services.map(s => ({
        id: s.id,
        name: s.name || s.id,
        nodeId: s.nodeId,
        serviceUrl: s.url,
      }))
      setServices(loadedServices)
    }
  } catch (error) {
    console.error('Failed to fetch services:', error)
    toaster.create({
      title: 'Failed to load services',
      description: error instanceof Error ? error.message : 'Unknown error',
      type: 'error',
    })
  }
}
