from __future__ import annotations

import io
import os
from typing import Any

import numpy as np
import onnxruntime as ort
from fastapi import FastAPI, HTTPException, Request
from PIL import Image, ImageOps

MODEL_ID = "clip-vit-b32"
DIM = 512
MODEL_PATH = os.environ.get("OK_FOLIO_EMBEDDER_MODEL", "/models/clip-vit-b32-visual.onnx")

app = FastAPI(title="OK Folio CLIP embedder")
session: ort.InferenceSession | None = None
input_name = ""


def load_session() -> None:
    global session, input_name
    if not os.path.exists(MODEL_PATH):
        raise RuntimeError(f"model file not found: {MODEL_PATH}")
    session = ort.InferenceSession(MODEL_PATH, providers=["CPUExecutionProvider"])
    inputs = session.get_inputs()
    if not inputs:
        raise RuntimeError("model has no inputs")
    input_name = inputs[0].name


@app.on_event("startup")
def startup() -> None:
    load_session()


@app.get("/health")
def health() -> dict[str, Any]:
    return {"ok": session is not None, "model": MODEL_ID, "dim": DIM}


@app.post("/embed")
async def embed(request: Request) -> dict[str, Any]:
    if session is None:
        raise HTTPException(status_code=503, detail="model is not loaded")
    data = await request.body()
    if not data:
        raise HTTPException(status_code=400, detail="empty image body")
    try:
        tensor = preprocess(data)
        outputs = session.run(None, {input_name: tensor})
        vector = select_embedding(outputs)
    except HTTPException:
        raise
    except Exception as exc:
        raise HTTPException(status_code=400, detail=f"embedding failed: {exc}") from exc
    norm = float(np.linalg.norm(vector))
    if norm == 0:
        raise HTTPException(status_code=500, detail="model returned a zero vector")
    vector = (vector / norm).astype(np.float32)
    return {"embedding": vector.tolist(), "model": MODEL_ID, "dim": DIM}


def preprocess(data: bytes) -> np.ndarray:
    image = Image.open(io.BytesIO(data))
    image = ImageOps.exif_transpose(image).convert("RGB")
    image = ImageOps.fit(image, (224, 224), method=Image.Resampling.BICUBIC, centering=(0.5, 0.5))
    arr = np.asarray(image).astype(np.float32) / 255.0
    mean = np.asarray([0.48145466, 0.4578275, 0.40821073], dtype=np.float32)
    std = np.asarray([0.26862954, 0.26130258, 0.27577711], dtype=np.float32)
    arr = (arr - mean) / std
    arr = np.transpose(arr, (2, 0, 1))
    return np.expand_dims(arr, axis=0).astype(np.float32)


def select_embedding(outputs: list[np.ndarray]) -> np.ndarray:
    for output in outputs:
        arr = np.asarray(output)
        if arr.ndim == 2 and arr.shape[0] == 1 and arr.shape[1] == DIM:
            return arr[0].astype(np.float32)
        if arr.ndim == 1 and arr.shape[0] == DIM:
            return arr.astype(np.float32)
    shapes = ", ".join(str(np.asarray(output).shape) for output in outputs)
    raise RuntimeError(f"no {DIM}-dim embedding output found; model outputs: {shapes}")
