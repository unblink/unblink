import { For, Show, onMount, createSignal } from 'solid-js';
import { FiTrash2, FiCpu, FiMoreVertical, FiEye } from 'solid-icons/fi';
import { agents, agentsLoading, fetchAgents, relayFetch, type AgentInfo } from '../../shared';
import { Dialog } from '@ark-ui/solid/dialog';
import { Menu } from '@ark-ui/solid/menu';
import { Portal } from 'solid-js/web';
import { toaster } from '../../ark/ArkToast';
import { AgentForm } from '../AgentForm';

function AgentEditDialog(props: { agent: AgentInfo; open: boolean; onOpenChange: (details: { open: boolean }) => void }) {
  const [name, setName] = createSignal(props.agent.name);
  const [instruction, setInstruction] = createSignal(props.agent.instruction);
  const [selectedServiceIds, setSelectedServiceIds] = createSignal<string[]>(props.agent.service_ids || []);

  onMount(() => {
    setName(props.agent.name);
    setInstruction(props.agent.instruction);
    setSelectedServiceIds(props.agent.service_ids || []);
  });

  const handleSave = async () => {
    const newName = name().trim();
    const newInstruction = instruction().trim();

    if (!newName || !newInstruction) {
      return;
    }

    if (newName.length > 255) {
      toaster.error({
        title: 'Error',
        description: 'Name too long (max 255 characters)',
      });
      return;
    }

    if (newInstruction.length > 5000) {
      toaster.error({
        title: 'Error',
        description: 'Instruction too long (max 5000 characters)',
      });
      return;
    }

    toaster.promise(async () => {
      const response = await relayFetch(`/agents/${props.agent.id}`, {
        method: 'PATCH',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          name: newName,
          instruction: newInstruction,
          service_ids: selectedServiceIds(),
        }),
      });

      if (response.ok) {
        await fetchAgents();
      } else {
        throw new Error('Failed to update agent');
      }
    }, {
      loading: {
        title: 'Saving...',
        description: 'Your agent is being updated.',
      },
      success: {
        title: 'Success!',
        description: 'Agent has been updated successfully.',
      },
      error: {
        title: 'Failed',
        description: 'There was an error updating your agent. Please try again.',
      },
    });
  };

  return (
    <Dialog.Root open={props.open} onOpenChange={props.onOpenChange}>
      <Dialog.Backdrop class="fixed inset-0 bg-black/50 z-40" />
      <Dialog.Positioner class="fixed inset-0 z-50 flex items-center justify-center p-4">
        <Dialog.Content class="bg-neu-900 border border-neu-750 rounded-lg p-6 w-full max-w-lg shadow-xl">
          <Dialog.Title class="text-lg font-semibold text-white">Edit Agent</Dialog.Title>
          <Dialog.Description class="text-sm text-neu-400 mt-1">Modify your agent's configuration</Dialog.Description>
          <div class="mt-4 space-y-4">
            <AgentForm
              name={name}
              setName={setName}
              instruction={instruction}
              setInstruction={setInstruction}
              selectedServiceIds={selectedServiceIds}
              setSelectedServiceIds={setSelectedServiceIds}
              showTemplates={false}
            />
            <div class="flex justify-end pt-4">
              <Dialog.CloseTrigger>
                <button
                  onClick={handleSave}
                  class="btn-primary"
                >
                  Save Agent
                </button>
              </Dialog.CloseTrigger>
            </div>
          </div>
        </Dialog.Content>
      </Dialog.Positioner>
    </Dialog.Root>
  );
}

function AgentDeleteDialog(props: { agent: AgentInfo; open: boolean; onOpenChange: (details: { open: boolean }) => void }) {
  const handleDelete = async () => {
    toaster.promise(async () => {
      const response = await relayFetch(`/agents/${props.agent.id}`, {
        method: 'DELETE',
      });

      if (response.ok) {
        await fetchAgents();
      } else {
        throw new Error('Failed to delete agent');
      }
    }, {
      loading: {
        title: 'Deleting...',
        description: 'Your agent is being deleted.',
      },
      success: {
        title: 'Success!',
        description: 'Agent has been deleted successfully.',
      },
      error: {
        title: 'Failed',
        description: 'There was an error deleting your agent. Please try again.',
      },
    });
  };

  return (
    <Dialog.Root open={props.open} onOpenChange={props.onOpenChange}>
      <Dialog.Backdrop class="fixed inset-0 bg-black/50 z-40" />
      <Dialog.Positioner class="fixed inset-0 z-50 flex items-center justify-center p-4">
        <Dialog.Content class="bg-neu-900 border border-neu-750 rounded-lg p-6 w-full max-w-md shadow-xl">
          <Dialog.Title class="text-lg font-semibold text-white">Delete Agent</Dialog.Title>
          <Dialog.Description class="text-sm text-neu-400 mt-1">
            Are you sure you want to delete "{props.agent.name}"? This action cannot be undone.
          </Dialog.Description>
          <div class="mt-4 flex justify-end gap-2">
            <Dialog.CloseTrigger>
              <button class="px-4 py-2 rounded-lg border border-neu-750 bg-neu-850 hover:bg-neu-800 text-neu-300 transition-colors">
                Cancel
              </button>
            </Dialog.CloseTrigger>
            <Dialog.CloseTrigger>
              <button
                onClick={handleDelete}
                class="px-4 py-2 rounded-lg bg-red-600 hover:bg-red-700 text-white transition-colors"
              >
                Delete Agent
              </button>
            </Dialog.CloseTrigger>
          </div>
        </Dialog.Content>
      </Dialog.Positioner>
    </Dialog.Root>
  );
}

function AgentCard(props: { agent: AgentInfo }) {
  const [editOpen, setEditOpen] = createSignal(false);
  const [deleteOpen, setDeleteOpen] = createSignal(false);

  return (
    <>
      <div onClick={() => setEditOpen(true)} class="bg-neu-850 border border-neu-750 rounded-lg p-4 hover:border-neu-700 transition-all cursor-pointer">
        <div class="flex items-start justify-between mb-3">
          <div class="flex items-center gap-3">
            <div class="w-10 h-10 rounded-lg bg-neu-800 border border-neu-750 flex items-center justify-center">
              <FiEye class="w-5 h-5 text-neu-300" />
            </div>
            <div class="min-w-0 flex-1">
              <h3 class="text-white font-medium line-clamp-1">{props.agent.name}</h3>
              <p class="text-xs text-neu-500 line-clamp-1">
                Worker: <span class="text-neu-400">{props.agent.worker_id}</span>
              </p>
            </div>
          </div>
          <Menu.Root>
            <Menu.Trigger
              onClick={(e) => e.stopPropagation()}
              class="p-1.5 rounded-lg border border-neu-750 bg-neu-800 hover:bg-neu-850 text-neu-300 transition-colors"
            >
              <FiMoreVertical class="w-4 h-4" />
            </Menu.Trigger>
            <Portal>
              <Menu.Positioner>
                <Menu.Content class="bg-neu-850 border border-neu-800 rounded-lg shadow-lg py-1 min-w-[140px] z-50 focus:outline-none">
                  <Menu.Item
                    value="delete"
                    onSelect={() => setDeleteOpen(true)}
                    onClick={(e) => e.stopPropagation()}
                    class="flex items-center gap-2 px-3 py-2 text-sm text-red-400 hover:bg-neu-800 hover:text-red-300 cursor-pointer transition-colors rounded-md mx-1"
                  >
                    <FiTrash2 class="w-4 h-4" />
                    <span>Delete</span>
                  </Menu.Item>
                </Menu.Content>
              </Menu.Positioner>
            </Portal>
          </Menu.Root>
        </div>

        <div class="space-y-2">
          <div>
            <label class="text-xs font-medium text-neu-500 uppercase tracking-wide">Instruction</label>
            <p class="mt-1 text-sm text-neu-300 line-clamp-3">{props.agent.instruction}</p>
          </div>
        </div>
      </div>

      <AgentEditDialog
        agent={props.agent}
        open={editOpen()}
        onOpenChange={(details) => setEditOpen(details.open)}
      />
      <AgentDeleteDialog
        agent={props.agent}
        open={deleteOpen()}
        onOpenChange={(details) => setDeleteOpen(details.open)}
      />
    </>
  );
}

export default function TabAgents() {
  onMount(fetchAgents);

  return (
    <div class="w-full h-full overflow-y-auto p-4">


      <Show
        when={!agentsLoading()}
        fallback={
          <div class="flex items-center justify-center h-64">
            <div class="text-neu-400">Loading agents...</div>
          </div>
        }
      >
        <Show
          when={agents().length > 0}
          fallback={
            <div class="h-full flex items-center justify-center text-neu-500">
              <div class="text-center">
                <FiEye class="mx-auto mb-4 w-12 h-12" />
                <p>No agents yet</p>
                <p>Click the "Create Agent" button in the sidebar to get started</p>
              </div>
            </div>
          }
        >
          <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 xl:grid-cols-5 gap-4">
            <For each={agents()}>
              {(agent) => <AgentCard agent={agent} />}
            </For>
          </div>
        </Show>
      </Show>
    </div>
  );
}
