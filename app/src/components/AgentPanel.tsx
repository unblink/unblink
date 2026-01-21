import { FiChevronLeft, FiChevronRight, FiEye, FiEdit } from 'solid-icons/fi';
import { For, Show, createSignal, createEffect, type Accessor } from 'solid-js';
import { ArkSelect, type SelectItem } from '../ark/ArkSelect';
import { ArkTabs, type TabItem } from '../ark/ArkTabs';
import { ProseText } from './chat/ProseText';
import { formatDistance } from 'date-fns';
import {
  newestRTEvent,
  agents,
  nodes,
  services,
  setTab
} from '../shared';
import { agentClient } from '../lib/rpc';
import { AgentEvent } from '../../gen/unblink/agent/v1/agent_pb';
import AddAgentButton from './AddAgentButton';

export function useAgentPanel(serviceIdAccessor: Accessor<string | undefined>) {
  const [showAgentPanel, setShowAgentPanel] = createSignal(true);
  const [selectedAgent, setSelectedAgent] = createSignal('all');
  const [events, setEvents] = createSignal<AgentEvent[]>([]);

  // Listen to realtime events using signal-based pattern
  createEffect(() => {
    const msg = newestRTEvent();
    if (!msg?.event || msg.event.case !== 'agent') return;

    const agentEvent = msg.event.value;

    // Check if event belongs to current service
    const sid = serviceIdAccessor();
    if (sid && agentEvent.serviceId !== sid) return;

    setEvents(prev => [agentEvent, ...prev].slice(0, 100)); // Keep latest 100
  });

  // Request historical events when serviceId is available
  createEffect(async () => {
    const sid = serviceIdAccessor();

    // Reset events when service changes
    if (sid) {
      // Don't reset immediately to avoid flicker if just re-fetching?
      // Actually if service changes we want new events.
      // But this effect runs whenever dependencies change.
      // Dependencies: serviceIdAccessor()

      console.log('[AgentPanel] Requesting historical events for service:', sid);
      try {
        const response = await agentClient.listAgentEvents({
          serviceId: sid,
          limit: 100
        });
        setEvents(response.events);
      } catch (err) {
        console.error('[AgentPanel] Failed to fetch events:', err);
        setEvents([]);
      }
    } else {
      setEvents([]);
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
    return events().filter(event => event.agentId === selected);
  };

  // Map service_id to service name
  const getServiceName = (serviceId: string): string => {
    const serviceList = services();
    const service = serviceList.find(s => s.id === serviceId);
    if (service) {
      return service.name || serviceId;
    }
    return serviceId;
  };

  // Check if any agent is assigned to the current service
  const hasAgentForService = (): boolean => {
    const sid = serviceIdAccessor();
    if (!sid) return false;
    return agents().some(agent => agent.serviceIds.includes(sid));
  };

  // Get agents not assigned to the current service (for edit option)
  const getAgentsNotForService = () => {
    const sid = serviceIdAccessor();
    if (!sid) return agents();
    return agents().filter(agent => !agent.serviceIds.includes(sid));
  };

  const getPrimaryServiceName = (event: AgentEvent): string => {
    if (!event.serviceId) {
      return 'No service';
    }
    return getServiceName(event.serviceId);
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

  // Component to display event data with tabs
  const EventDisplay = (props: { data: unknown }) => {
    const data = props.data as Record<string, unknown> | null;
    const content = data?.content as string | undefined;
    // frame_uuids is now a native array from google.protobuf.Struct
    const frameUuids = data?.frame_uuids as string[] | undefined;
    const [expanded, setExpanded] = createSignal(false);

    const apiUrl = import.meta.env.VITE_RELAY_API_URL || '';

    const tabItems = (): TabItem[] => [
      {
        value: 'content',
        label: 'Content',
        content: (
          <Show when={content} fallback={
            <div class="text-neu-500 text-sm p-2">No content available</div>
          }>
            <div>
              <div
                class={`p-2 transition-all duration-200 ${expanded() ? '' : 'max-h-32 overflow-hidden'}`}
              >
                <ProseText content={content!} />
              </div>
              <Show when={!expanded()}>
                <button
                  onClick={() => setExpanded(true)}
                  class="w-full text-left px-2 py-1 text-sm text-violet-400 hover:text-violet-300 transition-colors focus:outline-none"
                >
                  Show more
                </button>
              </Show>
            </div>
          </Show>
        )
      },
      {
        value: 'frames',
        label: 'Frames',
        content: (
          <Show when={frameUuids && frameUuids.length > 0} fallback={
            <div class="text-neu-500 text-sm p-2">No frames available</div>
          }>
            <div class="p-2 grid grid-cols-2 gap-2">
              <For each={frameUuids!}>
                {(uuid) => (
                  <div class="aspect-square bg-neu-900 rounded-lg overflow-hidden border border-neu-700 hover:border-violet-500 transition-colors group relative">
                    <img
                      src={`${apiUrl}/storage/${uuid}`}
                      alt={`Frame ${uuid}`}
                      class="w-full h-full object-cover"
                      loading="lazy"
                    />
                  </div>
                )}
              </For>
            </div>
          </Show>
        )
      },
      {
        value: 'raw',
        label: 'JSON',
        content: (
          <pre class="text-xs text-neu-300 whitespace-pre-wrap break-words font-mono p-2">
            {JSON.stringify(props.data, null, 2)}
          </pre>
        )
      }
    ];

    return <ArkTabs items={tabItems()} defaultValue="content" indicatorPosition="bottom" />;
  };

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
              <Show when={hasAgentForService()} fallback={
                <div class="text-center py-8 bg-neu-850 rounded-xl border border-neu-750">
                  <p class="">No agent assigned to this service</p>
                  <p class="text-xs mt-1 mb-4 text-neu-500 ">Assign an agent to start processing events</p>
                  <div class="flex flex-col gap-2 items-center max-w-xs mx-auto">
                    <AddAgentButton />
                    <Show when={getAgentsNotForService().length > 0}>
                      <button
                        onClick={() => setTab({ type: 'agents' })}
                        class="w-full btn-secondary flex items-center justify-center space-x-2"
                      >
                        <FiEdit class="w-4 h-4" />
                        <span>Edit Existing Agent</span>
                      </button>
                    </Show>
                  </div>
                </div>
              }>
                <Show when={filteredEvents().length > 0} fallback={
                  <div class="text-center text-neu-500 py-8">
                    Waiting for events...
                  </div>
                }>
                  <For each={filteredEvents()}>
                    {(event) => (
                      <div class="animate-push-down p-4 bg-neu-850 rounded-2xl space-y-2">
                        <div class="flex items-center justify-between">
                          <div class="font-semibold text-white">{getPrimaryServiceName(event)}</div>
                          <Show when={event.agentName}>
                            <div class="flex items-center gap-1.5 text-xs px-2 py-1 bg-neu-800 rounded-lg text-neu-300 font-medium">
                              <FiEye class="w-3 h-3" />
                              {event.agentName}
                            </div>
                          </Show>
                        </div>
                        <div class="text-neu-400 text-sm">
                          {formatDistance(new Date(Number(event.createdAt) * 1000), new Date(), {
                            addSuffix: true,
                            includeSeconds: true
                          })}
                        </div>
                        <EventDisplay data={event.data} />
                      </div>
                    )}
                  </For>
                </Show>
              </Show>
            </div>
          </Show>
        </div>
      </div>
    )
  };
}
