FROM python:3.12-slim AS model

ARG MODEL_URL=https://huggingface.co/Xenova/clip-vit-base-patch32/resolve/main/onnx/vision_model_quantized.onnx
ARG MODEL_SHA256=583fd1110a514667812fee7d684952aaf82a99b959760c8d7dca7e0ab9839299
RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates curl \
    && rm -rf /var/lib/apt/lists/*
RUN mkdir -p /models \
    && curl -fsSL "$MODEL_URL" -o /models/clip-vit-b32-visual.onnx \
    && if [ -n "$MODEL_SHA256" ]; then echo "$MODEL_SHA256  /models/clip-vit-b32-visual.onnx" | sha256sum -c -; fi

FROM python:3.12-slim

ENV PYTHONUNBUFFERED=1 \
    PYTHONDONTWRITEBYTECODE=1 \
    OK_FOLIO_EMBEDDER_MODEL=/models/clip-vit-b32-visual.onnx

WORKDIR /app
COPY sidecar/embedder/requirements.txt /app/requirements.txt
RUN pip install --no-cache-dir -r /app/requirements.txt
COPY --from=model /models /models
COPY sidecar/embedder/app.py /app/app.py

EXPOSE 8081
CMD ["uvicorn", "app:app", "--host", "0.0.0.0", "--port", "8081"]
