import { For, Show, onMount, createSignal } from 'solid-js';
import { FiVideo, FiServer, FiEdit2, FiTrash2 } from 'solid-icons/fi';
import { nodes, nodesLoading, services, fetchNodes, fetchNodesAndServices, fetchServices, setTab } from '../../shared';
import { ArkDialog } from '../../ark/ArkDialog';
import { Dialog } from '@ark-ui/solid/dialog';
import { toaster } from '../../ark/ArkToast';
import { ServiceForm } from '../ServiceForm';
import { serviceClient, nodeClient } from '../../lib/rpc';
import { Service } from '@/gen/unblink/service/v1/service_pb';
import { Node } from '@/gen/unblink/node/v1/node_pb';

function ServiceEditDialog(props: { service: Service }) {
  const [selectedNodeId, setSelectedNodeId] = createSignal(props.service.nodeId);
  const [name, setName] = createSignal(props.service.name);
  const [description, setDescription] = createSignal(props.service.description || "");
  const [serviceUrl, setServiceUrl] = createSignal(props.service.serviceUrl);

  const handleSave = async () => {
    const newName = name().trim();
    const newDescription = description().trim();
    const newServiceUrl = serviceUrl().trim();

    if (!newName || !newServiceUrl) {
      return;
    }

    toaster.promise(async () => {
      await serviceClient.updateService({
        serviceId: props.service.id,
        name: newName,
        description: newDescription,
        serviceUrl: newServiceUrl,
        status: '',
      });

      await fetchServices();
    }, {
      loading: {
        title: 'Saving...',
        description: 'Your service is being updated.',
      },
      success: {
        title: 'Success!',
        description: 'Service has been updated successfully.',
      },
      error: {
        title: 'Failed',
        description: 'There was an error updating your service. Please try again.',
      },
    });
  };

  return (
    <ArkDialog
      trigger={(_, setOpen) => (
        <button
          onClick={(e) => {
            e.stopPropagation();
            e.preventDefault();
            setOpen(true);
          }}
          class="px-3 py-1.5 rounded-lg border border-neu-750 bg-neu-800 hover:bg-neu-850 text-neu-300 text-xs font-medium transition-colors flex items-center gap-1.5"
        >
          <FiEdit2 class="w-3 h-3" />
          Edit
        </button>
      )}
      title="Edit Service"
      description="Modify your service configuration"
    >
      {() => (
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
            <Dialog.CloseTrigger>
              <button
                onClick={handleSave}
                class="btn-primary"
              >
                Save Service
              </button>
            </Dialog.CloseTrigger>
          </div>
        </div>
      )}
    </ArkDialog>
  );
}

function ServiceDeleteDialog(props: { service: Service }) {
  const handleDelete = async () => {
    toaster.promise(async () => {
      await serviceClient.deleteService({
        serviceId: props.service.id,
      });
      await fetchServices();
    }, {
      loading: {
        title: 'Deleting...',
        description: 'Your service is being deleted.',
      },
      success: {
        title: 'Success!',
        description: 'Service has been deleted successfully.',
      },
      error: {
        title: 'Failed',
        description: 'There was an error deleting your service. Please try again.',
      },
    });
  };

  return (
    <ArkDialog
      trigger={(_, setOpen) => (
        <button
          onClick={(e) => {
            e.stopPropagation();
            e.preventDefault();
            setOpen(true);
          }}
          class="px-3 py-1.5 rounded-lg border border-neu-750 bg-neu-800 hover:bg-neu-850 text-neu-300 text-xs font-medium transition-colors flex items-center gap-1.5"
        >
          <FiTrash2 class="w-3 h-3" />
          Delete
        </button>
      )}
      title="Delete Service"
      description={`Are you sure you want to delete "${props.service.name || 'Unnamed Service'}"? This action cannot be undone.`}
    >
      <div class="flex justify-end pt-4">
        <Dialog.CloseTrigger>
          <button
            onClick={handleDelete}
            class="btn-danger"
          >
            Delete Service
          </button>
        </Dialog.CloseTrigger>
      </div>
    </ArkDialog>
  );
}

function ServiceInfoDialog(props: { service: Service; nodeId: string }) {
  return (
    <ArkDialog
      trigger={(_, setOpen) => (
        <button
          onClick={(e) => {
            e.stopPropagation();
            e.preventDefault();
            setOpen(true);
          }}
          class="px-3 py-1.5 rounded-lg border border-neu-750 bg-neu-800 hover:bg-neu-850 text-neu-300 text-xs font-medium transition-colors"
        >
          Detail
        </button>
      )}
      title="Service Details"
      description="View all information about this service"
    >
      <div class="mt-4 space-y-4">
        <div>
          <label class="text-xs font-medium text-neu-500 uppercase tracking-wide">Service ID</label>
          <p class="mt-1 text-sm text-white line-clamp-1 font-mono">{props.service.id}</p>
        </div>
        <div>
          <label class="text-xs font-medium text-neu-500 uppercase tracking-wide">Service Name</label>
          <p class="mt-1 text-sm text-white line-clamp-1 whitespace-nowrap">{props.service.name || 'Unnamed Service'}</p>
        </div>
        <div>
          <label class="text-xs font-medium text-neu-500 uppercase tracking-wide">Service URL</label>
          <p class="mt-1 text-sm text-white font-mono line-clamp-1 break-all">{props.service.serviceUrl}</p>
        </div>
        <div>
          <label class="text-xs font-medium text-neu-500 uppercase tracking-wide">Node ID</label>
          <p class="mt-1 text-sm text-white line-clamp-1 font-mono">{props.nodeId}</p>
        </div>
      </div>
    </ArkDialog>
  );
}


function NodeDeleteDialog(props: { node: Node }) {
  const handleDelete = async () => {
    toaster.promise(async () => {
      await nodeClient.deleteNode({
        nodeId: props.node.id,
      });
      await fetchNodes();
    }, {
      loading: {
        title: 'Deleting...',
        description: 'Your node is being deleted.',
      },
      success: {
        title: 'Success!',
        description: 'Node has been deleted successfully.',
      },
      error: {
        title: 'Failed',
        description: 'There was an error deleting your node. Please try again.',
      },
    });
  };

  return (
    <ArkDialog
      trigger={(_, setOpen) => (
        <button
          onClick={(e) => {
            e.stopPropagation();
            e.preventDefault();
            setOpen(true);
          }}
          class="px-3 py-1.5 rounded-lg border border-neu-750 bg-neu-800 hover:bg-neu-850 text-neu-300 text-xs font-medium transition-colors flex items-center gap-1.5"
        >
          <FiTrash2 class="w-3 h-3" />
          Delete
        </button>
      )}
      title="Delete Node"
      description={`Are you sure you want to delete "${props.node.hostname || 'Unnamed Node'}"? This action cannot be undone.`}
    >
      <div class="flex justify-end pt-4">
        <Dialog.CloseTrigger>
          <button
            onClick={handleDelete}
            class="btn-danger"
          >
            Delete Node
          </button>
        </Dialog.CloseTrigger>
      </div>
    </ArkDialog>
  );
}

function NodeInfoDialog(props: { node: Node }) {
  return (
    <ArkDialog
      trigger={(_, setOpen) => (
        <button
          onClick={(e) => {
            e.stopPropagation();
            e.preventDefault();
            setOpen(true);
          }}
          class="px-3 py-1.5 rounded-lg border border-neu-750 bg-neu-800 hover:bg-neu-850 text-neu-300 text-xs font-medium transition-colors"
        >
          Detail
        </button>
      )}
      title="Node Details"
      description="View all information about this node"
    >
      <div class="mt-4 space-y-4">
        <Show when={props.node.hostname}>
          <div>
            <label class="text-xs font-medium text-neu-500 uppercase tracking-wide">Hostname</label>
            <p class="mt-1 text-sm text-white line-clamp-1 whitespace-nowrap">{props.node.hostname}</p>
          </div>
        </Show>
        <Show when={props.node.macAddresses && props.node.macAddresses.length > 0}>
          <div>
            <label class="text-xs font-medium text-neu-500 uppercase tracking-wide">MAC Addresses</label>
            <div class="mt-1 text-sm text-neu-300 space-y-1">
              <For each={props.node.macAddresses!}>
                {(mac) => <p class="line-clamp-1 font-mono">{mac}</p>}
              </For>
            </div>
          </div>
        </Show>
        <div>
          <label class="text-xs font-medium text-neu-500 uppercase tracking-wide">Node ID</label>
          <p class="mt-1 text-sm text-white line-clamp-1 font-mono">{props.node.id}</p>
        </div>
        <div>
          <label class="text-xs font-medium text-neu-500 uppercase tracking-wide">Status</label>
          <p class="mt-1">
            <span class={`inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded text-xs font-medium whitespace-nowrap ${props.node.status === 'online'
              ? 'bg-green-900/30 text-green-400'
              : 'bg-neu-800 text-neu-400'
              }`}>
              <span class={`w-1.5 h-1.5 rounded-full ${props.node.status === 'online' ? 'bg-green-500' : 'bg-neu-500'
                }`}></span>
              <span class="capitalize">{props.node.status}</span>
            </span>
          </p>
        </div>
        <div>
          <label class="text-xs font-medium text-neu-500 uppercase tracking-wide">Services</label>
          <p class="mt-1 text-sm text-white line-clamp-1">{services().filter(s => s.nodeId === props.node.id).length} service(s)</p>
        </div>
        <Show when={props.node.lastConnectedAt}>
          <div>
            <label class="text-xs font-medium text-neu-500 uppercase tracking-wide">Last Connected</label>
            <p class="mt-1 text-sm text-neu-300 line-clamp-1">
              {new Date(Number(props.node.lastConnectedAt)).toLocaleString()}
            </p>
          </div>
        </Show>
      </div>
    </ArkDialog>
  );
}

export default function HomeContent() {
  const sortedNodes = () => {
    return [...nodes()].sort((a, b) => {
      // Online nodes come first
      if (a.status === 'online' && b.status !== 'online') return -1;
      if (a.status !== 'online' && b.status === 'online') return 1;

      // Within same status, sort by last connected time (most recent first)
      const aTime = a.lastConnectedAt ? new Date(Number(a.lastConnectedAt)).getTime() : 0;
      const bTime = b.lastConnectedAt ? new Date(Number(b.lastConnectedAt)).getTime() : 0;
      return bTime - aTime;
    });
  };

  const handleServiceClick = (nodeId: string, serviceId: string, name: string) => {
    setTab({ type: 'view', nodeId, serviceId, name });
  };

  onMount(fetchNodesAndServices);

  return (
    <Show
      when={!nodesLoading()}
      fallback={
        <div class="h-full flex items-center justify-center">
          <div class="text-neu-500">Loading nodes...</div>
        </div>
      }
    >
      <Show
        when={nodes().length > 0}
        fallback={
          <div class="h-full flex items-center justify-center text-neu-500">
            <div class="text-center">
              <FiServer class="mx-auto mb-4 w-12 h-12" />
              <p>No nodes found</p>
              <p>Connect a node to get started</p>
            </div>
          </div>
        }
      >
        <div class="px-4 py-4 space-y-6">
          <For each={sortedNodes()}>
            {(node) => (
              <div class="bg-neu-850 rounded-xl p-4 hover:border-neu-700 transition-all">
                <div class="flex items-center justify-between mb-0.5">
                  <h3 class="text-lg font-medium text-white line-clamp-1 whitespace-nowrap">{node.hostname || 'Unnamed Node'}</h3>
                  <div class="flex items-center gap-4">
                    <span class="inline-flex items-center gap-1.5">
                      <span class={`w-2 h-2 rounded-full ${node.status === 'online' ? 'bg-green-500' : 'bg-neu-500'}`}></span>
                      <span class="text-xs text-neu-300 capitalize">{node.status}</span>
                    </span>
                    <div class="flex items-center gap-2">
                      <NodeDeleteDialog node={node} />
                      <NodeInfoDialog node={node} />
                    </div>
                  </div>
                </div>
                <p class="text-sm text-neu-400 line-clamp-1 break-all mb-4">{node.id}</p>
                {(() => {
                  const nodeServices = () => services().filter(s => s.nodeId === node.id);
                  return (
                    <div>
                      <Show
                        when={nodeServices().length > 0}
                        fallback={<p class="text-sm text-neu-500">No services</p>}
                      >
                        <div class="space-y-2">
                          <For each={nodeServices()}>
                            {(service) => (
                              <div class="bg-neu-850 border border-neu-750 rounded-xl p-4 cursor-pointer hover:bg-neu-800 hover:border-neu-700 transition-all" onClick={() => handleServiceClick(node.id, service.id, service.name)}>
                                <div class="flex items-center justify-between">
                                  <div class="flex items-center gap-3 min-w-0 flex-1">
                                    <div class="w-10 h-10 rounded-lg bg-neu-800 border border-neu-750 flex items-center justify-center flex-shrink-0">
                                      <FiVideo class="w-5 h-5 text-neu-300" />
                                    </div>
                                    <div class="min-w-0 flex-1">
                                      <h4 class="text-white font-medium line-clamp-1 whitespace-nowrap">{service.name || 'Unnamed Service'}</h4>
                                      <p class="text-xs text-neu-500 line-clamp-1 break-all">{service.serviceUrl}</p>
                                    </div>
                                  </div>
                                  <div class="flex items-center gap-3 ml-4">
                                    <ServiceEditDialog service={service} />
                                    <ServiceInfoDialog service={service} nodeId={node.id} />
                                    <ServiceDeleteDialog service={service} />
                                  </div>
                                </div>
                              </div>
                            )}
                          </For>
                        </div>
                      </Show>
                    </div>
                  );
                })()}
              </div>
            )}
          </For>
        </div>
      </Show>
    </Show>
  );
}
