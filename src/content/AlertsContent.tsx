import { FaSolidChevronRight } from "solid-icons/fa";
import { ArkCollapsible } from "../ark/ArkCollapsible";

export default function AlertsContent() {
    return <div class="h-screen py-2 pr-2">
        <div class="flex items-center flex-col space-y-16 relative isolate overflow-auto p-4  bg-neu-900 h-full rounded-2xl border border-neu-800 ">
            <div class="border bg-neu-850 border-neu-800 p-4 rounded-2xl w-full max-w-2xl">
                <ArkCollapsible
                    toggle={<div class="p-2 flex-1">
                        <div class="float-left">
                            API Hooks
                        </div>
                    </div>}
                >
                    <div class="p-2">
                        Add
                    </div>
                </ArkCollapsible>
            </div>
        </div>
    </div>
}