import { Dialog } from '@ark-ui/solid/dialog';
import { ArkDialog } from './ark/ArkDialog';
import { createSignal, untrack, onMount, Show } from 'solid-js';
import { fetchCameras, type Camera } from './shared';
import { FiEdit } from 'solid-icons/fi';
import { Switch } from '@ark-ui/solid';


export default function EditCameraButton(props: { camera: Camera, children: any }) {
    const [name, setName] = createSignal('');
    const [uri, setUri] = createSignal('');
    const [labels, setLabels] = createSignal('');
    const [saveToDisk, setSaveToDisk] = createSignal(false);
    const [saveDir, setsaveDir] = createSignal('');

    onMount(() => {
        setName(props.camera.name);
        setUri(props.camera.uri);
        setLabels(props.camera.labels.join(', '));
        setSaveToDisk(props.camera.saveToDisk || false);
        setsaveDir(props.camera.saveDir || '');
    });

    const handleSave = async () => {
        const _name = untrack(name).trim();
        const _uri = untrack(uri).trim();
        const _saveDir = untrack(saveDir).trim();
        const _saveToDisk = untrack(saveToDisk);
        if (!_name || !_uri) {
            return;
        }

        const labelsArray = labels().split(',').map(l => l.trim()).filter(l => l);

        try {
            const response = await fetch(`/media/${props.camera.id}`, {
                method: 'PUT',
                headers: {
                    'Content-Type': 'application/json',
                },
                body: JSON.stringify({
                    name: _name,
                    uri: _uri,
                    labels: labelsArray,
                    saveToDisk: _saveToDisk,
                    saveDir: _saveDir,
                }),
            });

            if (response.ok) {
                fetchCameras();
            } else {
                console.error('Failed to save camera');
            }
        } catch (error) {
            console.error('Error saving camera:', error);
        }
    };

    return <ArkDialog
        trigger={(_, setOpen) => <button
            onClick={() => setOpen(true)}
            class="btn-primary">
            {props.children}
        </button>}
        title="Edit camera"
        description="Enter the details for your new camera."
    >
        <div class="mt-4 space-y-4">
            <div>
                <label for="camera-name" class="text-sm font-medium text-neu-300">Camera Name</label>
                <input
                    value={name()}
                    onInput={(e: Event & { currentTarget: HTMLInputElement }) => setName(e.currentTarget.value)}
                    placeholder='Front Gate'
                    type="text" id="camera-name" class="px-3 py-1.5 mt-1 block w-full rounded-lg bg-neu-850 border border-neu-750 text-white focus:outline-none placeholder:text-neu-500" />
            </div>
            <div>
                <label for="camera-url" class="text-sm font-medium text-neu-300">Camera URL</label>
                <input
                    value={uri()}
                    onInput={(e: Event & { currentTarget: HTMLInputElement }) => setUri(e.currentTarget.value)}
                    placeholder='rtsp://localhost:8554/cam'
                    type="text" id="camera-url" class="px-3 py-1.5 mt-1 block w-full rounded-lg bg-neu-850 border border-neu-750 text-white focus:outline-none placeholder:text-neu-500" />
            </div>
            <div>
                <label for="camera-labels" class="text-sm font-medium text-neu-300">Labels (comma-separated)</label>
                <input
                    value={labels()}
                    onInput={(e: Event & { currentTarget: HTMLInputElement }) => setLabels(e.currentTarget.value)}
                    placeholder='Outside, Security, Front Door'
                    type="text" id="camera-labels" class="px-3 py-1.5 mt-1 block w-full rounded-lg bg-neu-850 border border-neu-750 text-white focus:outline-none placeholder:text-neu-500" />
            </div>
            <div class="flex items-center justify-between">
                <Switch.Root
                    checked={saveToDisk()}
                    onCheckedChange={(details) => setSaveToDisk(details.checked)}
                    class="flex items-center"
                >
                    <Switch.Control class="relative inline-flex h-6 w-11 items-center rounded-full border-2 border-transparent transition-colors focus:outline-none data-[state=checked]:bg-violet-500 data-[state=unchecked]:bg-neu-700">
                        <Switch.Thumb class="inline-block h-5 w-5 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out data-[state=checked]:translate-x-5 data-[state=unchecked]:translate-x-0" />
                    </Switch.Control>
                    <Switch.Label class="ml-3 text-sm font-medium text-neu-300">Save to Disk</Switch.Label>
                    <Switch.HiddenInput />
                </Switch.Root>
            </div>
            <Show when={saveToDisk()}>
                <div>
                    <label for="save-dir" class="text-sm font-medium text-neu-300">Save Directory (optional)</label>
                    <input
                        value={saveDir()}
                        onInput={(e: Event & { currentTarget: HTMLInputElement }) => setsaveDir(e.currentTarget.value)}
                        placeholder='/path/to/recordings'
                        type="text" id="save-dir" class="px-3 py-1.5 mt-1 block w-full rounded-lg bg-neu-850 border border-neu-750 text-white focus:outline-none placeholder:text-neu-500" />
                </div>
            </Show>
            <div class="flex justify-end pt-4">
                {/* There should be no asChild here */}
                <Dialog.CloseTrigger>
                    <button
                        onClick={handleSave}
                        class="px-4 py-2 text-sm font-medium text-white bg-neu-800 rounded-lg hover:bg-neu-850 border border-neu-750 focus:outline-none">
                        Save Camera
                    </button>
                </Dialog.CloseTrigger>
            </div>
        </div>
    </ArkDialog>
}