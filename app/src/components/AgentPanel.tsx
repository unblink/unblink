import { FiChevronLeft, FiChevronRight, FiEye } from 'solid-icons/fi';
import { For, Show, createSignal, onMount, onCleanup, createEffect } from 'solid-js';
import { ArkSelect, type SelectItem } from '../ark/ArkSelect';
import { formatDistance } from 'date-fns';
import {
  connectAgentEventsWebSocket,
  subscribeToAgentEvents,
  disconnectAgentEventsWebSocket,
  requestAgentEvents,
  agents,
  nodes,
  type AgentEvent
} from '../shared';

export function useAgentPanel(serviceId?: string) {
  const [showAgentPanel, setShowAgentPanel] = createSignal(true);
  const [selectedAgent, setSelectedAgent] = createSignal('all');
  const [events, setEvents] = createSignal<AgentEvent[]>([]);
  const [requestedHistory, setRequestedHistory] = createSignal(false);

  onMount(() => {
    // Connect to WebSocket
    connectAgentEventsWebSocket();

    // Subscribe to events
    const unsubscribe = subscribeToAgentEvents((event) => {
      setEvents(prev => [event, ...prev].slice(0, 100)); // Keep latest 100
    });

    onCleanup(() => {
      unsubscribe();
      disconnectAgentEventsWebSocket();
    });
  });

  // Request historical events when serviceId is available
  createEffect(() => {
    if (serviceId && !requestedHistory()) {
      // Wait a bit for WebSocket to be ready
      setTimeout(() => {
        console.log('[AgentPanel] Requesting historical events for service:', serviceId);
        requestAgentEvents({ service_id: serviceId, limit: 100 });
        setRequestedHistory(true);
      }, 500);
    }
  });

  // Build agent filter options from real agents
  const agentOptions = (): SelectItem[] => {
    const agentList = agents();
    return [
      { label: 'All Agents', value: 'all' },
      ...agentList.map(agent => ({
        label: agent.name,
        value: agent.id
      }))
    ];
  };

  // Filter events based on selected agent
  const filteredEvents = () => {
    const selected = selectedAgent();
    if (selected === 'all') {
      return events();
    }
    return events().filter(event => event.agent_id === selected);
  };

  // Map service_id to service name
  const getServiceName = (serviceId: string): string => {
    const nodeList = nodes();
    for (const node of nodeList) {
      const service = node.services.find(s => s.id === serviceId);
      if (service) {
        return service.name || serviceId;
      }
    }
    return serviceId;
  };

  const getPrimaryServiceName = (event: AgentEvent): string => {
    if (!event.service_ids || event.service_ids.length === 0) {
      return 'No service';
    }
    return getServiceName(event.service_ids[0]);
  };

  const Toggle = () => (
    <button
      onClick={() => setShowAgentPanel(prev => !prev)}
      class="btn-small"
    >
      <Show when={showAgentPanel()} fallback={
        <>
          <FiChevronLeft class="w-4 h-4" />
          <div>Events</div>
        </>
      }>
        <FiChevronRight class="w-4 h-4" />
      </Show>
    </button>
  );

  // Component to display JSON data nicely formatted
  const JsonDisplay = (props: { data: unknown }) => (
    <pre class="text-xs text-neu-300 overflow-x-auto font-mono">
      {JSON.stringify(props.data, null, 2)}
    </pre>
  );

  return {
    showAgentPanel,
    setShowAgentPanel,
    Toggle,
    Comp: () => (
      <div
        data-show={showAgentPanel()}
        class="flex-none data-[show=true]:w-[400px] w-0 h-screen transition-[width] duration-300 ease-in-out overflow-hidden flex flex-col"
      >
        <div class="border-l border-neu-800 bg-neu-900 shadow-2xl rounded-2xl flex-1 mr-2 my-2 flex flex-col h-full overflow-hidden">
          <div class="h-14 flex items-center gap-2 p-2">
            <Toggle />
            <Show when={showAgentPanel()}>
              <ArkSelect
                items={agentOptions()}
                value={selectedAgent}
                onValueChange={(details) => setSelectedAgent(details.value[0] || 'all')}
                placeholder="Filter by agent"
                positioning={{ sameWidth: true }}
              />
            </Show>
          </div>

          <Show when={showAgentPanel()}>
            <div class="flex-1 p-2 overflow-y-auto space-y-4">
              <Show when={filteredEvents().length > 0} fallback={
                <div class="text-center text-neu-500 py-8">
                  No events yet
                </div>
              }>
                <For each={filteredEvents()}>
                  {(event) => (
                    <div class="animate-push-down p-4 bg-neu-850 rounded-2xl space-y-2">
                      <div class="flex items-center justify-between">
                        <div class="font-semibold text-white">{getPrimaryServiceName(event)}</div>
                        <Show when={event.agent_name}>
                          <div class="flex items-center gap-1.5 text-xs px-2 py-1 bg-neu-800 rounded-lg text-neu-300 font-medium">
                            <FiEye class="w-3 h-3" />
                            {event.agent_name}
                          </div>
                        </Show>
                      </div>
                      <div class="text-neu-400 text-sm">
                        {formatDistance(new Date(event.created_at), new Date(), {
                          addSuffix: true,
                          includeSeconds: true
                        })}
                      </div>
                      <JsonDisplay data={event.data} />
                    </div>
                  )}
                </For>
              </Show>
            </div>
          </Show>
        </div>
      </div>
    )
  };
}
