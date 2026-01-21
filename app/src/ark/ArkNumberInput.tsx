import { NumberInput } from '@ark-ui/solid'
import { FiChevronUp, FiChevronDown } from 'solid-icons/fi'

export const ArkNumberInput = (props: {
    value: () => string
    onValueChange: (details: { value: string }) => void
    min?: number
    max?: number
    placeholder?: string
    disabled?: boolean
}) => {
    return (
        <NumberInput.Root
            value={props.value()}
            onValueChange={props.onValueChange}
            min={props.min}
            max={props.max}
            disabled={props.disabled}
        >
            <NumberInput.Control class="flex items-center h-10">
                <NumberInput.Input
                    placeholder={props.placeholder}
                    class="flex-1 px-4 py-2 h-full bg-neu-850 border border-neu-750 text-white text-center focus:outline-none min-w-16 rounded-l-lg"
                />
                <div class="flex flex-col border border-neu-750 border-l-0 rounded-r-lg overflow-hidden h-full">
                    <NumberInput.IncrementTrigger class="p-1 bg-neu-850 text-neu-400 hover:text-white hover:bg-neu-800 transition-colors flex-1 flex items-center justify-center">
                        <FiChevronUp class="w-3 h-3" />
                    </NumberInput.IncrementTrigger>
                    <NumberInput.DecrementTrigger class="p-1 bg-neu-850 text-neu-400 hover:text-white hover:bg-neu-800 transition-colors flex-1 flex items-center justify-center border-t border-neu-750">
                        <FiChevronDown class="w-3 h-3" />
                    </NumberInput.DecrementTrigger>
                </div>
            </NumberInput.Control>
        </NumberInput.Root>
    )
}
