import { FiChevronLeft, FiChevronRight, FiEye } from 'solid-icons/fi';
import { For, Show, createSignal, onMount, onCleanup } from 'solid-js';
import { ArkSelect, type SelectItem } from '../ark/ArkSelect';
import { formatDistance } from 'date-fns';
import {
  agents,
  nodes,
  type AgentEvent
} from '../shared';

// Demo mode: fake agent events data
const DEMO_MODE = true;

const createFakeEvents = (): AgentEvent[] => [
  {
    id: '45a6df4b-9e93-4440-af6e-570819056a6a',
    agent_id: '17aaa780-6d7d-4c71-93b8-7aa9f22ef618',
    agent_name: 'Wellness Check',
    service_ids: ['d56c306c-567b-40ed-90b3-a418d225d017'],
    created_at: new Date(Date.now() - 5000).toISOString(),
    data: {
      status: 'safe',
      people_detected: 3,
      summary: 'All individuals appear safe. No signs of distress detected.',
      confidence: 0.94
    }
  },
  {
    id: 'e1234567-aaaa-4444-bbbb-ccccddddeeee',
    agent_id: '55ddd012-9a0a-7f04-c6eb-0dd2h55hi941',
    agent_name: 'PPE Compliance',
    service_ids: ['a1b2c3d4-3333-4000-8000-000000000003'],
    created_at: new Date(Date.now() - 8000).toISOString(),
    data: {
      compliant: true,
      workers_detected: 4,
      ppe_status: {
        hard_hats: 4,
        safety_vests: 4,
        gloves: 3
      },
      violations: []
    }
  },
  {
    id: 'f8c91234-1234-4567-890a-bcdef1234567',
    agent_id: '17aaa780-6d7d-4c71-93b8-7aa9f22ef618',
    agent_name: 'Wellness Check',
    service_ids: ['f6b26572-dc04-42be-9681-c7214701d382'],
    created_at: new Date(Date.now() - 15000).toISOString(),
    data: {
      status: 'attention',
      people_detected: 1,
      summary: 'One person detected moving slowly. Monitoring for potential assistance needed.',
      confidence: 0.78
    }
  },
  {
    id: 'f2345678-bbbb-5555-cccc-ddddeeeeffff',
    agent_id: '66eee123-0b1b-8g15-d7fc-1ee3i66ij052',
    agent_name: 'Vehicle Counter',
    service_ids: ['f6b26572-dc04-42be-9681-c7214701d382'],
    created_at: new Date(Date.now() - 22000).toISOString(),
    data: {
      vehicles_in_lot: 23,
      capacity: 50,
      occupancy_percent: 46,
      recent_entries: 2,
      recent_exits: 1
    }
  },
  {
    id: 'a1b2c3d4-5678-90ab-cdef-111122223333',
    agent_id: '28bbb891-7e8e-5d82-a4c9-8bb0f33fg729',
    agent_name: 'Motion Detection',
    service_ids: ['a1b2c3d4-1111-4000-8000-000000000001'],
    created_at: new Date(Date.now() - 45000).toISOString(),
    data: {
      motion_detected: true,
      zones: ['entrance', 'loading_bay'],
      activity_level: 'high',
      object_count: 2
    }
  },
  {
    id: 'g3456789-cccc-6666-dddd-eeeeffff0000',
    agent_id: '77fff234-1c2c-9h26-e8gd-2ff4j77jk163',
    agent_name: 'Forklift Tracker',
    service_ids: ['a1b2c3d4-1111-4000-8000-000000000001'],
    created_at: new Date(Date.now() - 60000).toISOString(),
    data: {
      forklifts_active: 3,
      forklifts_idle: 1,
      near_miss_events: 0,
      zone_violations: [],
      avg_speed_mph: 4.2
    }
  },
  {
    id: 'h4567890-dddd-7777-eeee-ffff00001111',
    agent_id: '55ddd012-9a0a-7f04-c6eb-0dd2h55hi941',
    agent_name: 'PPE Compliance',
    service_ids: ['a1b2c3d4-4444-4000-8000-000000000004'],
    created_at: new Date(Date.now() - 90000).toISOString(),
    data: {
      compliant: false,
      workers_detected: 6,
      ppe_status: {
        hard_hats: 5,
        safety_vests: 6,
        gloves: 4
      },
      violations: ['Worker near welding station missing face shield']
    }
  },
  {
    id: 'b2c3d4e5-6789-01bc-def0-222233334444',
    agent_id: '17aaa780-6d7d-4c71-93b8-7aa9f22ef618',
    agent_name: 'Wellness Check',
    service_ids: ['a1b2c3d4-2222-4000-8000-000000000002'],
    created_at: new Date(Date.now() - 120000).toISOString(),
    data: {
      status: 'safe',
      people_detected: 5,
      summary: 'Production area clear. Workers following safety protocols.',
      confidence: 0.91
    }
  },
  {
    id: 'i5678901-eeee-8888-ffff-000011112222',
    agent_id: '88ggg345-2d3d-0i37-f9he-3gg5k88kl274',
    agent_name: 'Quality Inspector',
    service_ids: ['a1b2c3d4-2222-4000-8000-000000000002'],
    created_at: new Date(Date.now() - 180000).toISOString(),
    data: {
      items_inspected: 147,
      defects_found: 2,
      defect_rate: 0.014,
      defect_types: ['surface_scratch', 'alignment_offset'],
      line_efficiency: 0.96
    }
  },
  {
    id: 'j6789012-ffff-9999-0000-111122223333',
    agent_id: '99hhh456-3e4e-1j48-g0if-4hh6l99lm385',
    agent_name: 'Spill Detection',
    service_ids: ['a1b2c3d4-3333-4000-8000-000000000003'],
    created_at: new Date(Date.now() - 240000).toISOString(),
    data: {
      spill_detected: false,
      floor_status: 'clear',
      last_incident: null,
      monitored_zones: ['assembly_line', 'chemical_storage', 'loading_dock']
    }
  },
  {
    id: 'n0123456-3333-dddd-4444-555566667777',
    agent_id: '17aaa780-6d7d-4c71-93b8-7aa9f22ef618',
    agent_name: 'Wellness Check',
    service_ids: ['a1b2c3d4-3333-4000-8000-000000000003'],
    created_at: new Date(Date.now() - 270000).toISOString(),
    data: {
      status: 'safe',
      people_detected: 2,
      summary: '2 workers actively operating on the mask production line. All safety protocols observed.',
      confidence: 0.97
    }
  },
  {
    id: 'c3d4e5f6-7890-12cd-ef01-333344445555',
    agent_id: '39ccc902-8f9f-6e93-b5da-9cc1g44gh830',
    agent_name: 'Equipment Monitor',
    service_ids: ['a1b2c3d4-4444-4000-8000-000000000004'],
    created_at: new Date(Date.now() - 300000).toISOString(),
    data: {
      equipment_status: 'operational',
      temperature: 'normal',
      alerts: [],
      uptime_hours: 142.5
    }
  },
  {
    id: 'k7890123-0000-aaaa-1111-222233334444',
    agent_id: '00iii567-4f5f-2k59-h1jg-5ii7m00mn496',
    agent_name: 'Crowd Density',
    service_ids: ['d56c306c-567b-40ed-90b3-a418d225d017'],
    created_at: new Date(Date.now() - 360000).toISOString(),
    data: {
      current_count: 12,
      max_capacity: 100,
      density_level: 'low',
      hotspots: [],
      flow_direction: 'scattered'
    }
  },
  {
    id: 'l8901234-1111-bbbb-2222-333344445555',
    agent_id: '28bbb891-7e8e-5d82-a4c9-8bb0f33fg729',
    agent_name: 'Motion Detection',
    service_ids: ['d56c306c-567b-40ed-90b3-a418d225d017'],
    created_at: new Date(Date.now() - 420000).toISOString(),
    data: {
      motion_detected: false,
      zones: ['main_entrance', 'lobby'],
      activity_level: 'low',
      last_movement: '7 minutes ago'
    }
  },
  {
    id: 'm9012345-2222-cccc-3333-444455556666',
    agent_id: '11jjj678-5g6g-3l60-i2kh-6jj8n11no507',
    agent_name: 'Fire Safety',
    service_ids: ['a1b2c3d4-4444-4000-8000-000000000004'],
    created_at: new Date(Date.now() - 500000).toISOString(),
    data: {
      fire_detected: false,
      smoke_detected: false,
      exit_paths_clear: true,
      extinguisher_visible: true,
      thermal_anomalies: []
    }
  }
];

export function useAgentPanel(_serviceId?: string) {
  const [showAgentPanel, setShowAgentPanel] = createSignal(true);
  const [selectedAgent, setSelectedAgent] = createSignal('all');
  const [events, setEvents] = createSignal<AgentEvent[]>([]);

  onMount(() => {
    if (DEMO_MODE) {
      // Load fake events for demo
      setEvents(createFakeEvents());

      // Simulate new events arriving periodically
      const interval = setInterval(() => {
        const newEvent: AgentEvent = {
          id: crypto.randomUUID(),
          agent_id: '17aaa780-6d7d-4c71-93b8-7aa9f22ef618',
          agent_name: 'Wellness Check',
          service_ids: ['d56c306c-567b-40ed-90b3-a418d225d017'],
          created_at: new Date().toISOString(),
          data: {
            status: Math.random() > 0.3 ? 'safe' : 'attention',
            people_detected: Math.floor(Math.random() * 5) + 1,
            summary: Math.random() > 0.3
              ? 'All individuals appear safe. No signs of distress detected.'
              : 'Monitoring situation. Possible assistance may be needed.',
            confidence: 0.85 + Math.random() * 0.14
          }
        };
        setEvents(prev => [newEvent, ...prev].slice(0, 100));
      }, 8000); // New event every 8 seconds

      onCleanup(() => clearInterval(interval));
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
    <pre class="text-xs text-neu-300 whitespace-pre-wrap break-words font-mono">
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
