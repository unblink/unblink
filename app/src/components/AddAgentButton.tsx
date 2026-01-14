import { ArkDialog } from "../ark/ArkDialog";
import { createSignal, untrack } from "solid-js";
import AgentPlusSVG from "../assets/icons/AgentPlus.svg";
import { relayFetch, setTab, fetchAgents } from "../shared";
import { toaster } from "../ark/ArkToast";
import { AgentForm } from "./AgentForm";

export default function AddAgentButton() {
    const [name, setName] = createSignal("");
    const [instruction, setInstruction] = createSignal("");
    const [selectedServiceIds, setSelectedServiceIds] = createSignal<string[]>([]);

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
                    worker_id: 'unblink/base-vl',
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
                    <AgentForm
                        name={name}
                        setName={setName}
                        instruction={instruction}
                        setInstruction={setInstruction}
                        selectedServiceIds={selectedServiceIds}
                        setSelectedServiceIds={setSelectedServiceIds}
                        showTemplates={true}
                    />
                    <div class="flex justify-end pt-4">
                        <button
                            onClick={() => handleSave(() => setOpen(false))}
                            class="btn-primary"
                        >
                            Create Agent
                        </button>
                    </div>
                </div>
            )}
        </ArkDialog>
    );
}
