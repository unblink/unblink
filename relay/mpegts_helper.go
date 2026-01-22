package relay

import (
	"fmt"
	"io"
	"log"

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
	// Supported: AAC, PCMU (G.711 u-law), PCMA (G.711 A-law), OPUS
	var audioMedia *core.Media
	for _, media := range producer.GetMedias() {
		if media.Kind == core.KindAudio {
			audioMedia = media
			break
		}
	}

	if audioMedia != nil {
		supportedCodecs := []string{
			core.CodecAAC,
			core.CodecPCMU,
			core.CodecPCMA,
			core.CodecOpus,
			"opus",
			"PCMU",
			"PCMA",
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
	return c.consumer.WriteTo(w)
}
