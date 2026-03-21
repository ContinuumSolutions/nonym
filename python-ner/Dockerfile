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

# Pre-download model weights at build time so the container starts quickly
RUN python3 -c "from gliner import GLiNER; GLiNER.from_pretrained('urchade/gliner_medium-v2.1')"

CMD ["python", "gliner_server.py"]
