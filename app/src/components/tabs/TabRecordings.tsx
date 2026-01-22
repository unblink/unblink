import { For, Show, createSignal, onMount } from "solid-js";
import { FiFilm, FiClock, FiHardDrive } from "solid-icons/fi";
import { storageClient } from "../../lib/rpc";
import type { Storage } from "@/gen/unblink/storage/v1/storage_pb";

export default function TabRecordings() {
  const [recordings, setRecordings] = createSignal<Storage[]>([]);
  const [loading, setLoading] = createSignal(true);
  const [error, setError] = createSignal<string | null>(null);

  // Fetch recordings on mount
  onMount(async () => {
    try {
      const response = await storageClient.listStorage({
        type: "video",
        limit: 100,
      });
      setRecordings(response.items || []);
    } catch (e) {
      console.error("Failed to fetch recordings:", e);
      setError("Failed to load recordings");
    } finally {
      setLoading(false);
    }
  });

  const formatFileSize = (bytes: bigint): string => {
    if (bytes === 0n) return "0 B";
    const sizes = ["B", "KB", "MB", "GB"];
    const numBytes = Number(bytes);
    const i = Math.floor(Math.log(numBytes) / Math.log(1024));
    return `${parseFloat((numBytes / Math.pow(1024, i)).toFixed(1))} ${sizes[i]}`;
  };

  const formatDuration = (seconds: string): string => {
    const secs = parseFloat(seconds);
    if (isNaN(secs)) return "-";
    const h = Math.floor(secs / 3600);
    const m = Math.floor((secs % 3600) / 60);
    const s = Math.floor(secs % 60);
    if (h > 0) return `${h}h ${m}m`;
    if (m > 0) return `${m}m ${s}s`;
    return `${s}s`;
  };

  const formatDate = (timestamp: bigint): string => {
    return new Date(Number(timestamp) * 1000).toLocaleString();
  };

  // Get video URL
  const apiUrl = import.meta.env.VITE_RELAY_API_URL || '';
  const getVideoUrl = (id: string) => {
    return `${apiUrl}/storage/${id}`;
  };

  return (
    <div class="w-full h-full overflow-y-auto p-4">
      <Show
        when={!loading()}
        fallback={
          <div class="flex items-center justify-center h-64">
            <div class="text-neu-400">Loading recordings...</div>
          </div>
        }
      >
        <Show
          when={!error() && recordings().length > 0}
          fallback={
            <div class="h-full flex items-center justify-center text-neu-500">
              <div class="text-center">
                <FiFilm class="mx-auto mb-4 w-12 h-12" />
                <p>No recordings found</p>
                <p class="text-sm mt-2">Recordings will appear here when video recording is enabled</p>
              </div>
            </div>
          }
        >
          <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
            <For each={recordings()}>
              {(recording) => (
                <div class="bg-neu-900 rounded-xl border border-neu-800 overflow-hidden hover:border-neu-700 transition-colors">
                  {/* Video Player/Thumbnail */}
                  <div class="aspect-video bg-black relative">
                    <video
                      src={getVideoUrl(recording.id)}
                      controls
                      class="w-full h-full object-contain"
                      preload="metadata"
                    >
                      Your browser does not support the video tag.
                    </video>
                  </div>

                  {/* Recording Info */}
                  <div class="p-4">
                    <div class="flex items-start justify-between mb-2">
                      <div class="flex-1">
                        <div class="text-sm font-medium text-white mb-1">
                          {recording.metadata.service_name || `Service ${recording.serviceId.slice(0, 8)}`}
                        </div>
                        <div class="flex items-center gap-3 text-xs text-neu-500">
                          <Show when={recording.metadata.duration_seconds}>
                            <span class="flex items-center gap-1">
                              <FiClock class="w-3 h-3" />
                              {formatDuration(recording.metadata.duration_seconds)}
                            </span>
                          </Show>
                          <span class="flex items-center gap-1">
                            <FiHardDrive class="w-3 h-3" />
                            {formatFileSize(recording.fileSize)}
                          </span>
                        </div>
                      </div>
                    </div>

                    <div class="text-xs text-neu-600">
                      {formatDate(recording.timestamp)}
                    </div>

                    <Show when={recording.metadata.status === "recording"}>
                      <div class="mt-2 flex items-center gap-1">
                        <span class="w-2 h-2 rounded-full bg-red-500 animate-pulse"></span>
                        <span class="text-xs text-red-400">Recording...</span>
                      </div>
                    </Show>
                  </div>
                </div>
              )}
            </For>
          </div>
        </Show>
      </Show>

      <Show when={error()}>
        <div class="flex items-center justify-center h-64">
          <div class="text-red-500">{error()}</div>
        </div>
      </Show>
    </div>
  );
}
