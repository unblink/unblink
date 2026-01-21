import { createEffect, createSignal, onCleanup, onMount, Show, untrack, type Component } from 'solid-js';
import { FaSolidSpinner } from 'solid-icons/fa';
import { webrtcClient } from '../lib/rpc';

interface Props {
  nodeId: string;
  serviceId?: string;
  name?: string;
  autoStart?: boolean;
}

const VideoTile: Component<Props> = (props) => {
  const [videoRef, setVideoRef] = createSignal<HTMLVideoElement>();
  const [pc, setPc] = createSignal<RTCPeerConnection>();
  const [loading, setLoading] = createSignal(true);
  const [error, setError] = createSignal<string | null>(null);

  const startStream = async () => {
    const videoEl = untrack(videoRef);
    if (!videoEl) {
      setError('Video element not found');
      setLoading(false);
      return;
    }

    try {
      const newPc = new RTCPeerConnection({
        iceServers: [{ urls: 'stun:stun.l.google.com:19302' }],
      });

      newPc.ontrack = (event) => {
        console.log(`Got ${event.track.kind} track:`, event.track.label || event.track.id);
        videoEl.srcObject = event.streams[0];
        setLoading(false);
      };

      newPc.oniceconnectionstatechange = () => {
        console.log('ICE state:', newPc.iceConnectionState);
        if (newPc.iceConnectionState === 'failed' || newPc.iceConnectionState === 'disconnected') {
          setError('Connection failed');
          setLoading(false);
        }
      };

      // Add transceivers for video and audio
      newPc.addTransceiver('video', { direction: 'recvonly' });
      newPc.addTransceiver('audio', { direction: 'recvonly' });

      const offer = await newPc.createOffer();
      await newPc.setLocalDescription(offer);

      const response = await webrtcClient.createWebRTCSession({
        nodeId: props.nodeId,
        serviceId: props.serviceId || '',
        sdp: newPc.localDescription?.sdp || '',
      });

      await newPc.setRemoteDescription(
        new RTCSessionDescription({
          type: 'answer',
          sdp: response.sdp,
        })
      );
      setPc(newPc);
    } catch (e: any) {
      console.error(e);
      setError(e.message || 'Stream error');
      setLoading(false);
    }
  };

  onMount(() => {
    if (props.autoStart !== false) {
      startStream();
    }
  });

  // Restart stream when nodeId or serviceId changes
  createEffect(() => {
    const nodeId = props.nodeId;
    const serviceId = props.serviceId;

    // Clean up existing connection
    const peerConnection = untrack(pc);
    if (peerConnection) {
      peerConnection.close();
      setPc(undefined);
    }

    // Reset states
    setLoading(true);
    setError(null);

    // Start new stream if autoStart is not disabled
    if (props.autoStart !== false) {
      startStream();
    }
  });

  onCleanup(() => {
    const peerConnection = untrack(pc);
    if (peerConnection) {
      peerConnection.close();
    }
  });

  return (
    <div class="relative w-full h-full bg-black flex items-center justify-center rounded-2xl overflow-hidden">
      <Show
        when={error()}
        fallback={
          <>
            <Show when={loading()}>
              <div class="absolute inset-0 flex items-center justify-center">
                <FaSolidSpinner class="w-8 h-8 text-neu-400 animate-spin" />
              </div>
            </Show>
            <video
              ref={setVideoRef}
              autoplay
              playsinline
              muted
              class="w-full h-full object-contain"
            />
            <Show when={props.name}>
              <div class="absolute bottom-2 left-2 px-2 py-1 bg-black/70 text-white text-sm rounded">
                {props.name}
              </div>
            </Show>
          </>
        }
      >
        <div class="text-red-500 text-sm p-2">{error()}</div>
      </Show>
    </div>
  );
};

export default VideoTile;
