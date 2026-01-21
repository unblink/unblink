import { ArkTabs } from "../ark/ArkTabs";
import { For, Show, createSignal, untrack, Setter } from "solid-js";
import { FiSettings, FiVideo } from "solid-icons/fi";
import { services, nodes } from "../shared";
import ArkSwitch from "../ark/ArkSwitch";

const AGENT_TEMPLATES = [
    {
        name: "Security",
        instruction: "Are there any suspicious activities or unauthorized persons in the area? Monitor for security concerns.",
    },
    {
        name: "Safety Equipment",
        instruction: "Are all personnel wearing required safety equipment (helmets, vests, etc.)? Check for compliance.",
    },
    {
        name: "Wellness Check",
        instruction: "Is everyone in the area safe and healthy? Look for signs of distress or medical emergencies.",
    },
];

interface ServiceWithNode {
    id: string;
    name: string;
    type: string;
    nodeId: string;
    nodeName: string | null;
}

export function AgentForm(props: {
    name: () => string;
    setName: (name: string) => void;
    instruction: () => string;
    setInstruction: (instruction: string) => void;
    selectedServiceIds: () => string[];
    setSelectedServiceIds: (ids: string[]) => void;
    showTemplates?: boolean;
    activeTab?: () => string;
    setActiveTab?: Setter<string>;
}) {
    const [localActiveTab, setLocalActiveTab] = createSignal("config");
    const activeTab = props.activeTab ?? localActiveTab;
    const setActiveTab = props.setActiveTab ?? setLocalActiveTab;

    const allServices = () => {
        const result: ServiceWithNode[] = [];
        const nodeMap = new Map(nodes().map(n => [n.id, n]));

        for (const service of services()) {
            const node = nodeMap.get(service.nodeId);
            result.push({
                id: service.id,
                name: service.name,
                type: service.status || 'unknown',
                nodeId: service.nodeId,
                nodeName: node?.hostname ?? null,
            });
        }
        return result;
    };

    const applyTemplate = (template: typeof AGENT_TEMPLATES[0]) => {
        props.setName(template.name);
        props.setInstruction(template.instruction);
    };

    return (
        <ArkTabs
            indicatorPosition="bottom"
            items={[
                {
                    value: "config",
                    label: "Agent Config",
                    icon: <FiSettings class="w-4 h-4" />,
                    content: (
                        <div class="space-y-4 mt-4">
                            <Show when={props.showTemplates}>
                                <div>
                                    <label class="text-sm font-medium text-neu-300 block mb-2">
                                        Templates
                                    </label>
                                    <div class="flex gap-2">
                                        <For each={AGENT_TEMPLATES}>
                                            {(template) => (
                                                <button
                                                    onClick={() => applyTemplate(template)}
                                                    class="text-xs flex-1 px-3 py-2 rounded-lg bg-neu-850 border border-neu-750 text-neu-300 hover:bg-neu-800 hover:border-neu-700 transition-colors truncate"
                                                >
                                                    {template.name}
                                                </button>
                                            )}
                                        </For>
                                    </div>
                                </div>
                            </Show>
                            <div>
                                <label for="agent-name" class="text-sm font-medium text-neu-300">
                                    Agent Name
                                </label>
                                <input
                                    value={props.name()}
                                    onInput={(e) => props.setName(e.currentTarget.value)}
                                    placeholder="My Agent"
                                    type="text"
                                    id="agent-name"
                                    class="px-4 py-2 mt-1 block w-full rounded-lg bg-neu-850 border border-neu-750 text-white focus:outline-none placeholder:text-neu-500"
                                />
                            </div>
                            <div>
                                <label for="agent-instruction" class="text-sm font-medium text-neu-300">
                                    Instruction
                                </label>
                                <textarea
                                    value={props.instruction()}
                                    onInput={(e) => props.setInstruction(e.currentTarget.value)}
                                    placeholder="what happened in the video?"
                                    id="agent-instruction"
                                    class="min-h-32 px-4 py-2 mt-1 block w-full rounded-lg bg-neu-850 border border-neu-750 text-white focus:outline-none placeholder:text-neu-500 resize-none"
                                    rows="3"
                                />
                            </div>
                        </div>
                    ),
                },
                {
                    value: "services",
                    label: "Services",
                    icon: <FiVideo class="w-4 h-4" />,
                    content: (
                        <div class="space-y-4 mt-4">
                            <div class="text-sm text-neu-400">
                                Select which services this agent is allowed to read and monitor.
                            </div>
                            <Show
                                when={allServices().length > 0}
                                fallback={
                                    <div class="text-center text-neu-500 py-8">
                                        <p>No services available</p>
                                        <p class="text-xs mt-1">Connect a node with services to get started</p>
                                    </div>
                                }
                            >
                                <div class="flex items-center justify-between mb-2">
                                    <span class="text-xs text-neu-500">
                                        {props.selectedServiceIds().length} of {allServices().length} selected
                                    </span>
                                    <button
                                        onClick={() => {
                                            const allSelected = props.selectedServiceIds().length === allServices().length;
                                            if (allSelected) {
                                                props.setSelectedServiceIds([]);
                                            } else {
                                                props.setSelectedServiceIds(allServices().map(s => s.id));
                                            }
                                        }}
                                        class="text-xs px-2 py-1 rounded-md bg-neu-850 border border-neu-750 text-neu-300 hover:bg-neu-800 hover:border-neu-700 transition-colors"
                                    >
                                        {props.selectedServiceIds().length === allServices().length ? "Deselect All" : "Select All"}
                                    </button>
                                </div>
                                <div class="space-y-3 max-h-64 overflow-y-auto">
                                    <For each={allServices()}>
                                        {(service) => (
                                            <div class="flex items-center justify-between py-2">
                                                <p class="text-sm font-medium text-white truncate flex-1 mr-4">
                                                    {service.name || "Unnamed Service"}
                                                </p>
                                                <ArkSwitch
                                                    checked={() => props.selectedServiceIds().includes(service.id)}
                                                    onCheckedChange={(details) => {
                                                        if (details.checked) {
                                                            props.setSelectedServiceIds([...props.selectedServiceIds(), service.id]);
                                                        } else {
                                                            props.setSelectedServiceIds(props.selectedServiceIds().filter(id => id !== service.id));
                                                        }
                                                    }}
                                                    label=""
                                                />
                                            </div>
                                        )}
                                    </For>
                                </div>
                            </Show>
                        </div>
                    ),
                },
            ]}
            value={activeTab()}
            onValueChange={(value) => setActiveTab(value)}
        />
    );
}
