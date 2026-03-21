# Improve NER Engine from Regex based to ML

## Project setup

## Proposed setup for NER interface with Python. Both the main go app and the python app will be running in docker containers. gPRC will be used for communication because it is fast and efficient when it comes to memory.

    our_project/
    ├── pkg
    |   |__ ner
        │   └── ner_pb/          # generated from ner.proto
    ├── python-server/
    │   ├── gliner_server.py
    │   ├── requirements.txt
    │   └── ner_pb2*.py      # generated from ner.proto
    ├── ner.proto
    ├── docker-compose.yml


## ner.proto (gPRC Service)

    syntax = "proto3";

    package ner;

    message AnnotateRequest {
        string text = 1;
    }

    message Span {
        int32 start = 1;
        int32 end = 2;
        string label = 3;
    }

    message AnnotateResponse {
        string text = 1;
        repeated string tokens = 2;
        repeated Span spans = 3;
    }

    message BatchAnnotateRequest {
        repeated string texts = 1;
    }

    message BatchAnnotateResponse {
        repeated AnnotateResponse annotations = 1;
    }

    service NERService {
        rpc Annotate(AnnotateRequest) returns (AnnotateResponse);
        rpc BatchAnnotate(BatchAnnotateRequest) returns (BatchAnnotateResponse);
    }

## Python server

### python-server/requirements.txt

    grpcio==1.59.0
    grpcio-tools==1.59.0
    gliner==<latest_version>

### python-server/gliner_server.py

    from concurrent import futures
    import grpc
    import ner_pb2
    import ner_pb2_grpc
    from gliner import Annotation, EntityLabel

    class NERService(ner_pb2_grpc.NERServiceServicer):
        def Annotate(self, request, context):
            ann = Annotation(request.text)
            words = request.text.split()
            if words:
                ann.add_span(0, 1, EntityLabel.PERSON)
            return ner_pb2.AnnotateResponse(
                text=ann.text,
                tokens=ann.tokens,
                spans=[ner_pb2.Span(start=s.start, end=s.end, label=s.label.value) for s in ann.spans]
            )

        def BatchAnnotate(self, request, context):
            responses = []
            for text in request.texts:
                ann = Annotation(text)
                words = text.split()
                if words:
                    ann.add_span(0, 1, EntityLabel.PERSON)
                responses.append(
                    ner_pb2.AnnotateResponse(
                        text=ann.text,
                        tokens=ann.tokens,
                        spans=[ner_pb2.Span(start=s.start, end=s.end, label=s.label.value) for s in ann.spans]
                    )
                )
            return ner_pb2.BatchAnnotateResponse(annotations=responses)

    def serve():
        server = grpc.server(futures.ThreadPoolExecutor(max_workers=10))
        ner_pb2_grpc.add_NERServiceServicer_to_server(NERService(), server)
        server.add_insecure_port('[::]:50051')
        print("Python gRPC Gliner server running on port 50051")
        server.start()
        server.wait_for_termination()

    if __name__ == "__main__":
        serve()


### python-server/Dockerfile

    FROM python:3.11-slim

    WORKDIR /app

    COPY requirements.txt .
    RUN pip install --no-cache-dir -r requirements.txt

    COPY gliner_server.py .
    COPY ner_pb2*.py .

    EXPOSE 50051

    CMD ["python", "gliner_server.py"]


### Golang Code

    package main

    import (
        "context"
        "fmt"
        "log"
        "os"
        "time"

        pb "github.com/yourusername/ner_project/go-client/ner_pb"

        "google.golang.org/grpc"
    )

    func main() {
        host := os.Getenv("PYTHON_SERVER_HOST")
        port := os.Getenv("PYTHON_SERVER_PORT")

        conn, err := grpc.Dial(fmt.Sprintf("%s:%s", host, port), grpc.WithInsecure())
        if err != nil {
            log.Fatalf("Failed to connect: %v", err)
        }
        defer conn.Close()

        client := pb.NewNERServiceClient(conn)

        ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
        defer cancel()

        // Example: Single annotation
        resp, err := client.Annotate(ctx, &pb.AnnotateRequest{Text: "John lives in Amsterdam."})
        if err != nil {
            log.Fatalf("Annotate error: %v", err)
        }

        fmt.Println("Tokens:", resp.Tokens)
        for _, s := range resp.Spans {
            fmt.Printf("Entity: %s [%d-%d]\n", s.Label, s.Start, s.End)
        }

        // Example: Batch annotation
        batchResp, err := client.BatchAnnotate(ctx, &pb.BatchAnnotateRequest{
            Texts: []string{"Alice went to Paris", "Bob is in Nairobi"},
        })
        if err != nil {
            log.Fatalf("BatchAnnotate error: %v", err)
        }

        for _, ann := range batchResp.Annotations {
            fmt.Println("Text:", ann.Text)
            fmt.Println("Tokens:", ann.Tokens)
            for _, s := range ann.Spans {
                fmt.Printf("Entity: %s [%d-%d]\n", s.Label, s.Start, s.End)
            }
        }
    }