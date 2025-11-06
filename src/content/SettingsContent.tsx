import { createSignal, onMount, createEffect } from "solid-js";
import LayoutContent from "./LayoutContent";
import ArkSwitch from "~/src/ark/ArkSwitch";
import { saveSettings, settings } from "~/src/shared";

export default function SettingsContent() {
    const [objectDetection, setObjectDetection] = createSignal(false);

    createEffect(() => {
        const objDetSetting = settings()['object_detection_enabled'];
        if (objDetSetting !== undefined) {
            setObjectDetection(objDetSetting === 'true');
        }
    });

    const handleObjectDetectionChange = (details: { checked: boolean }) => {
        setObjectDetection(details.checked);
    };

    const handleSaveSettings = async () => {
        await saveSettings({ 'object_detection_enabled': objectDetection().toString() });
    };

    return <LayoutContent title="Settings">
        <div class="p-4">
            <div class="bg-neu-850 border border-neu-800 rounded-lg p-6">
                <div class="flex items-center justify-between">
                    <ArkSwitch
                        checked={objectDetection}
                        onCheckedChange={handleObjectDetectionChange}
                        label="Enable Object Detection"
                    />
                </div>
            </div>
            <div class="flex justify-end mt-4">
                <button
                    onClick={handleSaveSettings}
                    class="btn-primary">
                    Save Settings
                </button>
            </div>
        </div>
    </LayoutContent>
}