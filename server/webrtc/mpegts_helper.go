package webrtc

import (
	"fmt"
	"io"
	"log"
	"time"

	"github.com/AlexxIT/go2rtc/pkg/core"
	"github.com/AlexxIT/go2rtc/pkg/mpegts"
)

// MPEGTSConsumer encapsulates a go2rtc MPEG-TS consumer with video and audio tracks
type MPEGTSConsumer struct {
	consumer  *mpegts.Consumer
	receivers []*core.Receiver
}

// NewMPEGTSConsumer creates a new MPEG-TS consumer from a producer
// It finds H.264 video track and optionally adds audio tracks (AAC, PCMU, PCMA, etc.)
// Returns an error if no H.264 track is found.
func NewMPEGTSConsumer(producer core.Producer, serviceName string) (*MPEGTSConsumer, error) {
	consumer := mpegts.NewConsumer()
	var receivers []*core.Receiver

	// Find and add H.264 video track
	var videoMedia *core.Media
	for _, media := range producer.GetMedias() {
		if media.Kind == core.KindVideo {
			videoMedia = media
			break
		}
	}

	if videoMedia == nil {
		return nil, fmt.Errorf("no video media found")
	}

	var videoCodec *core.Codec
	for _, codec := range videoMedia.Codecs {
		if codec.Name == core.CodecH264 {
			videoCodec = codec
			break
		}
	}

	if videoCodec == nil {
		return nil, fmt.Errorf("no H.264 codec found")
	}

	receiver, err := producer.GetTrack(videoMedia, videoCodec)
	if err != nil {
		return nil, fmt.Errorf("failed to get video track: %w", err)
	}

	if err := consumer.AddTrack(videoMedia, videoCodec, receiver); err != nil {
		return nil, fmt.Errorf("failed to add video track: %w", err)
	}
	receivers = append(receivers, receiver)

	// Find and add audio track (optional)
	// IMPORTANT: go2rtc's MPEG-TS consumer only supports H264, H265, and AAC codecs
	// Other audio codecs (PCMA, PCMU, Opus) will cause panic if added
	var audioMedia *core.Media
	for _, media := range producer.GetMedias() {
		if media.Kind == core.KindAudio {
			audioMedia = media
			break
		}
	}

	if audioMedia != nil {
		// Only AAC is supported by go2rtc's MPEG-TS consumer
		supportedCodecs := []string{
			core.CodecAAC,
		}

		for _, codecName := range supportedCodecs {
			for _, codec := range audioMedia.Codecs {
				if codec.Name == codecName {
					audioReceiver, err := producer.GetTrack(audioMedia, codec)
					if err != nil {
						log.Printf("[MPEGTSConsumer] Failed to get audio track %s: %v", codecName, err)
						continue
					}
					if err := consumer.AddTrack(audioMedia, codec, audioReceiver); err != nil {
						log.Printf("[MPEGTSConsumer] Failed to add audio track %s: %v", codecName, err)
						continue
					}
					receivers = append(receivers, audioReceiver)
					log.Printf("[MPEGTSConsumer] Added audio track: %s/%s", audioMedia.Kind, codecName)
					break
				}
			}
		}

		// Log if audio was found but not supported (e.g., PCMA, PCMU, Opus)
		found := false
		for _, codec := range audioMedia.Codecs {
			if codec.Name == core.CodecAAC {
				found = true
				break
			}
		}
		if !found && len(audioMedia.Codecs) > 0 {
			codecNames := make([]string, len(audioMedia.Codecs))
			for i, codec := range audioMedia.Codecs {
				codecNames[i] = codec.Name
			}
			log.Printf("[MPEGTSConsumer] Audio codec(s) not supported by MPEG-TS consumer, skipping audio: %v", codecNames)
		}
	}

	log.Printf("[MPEGTSConsumer] Started consumer for service %s (video + %d audio)", serviceName, len(receivers)-1)

	return &MPEGTSConsumer{
		consumer:  consumer,
		receivers: receivers,
	}, nil
}

// Stop stops the consumer
func (c *MPEGTSConsumer) Stop() {
	c.consumer.Stop()
}

// WriteTo writes all data to the writer until error or EOF
func (c *MPEGTSConsumer) WriteTo(w io.Writer) (int64, error) {
	log.Printf("[MPEGTSConsumer] Starting WriteTo, consumer=%v", c.consumer != nil)

	// Wrap writer to monitor progress
	watched := &monitoredWriter{w: w, start: time.Now()}
	written, err := c.consumer.WriteTo(watched)

	log.Printf("[MPEGTSConsumer] WriteTo finished: written=%d err=%v duration=%v", written, err, time.Since(watched.start))
	return written, err
}

// monitoredWriter wraps io.Writer to log write activity
type monitoredWriter struct {
	w     io.Writer
	total int64
	start time.Time
	lastLog time.Time
}

func (m *monitoredWriter) Write(p []byte) (int, error) {
	n, err := m.w.Write(p)
	m.total += int64(n)

	// Log every 5 seconds
	if time.Since(m.lastLog) > 5*time.Second {
		if m.total > 0 {
			log.Printf("[MPEGTSConsumer] Written so far: %d bytes", m.total)
		} else {
			log.Printf("[MPEGTSConsumer] WARNING: No data from go2rtc consumer yet")
		}
		m.lastLog = time.Now()
	}

	return n, err
}
