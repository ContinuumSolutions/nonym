"""
GLiNER gRPC server – wraps the GLiNER ML model to provide Named Entity
Recognition as a gRPC service consumed by the Go gateway.
"""

from concurrent import futures
import logging
import os
import sys

import grpc
import ner_pb2
import ner_pb2_grpc

logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s [%(levelname)s] %(message)s",
)
log = logging.getLogger(__name__)

# Default labels used when the client sends none
DEFAULT_LABELS = [
    "person",
    "location",
    "organization",
    "date",
    "time",
    "money",
    "phone number",
    "address",
]


def _load_model(model_name: str):
    """Load GLiNER model, raising a clear error if the package is missing."""
    try:
        from gliner import GLiNER  # noqa: PLC0415
    except ImportError:
        log.error("gliner package not found – install it via pip install gliner")
        sys.exit(1)
    log.info("Loading GLiNER model: %s", model_name)
    model = GLiNER.from_pretrained(model_name)
    log.info("Model loaded successfully")
    return model


def _annotate_text(model, text: str, labels: list[str], threshold: float):
    """Run the model and return a list of entity dicts."""
    if not labels:
        labels = DEFAULT_LABELS
    if threshold <= 0:
        threshold = 0.5
    entities = model.predict_entities(text, labels, threshold=threshold)
    return entities


def _build_response(text: str, entities: list[dict]) -> ner_pb2.AnnotateResponse:
    tokens = text.split()
    spans = [
        ner_pb2.Span(
            start=e["start"],
            end=e["end"],
            label=e["label"],
            score=e.get("score", 1.0),
            text=e["text"],
        )
        for e in entities
    ]
    return ner_pb2.AnnotateResponse(text=text, tokens=tokens, spans=spans)


class NERServiceServicer(ner_pb2_grpc.NERServiceServicer):
    def __init__(self, model):
        self._model = model

    def Annotate(self, request, context):
        try:
            labels = list(request.labels) if request.labels else DEFAULT_LABELS
            entities = _annotate_text(
                self._model, request.text, labels, request.threshold
            )
            return _build_response(request.text, entities)
        except Exception as exc:  # noqa: BLE001
            log.exception("Annotate error")
            context.set_details(str(exc))
            context.set_code(grpc.StatusCode.INTERNAL)
            return ner_pb2.AnnotateResponse()

    def BatchAnnotate(self, request, context):
        responses = []
        labels = list(request.labels) if request.labels else DEFAULT_LABELS
        for text in request.texts:
            try:
                entities = _annotate_text(
                    self._model, text, labels, request.threshold
                )
                responses.append(_build_response(text, entities))
            except Exception as exc:  # noqa: BLE001
                log.exception("BatchAnnotate error on text: %s", text[:80])
                context.set_details(str(exc))
                context.set_code(grpc.StatusCode.INTERNAL)
                return ner_pb2.BatchAnnotateResponse()
        return ner_pb2.BatchAnnotateResponse(annotations=responses)


def serve():
    # model_name = os.environ.get("GLINER_MODEL", "urchade/gliner_medium-v2.1")
    model_name = os.environ.get("GLINER_MODEL", "knowledgator/gliner-pii-base-v1.0")
    port = os.environ.get("NER_GRPC_PORT", "50051")
    max_workers = int(os.environ.get("NER_MAX_WORKERS", "4"))

    model = _load_model(model_name)

    server = grpc.server(futures.ThreadPoolExecutor(max_workers=max_workers))
    ner_pb2_grpc.add_NERServiceServicer_to_server(NERServiceServicer(model), server)
    server.add_insecure_port(f"[::]:{port}")
    log.info("GLiNER gRPC server listening on port %s", port)
    server.start()
    server.wait_for_termination()


if __name__ == "__main__":
    serve()
