import { ArkDialog } from "../ark/ArkDialog";
import { createSignal, untrack } from "solid-js";
import AgentPlusSVG from "../assets/icons/AgentPlus.svg";
import { setTab, fetchAgents } from "../shared";
import { toaster } from "../ark/ArkToast";
import { AgentForm } from "./AgentForm";
import { agentClient } from "../lib/rpc";

const TABS = ["config", "services"] as const;

export default function AddAgentButton() {
    const [name, setName] = createSignal("");
    const [instruction, setInstruction] = createSignal("");
    const [selectedServiceIds, setSelectedServiceIds] = createSignal<string[]>([]);
    const [activeTab, setActiveTab] = createSignal<string>("config");

    const isLastTab = () => activeTab() === TABS[TABS.length - 1];

    const handleNext = () => {
        const currentIndex = TABS.indexOf(activeTab() as typeof TABS[number]);
        if (currentIndex < TABS.length - 1) {
            setActiveTab(TABS[currentIndex + 1]);
        }
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
            await agentClient.createAgent({
                name: _name,
                type: 'openai_compat',
                config: {
                    instruction: _instruction,
                },
                serviceIds: _serviceIds,
            });

            setName("");
            setInstruction("");
            setSelectedServiceIds([]);
            setActiveTab("config"); // Reset to first tab
            closeDialog(); // Close dialog after success
            setTab({ type: 'agents' }); // Redirect to agents tab
            await fetchAgents(); // Refresh agents list

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
                        activeTab={activeTab}
                        setActiveTab={setActiveTab}
                    />
                    <div class="flex justify-end pt-4">
                        <button
                            onClick={() => {
                                if (isLastTab()) {
                                    handleSave(() => setOpen(false));
                                } else {
                                    handleNext();
                                }
                            }}
                            class="btn-primary"
                        >
                            {isLastTab() ? "Create Agent" : "Next"}
                        </button>
                    </div>
                </div>
            )}
        </ArkDialog>
    );
}
