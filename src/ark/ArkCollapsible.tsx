import { Collapsible } from '@ark-ui/solid/collapsible'
import { FaSolidChevronRight } from 'solid-icons/fa'


export const ArkCollapsible = (props: {
    children: any
    toggle: any
}) => (
    <Collapsible.Root>
        <Collapsible.Trigger class='w-full flex items-center text-neu-400 hover:text-white transition-all focus:outline-none'>
            {props.toggle}
            <Collapsible.Indicator>
                <FaSolidChevronRight />
            </Collapsible.Indicator>
        </Collapsible.Trigger>
        <Collapsible.Content>{props.children}</Collapsible.Content>
    </Collapsible.Root>
)
