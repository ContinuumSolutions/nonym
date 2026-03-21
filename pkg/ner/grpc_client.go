package ner

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"

	pb "github.com/ContinuumSolutions/nonym/pkg/ner/ner_pb"
)

// MLSpan represents a span detected by the ML NER service
type MLSpan struct {
	Start int
	End   int
	Label string
	Score float64
	Text  string
}

// NERGRPCClient wraps the gRPC connection to the Python GLiNER server
type NERGRPCClient struct {
	conn   *grpc.ClientConn
	client pb.NERServiceClient
	mu     sync.RWMutex
}

var (
	globalGRPCClient *NERGRPCClient
	grpcOnce         sync.Once
)

// DefaultMLLabels are the entity labels sent to the GLiNER model
var DefaultMLLabels = []string{
	"person",
	"location",
	"organization",
	"date",
	"time",
	"money",
	"phone number",
	"address",
}

// InitGRPCClient initialises the gRPC connection to the NER Python server.
// It is safe to call multiple times; only the first call creates the connection.
func InitGRPCClient() error {
	host := os.Getenv("NER_GRPC_HOST")
	if host == "" {
		host = "localhost"
	}
	port := os.Getenv("NER_GRPC_PORT")
	if port == "" {
		port = "50051"
	}
	addr := fmt.Sprintf("%s:%s", host, port)

	var initErr error
	grpcOnce.Do(func() {
		conn, err := grpc.NewClient(addr,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		)
		if err != nil {
			initErr = fmt.Errorf("failed to create gRPC client: %w", err)
			return
		}
		globalGRPCClient = &NERGRPCClient{
			conn:   conn,
			client: pb.NewNERServiceClient(conn),
		}
		log.Printf("NER gRPC client connected to %s", addr)
	})
	return initErr
}

// IsGRPCAvailable returns true when the gRPC connection is ready.
func IsGRPCAvailable() bool {
	if globalGRPCClient == nil {
		return false
	}
	globalGRPCClient.mu.RLock()
	defer globalGRPCClient.mu.RUnlock()
	state := globalGRPCClient.conn.GetState()
	return state == connectivity.Ready || state == connectivity.Idle
}

// AnnotateML calls the Python GLiNER gRPC server and returns detected spans.
func AnnotateML(text string, labels []string, threshold float32) ([]MLSpan, error) {
	if globalGRPCClient == nil {
		return nil, fmt.Errorf("gRPC client not initialized")
	}
	if len(labels) == 0 {
		labels = DefaultMLLabels
	}
	if threshold == 0 {
		threshold = 0.5
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	resp, err := globalGRPCClient.client.Annotate(ctx, &pb.AnnotateRequest{
		Text:      text,
		Labels:    labels,
		Threshold: threshold,
	})
	if err != nil {
		return nil, fmt.Errorf("gRPC Annotate error: %w", err)
	}

	spans := make([]MLSpan, 0, len(resp.Spans))
	for _, s := range resp.Spans {
		spans = append(spans, MLSpan{
			Start: int(s.Start),
			End:   int(s.End),
			Label: s.Label,
			Score: float64(s.Score),
			Text:  s.Text,
		})
	}
	return spans, nil
}

// BatchAnnotateML calls the Python GLiNER gRPC server for multiple texts.
func BatchAnnotateML(texts []string, labels []string, threshold float32) ([][]MLSpan, error) {
	if globalGRPCClient == nil {
		return nil, fmt.Errorf("gRPC client not initialized")
	}
	if len(labels) == 0 {
		labels = DefaultMLLabels
	}
	if threshold == 0 {
		threshold = 0.5
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := globalGRPCClient.client.BatchAnnotate(ctx, &pb.BatchAnnotateRequest{
		Texts:     texts,
		Labels:    labels,
		Threshold: threshold,
	})
	if err != nil {
		return nil, fmt.Errorf("gRPC BatchAnnotate error: %w", err)
	}

	results := make([][]MLSpan, 0, len(resp.Annotations))
	for _, ann := range resp.Annotations {
		spans := make([]MLSpan, 0, len(ann.Spans))
		for _, s := range ann.Spans {
			spans = append(spans, MLSpan{
				Start: int(s.Start),
				End:   int(s.End),
				Label: s.Label,
				Score: float64(s.Score),
				Text:  s.Text,
			})
		}
		results = append(results, spans)
	}
	return results, nil
}

// CloseGRPCClient closes the gRPC connection.
func CloseGRPCClient() {
	if globalGRPCClient != nil {
		globalGRPCClient.mu.Lock()
		defer globalGRPCClient.mu.Unlock()
		globalGRPCClient.conn.Close()
	}
}

// mlLabelToEntityType maps GLiNER label strings to our EntityType constants.
func mlLabelToEntityType(label string) EntityType {
	switch label {
	case "person":
		return EntityPerson
	case "location":
		return EntityLocation
	case "organization":
		return EntityOrganization
	default:
		return EntityType(label)
	}
}
