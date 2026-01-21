package relay

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"time"

	"connectrpc.com/connect"
	elizav1 "github.com/unblink/unblink/relay/gen/connectrpc/eliza/v1"
)

// ElizaServer implements the ElizaServiceHandler interface
type ElizaServer struct{}

// Say is a unary RPC that echoes back the user's input
func (s *ElizaServer) Say(
	ctx context.Context,
	req *connect.Request[elizav1.SayRequest],
) (*connect.Response[elizav1.SayResponse], error) {
	log.Printf("[ElizaService] Say received: %s", req.Msg.Sentence)
	res := connect.NewResponse(&elizav1.SayResponse{
		Sentence: fmt.Sprintf("You said: %s", req.Msg.Sentence),
	})
	return res, nil
}

// Introduce is a server-streaming RPC that sends multiple messages
func (s *ElizaServer) Introduce(
	ctx context.Context,
	req *connect.Request[elizav1.IntroduceRequest],
	stream *connect.ServerStream[elizav1.IntroduceResponse],
) error {
	name := req.Msg.Name
	log.Printf("[ElizaService] Introduce request for %s", name)

	messages := []string{
		fmt.Sprintf("Hello, %s!", name),
		"I am Eliza, a therapeutic chatbot.",
		"I am here to listen and help.",
		"Tell me what is on your mind.",
	}

	for _, msg := range messages {
		if err := stream.Send(&elizav1.IntroduceResponse{
			Sentence: msg,
		}); err != nil {
			return err
		}
		time.Sleep(500 * time.Millisecond)
	}
	return nil
}

// Consult is a client-streaming RPC that receives multiple messages
func (s *ElizaServer) Consult(
	ctx context.Context,
	stream *connect.ClientStream[elizav1.ConsultRequest],
) (*connect.Response[elizav1.ConsultResponse], error) {
	log.Println("[ElizaService] Consult (Client Streaming) started")
	var sentences []string
	for stream.Receive() {
		msg := stream.Msg()
		log.Printf("[ElizaService] Received: %s", msg.Sentence)
		sentences = append(sentences, msg.Sentence)
	}
	if err := stream.Err(); err != nil {
		return nil, connect.NewError(connect.CodeUnknown, err)
	}

	response := fmt.Sprintf("You told me %d things: %v", len(sentences), sentences)
	return connect.NewResponse(&elizav1.ConsultResponse{
		Sentence: response,
	}), nil
}

// Chat is a bidirectional streaming RPC
func (s *ElizaServer) Chat(
	ctx context.Context,
	stream *connect.BidiStream[elizav1.ChatRequest, elizav1.ChatResponse],
) error {
	log.Println("[ElizaService] Chat (Bidi Streaming) started")
	for {
		req, err := stream.Receive()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
		log.Printf("[ElizaService] Chat received: %s", req.Sentence)

		// Echo back with a prefix
		if err := stream.Send(&elizav1.ChatResponse{
			Sentence: fmt.Sprintf("You said: %s", req.Sentence),
		}); err != nil {
			return err
		}
	}
}
