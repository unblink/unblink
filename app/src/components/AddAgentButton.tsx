import { ArkDialog } from "../ark/ArkDialog";
import { ArkTabs } from "../ark/ArkTabs";
import { For, Show, createSignal, untrack, onMount } from "solid-js";
import AgentPlusSVG from "../assets/icons/AgentPlus.svg";
import { FiCheck } from "solid-icons/fi";
import { relayFetch, setTab, fetchAgents, nodes, fetchNodes } from "../shared";
import { toaster } from "../ark/ArkToast";

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

export default function AddAgentButton() {
    const [name, setName] = createSignal("");
    const [instruction, setInstruction] = createSignal("");
    const [activeTab, setActiveTab] = createSignal("config");
    const [selectedServiceIds, setSelectedServiceIds] = createSignal<string[]>([]);

    interface ServiceWithNode {
        id: string;
        name: string;
        type: string;
        nodeId: string;
        nodeName: string | null;
    }

    const allServices = () => {
        const result: ServiceWithNode[] = [];
        for (const node of nodes()) {
            for (const service of node.services) {
                result.push({
                    id: service.id,
                    name: service.name,
                    type: service.type,
                    nodeId: node.id,
                    nodeName: node.name ?? null,
                });
            }
        }
        return result;
    };

    onMount(() => {
        fetchNodes();
    });

    const toggleService = (serviceId: string) => {
        setSelectedServiceIds((prev) =>
            prev.includes(serviceId) ? prev.filter((id) => id !== serviceId) : [...prev, serviceId]
        );
    };

    const applyTemplate = (template: typeof AGENT_TEMPLATES[0]) => {
        setName(template.name);
        setInstruction(template.instruction);
    };

    const handleSave = async (closeDialog: () => void) => {
        const _name = untrack(name).trim();
        const _instruction = untrack(instruction).trim();
        const _serviceIds = untrack(selectedServiceIds);

        if (!_name || !_instruction) {
            return;
        }

        console.log('Toaster promise start');
        toaster.promise(async () => {
            const response = await relayFetch('/agents', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                    name: _name,
                    instruction: _instruction,
                    service_ids: _serviceIds,
                }),
            });

            if (response.ok) {
                const data = await response.json();
                console.log('Agent created:', data.agent);
                setName("");
                setInstruction("");
                setSelectedServiceIds([]);
                closeDialog(); // Close dialog after success
                setTab({ type: 'agents' }); // Redirect to agents tab
                await fetchAgents(); // Refresh agents list
            } else {
                throw new Error('Failed to create agent');
            }



            console.log('Toaster promise end');
        }, {
            loading: {
                title: 'Creating...',
                description: 'Your agent is being created.',
            },
            success: {
                title: 'Success!',
                description: 'Agent has been created successfully.',
            },
            error: {
                title: 'Failed',
                description: 'There was an error creating your agent. Please try again.',
            },
        });
    };

    return (
        <ArkDialog
            trigger={(_, setOpen) => (
                <button
                    onClick={() => setOpen(true)}
                    class="w-full btn-primary flex items-center justify-center space-x-2"
                >
                    <img src={AgentPlusSVG} class="w-5 h-5" style="filter: brightness(0) invert(1)" />
                    <span>Create Agent</span>
                </button>
            )}
            title="Create a new agent"
            description="Enter the details for your new agent."
        >
            {(setOpen) => (
                <div class="mt-4 space-y-4">
                    <ArkTabs
                        items={[
                            {
                                value: "config",
                                label: "Agent Config",
                                content: (
                                    <div class="space-y-4 mt-4">
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
                                        <div>
                                            <label for="agent-name" class="text-sm font-medium text-neu-300">
                                                Agent Name
                                            </label>
                                            <input
                                                value={name()}
                                                onInput={(e) => setName(e.currentTarget.value)}
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
                                                value={instruction()}
                                                onInput={(e) => setInstruction(e.currentTarget.value)}
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
                                content: (
                                    <div class="space-y-4 mt-4">
                                        <div class="text-sm text-neu-400">
                                            Select which services this agent is allowed to read and monitor.
                                        </div>
                                        <div class="rounded-lg border border-neu-750 bg-neu-850 max-h-64 overflow-y-auto">
                                            <Show
                                                when={allServices().length > 0}
                                                fallback={
                                                    <div class="p-8 text-center text-neu-500">
                                                        <p>No services available</p>
                                                        <p class="text-xs mt-1">Connect a node with services to get started</p>
                                                    </div>
                                                }
                                            >
                                                <div class="divide-y divide-neu-750">
                                                    <For each={allServices()}>
                                                        {(service) => (
                                                            <div
                                                                onClick={() => toggleService(service.id)}
                                                                class="flex items-center gap-3 p-3 hover:bg-neu-800 cursor-pointer transition-colors"
                                                            >
                                                                <div
                                                                    class={`w-5 h-5 rounded border flex items-center justify-center transition-colors ${
                                                                        selectedServiceIds().includes(service.id)
                                                                            ? "bg-blue-600 border-blue-600"
                                                                            : "border-neu-600"
                                                                    }`}
                                                                >
                                                                    <Show when={selectedServiceIds().includes(service.id)}>
                                                                        <FiCheck class="w-3 h-3 text-white" />
                                                                    </Show>
                                                                </div>
                                                                <div class="flex-1 min-w-0">
                                                                    <p class="text-sm font-medium text-white truncate">{service.name || "Unnamed Service"}</p>
                                                                    <p class="text-xs text-neu-500">
                                                                        {service.type} • {service.nodeName || service.nodeId}
                                                                    </p>
                                                                </div>
                                                            </div>
                                                        )}
                                                    </For>
                                                </div>
                                            </Show>
                                        </div>
                                    </div>
                                ),
                            },
                        ]}
                        value={activeTab()}
                        onValueChange={setActiveTab}
                    />
                    <div class="flex justify-end pt-4">
                        <button
                            onClick={() =>
                                activeTab() === "services"
                                    ? handleSave(() => setOpen(false))
                                    : setActiveTab("services")
                            }
                            class="btn-primary"
                        >
                            {activeTab() === "services" ? "Create Agent" : "Next"}
                        </button>
                    </div>
                </div>
            )}
        </ArkDialog>
    );
}
