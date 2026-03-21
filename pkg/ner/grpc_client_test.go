package ner

import (
	"context"
	"net"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"

	pb "github.com/ContinuumSolutions/nonym/pkg/ner/ner_pb"
)

const bufSize = 1024 * 1024

// mockNERServer is an in-process gRPC server for testing.
type mockNERServer struct {
	pb.UnimplementedNERServiceServer
}

func (m *mockNERServer) Annotate(_ context.Context, req *pb.AnnotateRequest) (*pb.AnnotateResponse, error) {
	// Return a hard-coded PERSON span for any text containing "John"
	var spans []*pb.Span
	text := req.Text
	idx := -1
	for i := 0; i+4 <= len(text); i++ {
		if text[i:i+4] == "John" {
			idx = i
			break
		}
	}
	if idx >= 0 {
		spans = append(spans, &pb.Span{
			Start: int32(idx),
			End:   int32(idx + 4),
			Label: "person",
			Score: 0.95,
			Text:  "John",
		})
	}
	return &pb.AnnotateResponse{
		Text:   text,
		Tokens: splitWords(text),
		Spans:  spans,
	}, nil
}

func (m *mockNERServer) BatchAnnotate(_ context.Context, req *pb.BatchAnnotateRequest) (*pb.BatchAnnotateResponse, error) {
	var annotations []*pb.AnnotateResponse
	for _, text := range req.Texts {
		ann, _ := (&mockNERServer{}).Annotate(context.Background(), &pb.AnnotateRequest{Text: text})
		annotations = append(annotations, ann)
	}
	return &pb.BatchAnnotateResponse{Annotations: annotations}, nil
}

func splitWords(s string) []string {
	var words []string
	word := ""
	for _, ch := range s {
		if ch == ' ' {
			if word != "" {
				words = append(words, word)
				word = ""
			}
		} else {
			word += string(ch)
		}
	}
	if word != "" {
		words = append(words, word)
	}
	return words
}

// newMockGRPCClient creates an in-memory gRPC connection backed by the mock server.
func newMockGRPCClient(t *testing.T) {
	t.Helper()
	lis := bufconn.Listen(bufSize)
	srv := grpc.NewServer()
	pb.RegisterNERServiceServer(srv, &mockNERServer{})
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(func() { srv.Stop(); lis.Close() })

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("failed to create in-memory gRPC connection: %v", err)
	}
	t.Cleanup(func() { conn.Close() })

	// Inject directly into the global client
	globalGRPCClient = &NERGRPCClient{
		conn:   conn,
		client: pb.NewNERServiceClient(conn),
	}
}

func TestAnnotateML_DetectsPerson(t *testing.T) {
	newMockGRPCClient(t)

	spans, err := AnnotateML("John lives in Amsterdam.", nil, 0)
	if err != nil {
		t.Fatalf("AnnotateML error: %v", err)
	}
	if len(spans) == 0 {
		t.Fatal("expected at least one span")
	}
	found := false
	for _, s := range spans {
		if s.Label == "person" && s.Text == "John" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected person span for 'John', got %+v", spans)
	}
}

func TestAnnotateML_NoEntities(t *testing.T) {
	newMockGRPCClient(t)

	spans, err := AnnotateML("The sky is blue.", nil, 0)
	if err != nil {
		t.Fatalf("AnnotateML error: %v", err)
	}
	if len(spans) != 0 {
		t.Errorf("expected no spans, got %+v", spans)
	}
}

func TestBatchAnnotateML(t *testing.T) {
	newMockGRPCClient(t)

	texts := []string{
		"John went to Paris.",
		"The weather is nice.",
	}
	results, err := BatchAnnotateML(texts, nil, 0)
	if err != nil {
		t.Fatalf("BatchAnnotateML error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if len(results[0]) == 0 {
		t.Error("expected spans for 'John' in first text")
	}
	if len(results[1]) != 0 {
		t.Errorf("expected no spans in second text, got %+v", results[1])
	}
}

func TestMLLabelToEntityType(t *testing.T) {
	cases := []struct {
		label    string
		expected EntityType
	}{
		{"person", EntityPerson},
		{"location", EntityLocation},
		{"organization", EntityOrganization},
		{"custom", EntityType("custom")},
	}
	for _, tc := range cases {
		got := mlLabelToEntityType(tc.label)
		if got != tc.expected {
			t.Errorf("mlLabelToEntityType(%q) = %q, want %q", tc.label, got, tc.expected)
		}
	}
}
