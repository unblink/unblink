import { FiChevronLeft, FiChevronRight, FiEye } from 'solid-icons/fi';
import { For, Show, createSignal } from 'solid-js';
import { ArkSelect, type SelectItem } from '../ark/ArkSelect';

// Example JSON data for testing - will be replaced with real data later
const exampleCards = [
  {
    id: '1',
    agent_name: 'Person Detector',
    stream_name: 'Front Door Camera',
    data: {
      confidence: 0.95,
      detected_class: 'person',
      bbox: [100, 150, 300, 400],
      timestamp: '2025-01-15T10:30:00Z'
    }
  },
  {
    id: '2',
    agent_name: 'Vehicle Counter',
    stream_name: 'Parking Lot',
    data: {
      confidence: 0.88,
      detected_class: 'car',
      bbox: [50, 80, 200, 180],
      color: 'blue',
      timestamp: '2025-01-15T10:29:45Z'
    }
  },
  {
    id: '3',
    agent_name: 'Person Detector',
    stream_name: 'Backyard Camera',
    data: {
      confidence: 0.92,
      detected_class: 'person',
      bbox: [120, 200, 350, 500],
      timestamp: '2025-01-15T10:29:30Z'
    }
  }
];

const agentOptions: SelectItem[] = [
  { label: 'All Agents', value: 'all' },
  { label: 'Person Detector', value: 'Person Detector' },
  { label: 'Vehicle Counter', value: 'Vehicle Counter' }
];

export function useAgentPanel() {
  const [showAgentPanel, setShowAgentPanel] = createSignal(true);
  const [selectedAgent, setSelectedAgent] = createSignal('all');

  // Filter cards based on selected agent
  const filteredCards = () => {
    const selected = selectedAgent();
    if (selected === 'all') {
      return exampleCards;
    }
    return exampleCards.filter(card => card.agent_name === selected);
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
    <pre class="text-xs text-neu-300 bg-neu-900 rounded-lg p-3 overflow-x-auto font-mono">
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
                items={agentOptions}
                value={selectedAgent}
                onValueChange={(details) => setSelectedAgent(details.value[0] || 'all')}
                placeholder="Filter by agent"
                positioning={{ sameWidth: true }}
              />
            </Show>
          </div>

          <Show when={showAgentPanel()}>
            <div class="flex-1 p-2 overflow-y-auto space-y-4">
              <For each={filteredCards()}>
                {(card) => (
                  <div class="animate-push-down p-4 bg-neu-850 rounded-2xl space-y-2">
                    <div class="flex items-center justify-between">
                      <div class="font-semibold text-white">{card.stream_name}</div>
                      <Show when={card.agent_name}>
                        <div class="flex items-center gap-1.5 text-xs px-2 py-1 bg-neu-800 rounded-lg text-neu-300 font-medium">
                          <FiEye class="w-3 h-3" />
                          {card.agent_name}
                        </div>
                      </Show>
                    </div>
                    <div class="text-neu-400 text-sm">{card.id}</div>
                    <JsonDisplay data={card.data} />
                  </div>
                )}
              </For>
            </div>
          </Show>
        </div>
      </div>
    )
  };
}
