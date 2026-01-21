import { ArkDialog } from "../ark/ArkDialog";
import { createSignal, untrack } from "solid-js";
import ServicePlusSVG from "../../public/icons/ServicePlus.svg";
import { setTab, fetchServices } from "../shared";
import { toaster } from "../ark/ArkToast";
import { ServiceForm } from "./ServiceForm";
import { serviceClient } from "../lib/rpc";

export default function AddServiceButton() {
    const [selectedNodeId, setSelectedNodeId] = createSignal<string>("");
    const [name, setName] = createSignal("");
    const [description, setDescription] = createSignal("");
    const [serviceUrl, setServiceUrl] = createSignal("");

    const handleSave = async (closeDialog: () => void) => {
        const _nodeId = untrack(selectedNodeId);
        const _name = untrack(name).trim();
        const _description = untrack(description).trim();
        const _serviceUrl = untrack(serviceUrl).trim();

        if (!_nodeId || !_name || !_serviceUrl) {
            return;
        }

        toaster.promise(async () => {
            await serviceClient.createService({
                name: _name,
                description: _description,
                nodeId: _nodeId,
                serviceUrl: _serviceUrl,
                tags: [],
            });

            // Reset form
            setSelectedNodeId("");
            setName("");
            setDescription("");
            setServiceUrl("");
            closeDialog();
            await fetchServices(); // Refresh services list to show new service
        }, {
            loading: {
                title: 'Creating...',
                description: 'Your service is being created.',
            },
            success: {
                title: 'Success!',
                description: 'Service has been created successfully.',
            },
            error: {
                title: 'Failed',
                description: 'There was an error creating your service. Please try again.',
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
                    <img src={ServicePlusSVG} class="w-5 h-5" style="filter: brightness(0) invert(1)" />
                    <span>Add Service</span>
                </button>
            )}
            title="Add a new service"
            description="Select a node and configure your service."
        >
            {(setOpen) => (
                <div class="mt-4 space-y-4">
                    <ServiceForm
                        selectedNodeId={selectedNodeId}
                        setSelectedNodeId={setSelectedNodeId}
                        name={name}
                        setName={setName}
                        description={description}
                        setDescription={setDescription}
                        serviceUrl={serviceUrl}
                        setServiceUrl={setServiceUrl}
                    />
                    <div class="flex justify-end pt-4">
                        <button
                            onClick={() => handleSave(() => setOpen(false))}
                            class="btn-primary"
                        >
                            Add Service
                        </button>
                    </div>
                </div>
            )}
        </ArkDialog>
    );
}
