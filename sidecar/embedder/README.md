# OK Folio Embedder Sidecar

CPU-only FastAPI service for CLIP ViT-B/32 image embeddings. The Docker build
downloads the ONNX model into the image; the runtime container does not need
network egress.

Endpoints:

- `GET /health`
- `POST /embed` with JPEG bytes
