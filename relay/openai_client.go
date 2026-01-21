package relay

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"time"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
)

// OpenAIClient handles communication with OpenAI-compatible endpoints
type OpenAIClient struct {
	*openai.Client
}

// NewOpenAIClient creates a new OpenAI client
func NewOpenAIClient(endpointURL string, timeout time.Duration, model string, apiKey string) *OpenAIClient {
	opts := []option.RequestOption{
		option.WithAPIKey(apiKey),
	}

	if endpointURL != "" {
		opts = append(opts, option.WithBaseURL(endpointURL))
	}

	client := openai.NewClient(opts...)

	return &OpenAIClient{
		Client: &client,
	}
}

// SendFrameBatch sends a batch of frames to the OpenAI endpoint
func (c *OpenAIClient) SendFrameBatch(ctx context.Context, frames []*Frame, instruction string, model string) (*openai.ChatCompletion, error) {
	if len(frames) == 0 {
		return nil, fmt.Errorf("no frames to send")
	}

	// Build message content with images and text
	content := []openai.ChatCompletionContentPartUnionParam{}

	// Add text instruction
	if instruction != "" {
		content = append(content, openai.TextContentPart(instruction))
	}

	// Add images
	for _, frame := range frames {
		// Encode JPEG to base64
		base64Data := base64.StdEncoding.EncodeToString(frame.Data)
		dataURL := fmt.Sprintf("data:image/jpeg;base64,%s", base64Data)

		content = append(content, openai.ImageContentPart(openai.ChatCompletionContentPartImageImageURLParam{
			URL: dataURL,
		}))
	}

	// Build request
	params := openai.ChatCompletionNewParams{
		Model: openai.ChatModel(model),
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage(content),
		},
		MaxTokens: openai.Int(500), // Reasonable default
	}

	// Send request
	startTime := time.Now()
	// TODO: Use the passed context instead of relying entirely on http client timeout?
	// sharedClient.CreateChatCompletion calls openai-go which uses context.
	response, err := c.Chat.Completions.New(ctx, params)
	if err != nil {
		return nil, err
	}
	duration := time.Since(startTime)

	log.Printf("[OpenAIClient] Request successful: duration=%v, frames=%d, tokens=%d",
		duration, len(frames), response.Usage.TotalTokens)

	if len(response.Choices) > 0 {
		log.Printf("[OpenAIClient] Response content: %s", response.Choices[0].Message.Content)
	}

	return response, nil
}
