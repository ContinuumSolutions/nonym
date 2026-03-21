FROM python:3.11-slim

WORKDIR /app

# Install dependencies first for better layer caching
COPY python-ner/requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

# Copy generated protobuf stubs and server
COPY python-ner/ner_pb2.py .
COPY python-ner/ner_pb2_grpc.py .
COPY python-ner/gliner_server.py .

EXPOSE 50051

ENV GLINER_MODEL=urchade/gliner_medium-v2.1
ENV NER_GRPC_PORT=50051
ENV NER_MAX_WORKERS=4

# Model weights are downloaded on first start and cached in HF_HOME.
# Mount a volume at /root/.cache/huggingface to persist across restarts.

CMD ["python", "gliner_server.py"]
