
import { Dialog } from '@ark-ui/solid/dialog'
import { BsX } from 'solid-icons/bs'
import { createSignal, type JSX } from 'solid-js'
import { Portal } from 'solid-js/web'

export const ArkDialog = (props: {
  trigger: (open: boolean, setOpen: (open: boolean) => void) => JSX.Element
  title: string
  description?: string
  children: JSX.Element | ((setOpen: (open: boolean) => void) => JSX.Element)
}) => {
  const [open, setOpen] = createSignal(false)

  return (
    <>
      {props.trigger(open(), setOpen)}
      <Dialog.Root open={open()} onOpenChange={() => setOpen(false)}>
        <Portal>
          <Dialog.Backdrop
            class="fixed inset-0 bg-black/80 z-50"
            onClick={(e) => e.stopPropagation()}
          />
          <Dialog.Positioner
            class="fixed inset-0 flex items-center justify-center z-50"
            onClick={(e) => e.stopPropagation()}
          >
            <Dialog.Content
              class="relative w-full max-w-md max-h-[85vh] flex flex-col bg-neu-900 rounded-2xl border border-neu-800 shadow-lg"
              onClick={(e) => e.stopPropagation()}
            >
              {/* Fixed header - title, description */}
              <div class="flex-shrink-0 px-6 pt-6 pb-2">
                <Dialog.Title class="m-0 text-lg font-medium text-white">
                  {props.title}
                </Dialog.Title>
                {props.description && (
                  <Dialog.Description class="mt-2 text-sm leading-relaxed text-neu-400">
                    {props.description}
                  </Dialog.Description>
                )}
              </div>

              {/* Scrollable content area */}
              <div class="flex-1 overflow-y-auto px-6 pb-6 min-h-0">
                {typeof props.children === 'function' ? props.children(setOpen) : props.children}
              </div>

              {/* Close button - absolutely positioned */}
              <Dialog.CloseTrigger
                class="absolute top-2.5 right-2.5 text-neu-400 hover:text-white focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 focus-visible:ring-offset-2 focus-visible:ring-offset-neu-800"
                onClick={(e) => e.stopPropagation()}
              >
                <BsX class="w-6 h-6" />
              </Dialog.CloseTrigger>
            </Dialog.Content>
          </Dialog.Positioner>
        </Portal>
      </Dialog.Root>
    </>
  )
}
