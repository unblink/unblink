import { Show } from "solid-js";
import { nodes } from "../shared";

interface ServiceFormProps {
    selectedNodeId: () => string;
    setSelectedNodeId: (id: string) => void;
    name: () => string;
    setName: (name: string) => void;
    description: () => string;
    setDescription: (description: string) => void;
    serviceUrl: () => string;
    setServiceUrl: (url: string) => void;
}

// URL format examples for help text
const URL_EXAMPLES = [
    { type: "RTSP with auth", url: "rtsp://admin:password@192.168.1.100:554/stream" },
    { type: "RTSP without auth", url: "rtsp://192.168.1.100:554/stream" },
    { type: "HTTP with auth", url: "http://admin:pass@192.168.1.100:8080/video" },
    { type: "HTTP without auth", url: "http://192.168.1.100:8080/video" },
];

export function ServiceForm(props: ServiceFormProps) {
    const allNodes = () => nodes();

    return (
        <div class="space-y-6">
            {/* Node Selection Section */}
            <div>
                <h3 class="text-sm font-semibold text-neu-200 mb-3">Select Node</h3>
                <Show
                    when={allNodes().length > 0}
                    fallback={
                        <div class="text-center text-neu-500 py-8 bg-neu-850 rounded-lg border border-neu-750">
                            <p>No nodes available</p>
                            <p class="text-xs mt-1">Connect a node to get started</p>
                        </div>
                    }
                >
                    <div>
                        <label class="text-sm font-medium text-neu-300 block mb-2">
                            Node
                        </label>
                        <select
                            value={props.selectedNodeId()}
                            onInput={(e) => props.setSelectedNodeId(e.currentTarget.value)}
                            class="w-full px-4 py-2 bg-neu-850 border border-neu-750 rounded-lg text-white focus:outline-none"
                        >
                            <option value="">Select a node...</option>
                            {allNodes().map(n => (
                                <option value={n.id}>{n.hostname || n.id}</option>
                            ))}
                        </select>
                    </div>
                </Show>
            </div>

            {/* Service Configuration Section */}
            <Show when={allNodes().length > 0}>
                <div>
                    <h3 class="text-sm font-semibold text-neu-200 mb-3">Service Configuration</h3>
                    <div class="space-y-4">
                    <div>
                        <label for="service-name" class="text-sm font-medium text-neu-300">
                            Service Name
                        </label>
                        <input
                            value={props.name()}
                            onInput={(e) => props.setName(e.currentTarget.value)}
                            placeholder="My Service"
                            type="text"
                            id="service-name"
                            class="px-4 py-2 mt-1 block w-full rounded-lg bg-neu-850 border border-neu-750 text-white focus:outline-none placeholder:text-neu-500"
                        />
                    </div>

                    {/* NEW: Single Service URL field */}
                    <div>
                        <label for="service-url" class="text-sm font-medium text-neu-300">
                            Service URL
                        </label>
                        <input
                            value={props.serviceUrl()}
                            onInput={(e) => props.setServiceUrl(e.currentTarget.value)}
                            placeholder="rtsp://admin:password@192.168.1.100:554/stream"
                            type="text"
                            id="service-url"
                            class="px-4 py-2 mt-1 block w-full rounded-lg bg-neu-850 border border-neu-750 text-white focus:outline-none placeholder:text-neu-500 font-mono text-sm"
                        />
                        <p class="text-xs text-neu-500 mt-1">
                            Enter the complete URL for your service (rtsp://, https://, etc.)
                        </p>
                    </div>

                    {/* URL Examples / Help */}
                    <div class="bg-neu-900 rounded-lg border border-neu-750 p-3 overflow-x-auto">
                        <p class="text-xs font-medium text-neu-300 mb-2">URL Format Examples:</p>
                        <div class="space-y-1 min-w-max">
                            {URL_EXAMPLES.map(example => (
                                <div class="flex items-start gap-2">
                                    <span class="text-xs text-neu-400 w-32 flex-shrink-0">{example.type}:</span>
                                    <code class="text-xs text-neu-500 font-mono whitespace-nowrap">{example.url}</code>
                                </div>
                            ))}
                        </div>
                    </div>

                    <div>
                        <label for="service-description" class="text-sm font-medium text-neu-300">
                            Description <span class="text-neu-500">(optional)</span>
                        </label>
                        <textarea
                            value={props.description()}
                            onInput={(e) => props.setDescription(e.currentTarget.value)}
                            placeholder="A brief description of this service"
                            id="service-description"
                            class="min-h-24 px-4 py-2 mt-1 block w-full rounded-lg bg-neu-850 border border-neu-750 text-white focus:outline-none placeholder:text-neu-500 resize-none"
                            rows="2"
                        />
                    </div>
                </div>
            </div>
            </Show>
        </div>
    );
}
