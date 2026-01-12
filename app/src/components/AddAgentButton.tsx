import { ArkDialog } from "../ark/ArkDialog";
import { ArkSelect, type SelectItem } from "../ark/ArkSelect";
import { For, createSignal, untrack } from "solid-js";
import AgentPlusSVG from "../assets/icons/AgentPlus.svg";
import { relayFetch, setTab, fetchAgents } from "../shared";
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

const WORKER_OPTIONS: SelectItem[] = [
    { label: "unblink/qwen3-vl", value: "unblink/qwen3-vl" },
    { label: "unblink/llama3-vision", value: "unblink/llama3-vision" },
    { label: "unblink/gpt-4o", value: "unblink/gpt-4o" },
    { label: "unblink/claude-sonnet", value: "unblink/claude-sonnet" },
];

export default function AddAgentButton() {
    const [name, setName] = createSignal("");
    const [instruction, setInstruction] = createSignal("");
    const [worker, setWorker] = createSignal("unblink/qwen3-vl");

    const applyTemplate = (template: typeof AGENT_TEMPLATES[0]) => {
        setName(template.name);
        setInstruction(template.instruction);
    };

    const handleSave = async (closeDialog: () => void) => {
        const _name = untrack(name).trim();
        const _instruction = untrack(instruction).trim();
        const _worker = untrack(worker);

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
                    worker: _worker,
                }),
            });

            if (response.ok) {
                const data = await response.json();
                console.log('Agent created:', data.agent);
                setName("");
                setInstruction("");
                closeDialog(); // Close dialog after success
                setTab({ type: 'agents' }); // Redirect to agents tab
                await fetchAgents(); // Refresh the agents list
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
                        <label class="text-sm font-medium text-neu-300 block mb-2">
                            Worker
                        </label>
                        <ArkSelect
                            items={WORKER_OPTIONS}
                            value={worker}
                            onValueChange={(details) => setWorker(details.value[0])}
                            placeholder="Select a worker..."
                            disabled={true}
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
                            class="min-h-52 px-4 py-2 mt-1 block w-full rounded-lg bg-neu-850 border border-neu-750 text-white focus:outline-none placeholder:text-neu-500 resize-none"
                            rows="3"
                        />
                    </div>
                    <div class="flex justify-end pt-4">
                        <button onClick={() => handleSave(() => setOpen(false))} class="btn-primary">
                            Create Agent
                        </button>
                    </div>
                </div>
            )}
        </ArkDialog>
    );
}
