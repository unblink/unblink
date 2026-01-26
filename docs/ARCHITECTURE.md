# Unblink Relay Architecture

## Video Processing Pipeline

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                               CAMERA SOURCES                                │
└──────────────────────┬──────────────────────────────┬───────────────────────┘
                       │                              │
            ┌──────────▼──────────┐        ┌──────────▼──────────┐
            │     RTSP Camera     │        │    MJPEG Camera     │
            │     (H.264/RTP)     │        │    (MJPEG/HTTP)     │
            └──────────┬──────────┘        └──────────┬──────────┘
                       │                              │
┌──────────────────────▼──────────────────────────────▼───────────────────────┐
│                                   go2rtc                                    │
│  ┌───────────────────────────────────────────────────────────────────────┐  │
│  │ RTSP: rtsp.Conn extracts raw H.264 from RTP packets                   │  │
│  │ MJPEG: HTTP MJPEG stream handling                                     │  │
│  └───────────────────────────────────────────────────────────────────────┘  │
└──────────────┬──────────────────────────────────────────────┬───────────────┘
               │                                              │
               │ Raw H.264                                    │ MJPEG Stream
               │                                              │
┌──────────────▼─────────────┐                ┌──────────────▼──────────────┐
│     MediaSource (RTSP)     │                │         MJPEGSource         │
│  GetProducer() → rtsp.Conn │                │ ┌─────────────────────────┐ │
│                            │                │ │  FFmpeg #1: TRANSCODE   │ │
│                            │                │ │  MJPEG -> H.264         │ │
│                            │                │ │  Output: MPEG-TS        │ │
└──────────────┬─────────────┘                └──────────────┬──────────────┘
               │                                              │
               └──────────────────────┬───────────────────────┘
                                      │
┌─────────────────────────────────────▼───────────────────────────────────────┐
│                       MPEGTSConsumer (shared helper)                        │
│          Encapsulates mpegts.NewConsumer() + track/codec discovery          │
└─────────────────────────────────────┬───────────────────────────────────────┘
                                      │
                 ┌────────────────────┼────────────────────┐
                 │                    │                    │
      ┌──────────▼─────────┐ ┌────────▼──────────┐ ┌───────▼──────────┐
      │   FrameExtractor   │ │   VideoRecorder   │ │      WebRTC      │
      │ ┌────────────────┐ │ │ ┌───────────────┐ │ │ ┌──────────────┐ │
      │ │ FFmpeg #2:     │ │ │ │ FFmpeg #3:    │ │ │ │ FFmpeg #4:   │ │
      │ │ H.264 -> JPEG  │ │ │ │ MPEG-TS -> HLS│ │ │ │ H.264 -> VP9 │ │
      │ └───────┬────────┘ │ │ └───────┬───────┘ │ │ └──────┬───────┘ │
      └─────────│──────────┘ └─────────│─────────┘ └────────│─────────┘
                ▼                      ▼                    ▼
           JPEG for AI            HLS Stream          WebRTC Stream
```

## FFmpeg Stages Summary

### RTSP Camera Pipeline

1. **FrameExtractor**: H.264 → JPEG (transcode)
2. **VideoRecorder**: MPEG-TS → HLS (remux, auto-rotation)
3. **WebRTC**: H.264 → VP8/VP9 (transcode, if browser requires)

### MJPEG Camera Pipeline

1. **MJPEGSource**: MJPEG → H.264 (transcode, libx264)
2. **FrameExtractor**: H.264 → JPEG (transcode)
3. **VideoRecorder**: MPEG-TS → HLS (remux, auto-rotation)
4. **WebRTC**: H.264 → VP8/VP9 (transcode, if browser requires)

## Transcoding vs Remuxing

| Operation     | Description                      | CPU Usage |
| ------------- | -------------------------------- | --------- |
| **Transcode** | Decode → Encode (changes codec)  | HIGH      |
| **Remux**     | Copy codec data to new container | ~1%       |

## Key Components

| Component            | Input    | Output                  | FFmpeg?     | Purpose                                          |
| -------------------- | -------- | ----------------------- | ----------- | ------------------------------------------------ |
| **go2rtc rtsp.Conn** | RTSP/RTP | Raw H.264 + Audio       | No          | RTSP protocol handling                           |
| **MPEGTSConsumer**   | Producer | MPEG-TS (video + audio) | No (helper) | H.264 + audio track discovery + MPEG-TS wrapping |
| **MJPEGSource**      | MJPEG    | MPEG-TS                 | Yes #1      | MJPEG → H.264                                    |
| **FrameExtractor**   | MPEG-TS  | JPEG                    | Yes #2/#3   | AI frame extraction                              |
| **VideoRecorder**    | MPEG-TS  | HLS (.m3u8 + .ts)       | Yes #3      | Video + audio storage with auto-rotation         |
| **WebRTC**           | MPEG-TS  | WebRTC                  | Yes #4      | Browser streaming                                |
