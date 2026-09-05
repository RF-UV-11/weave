"""Turns a ChatStreamRequest's Attachment list into model-ready input:
image bytes for a vision-capable model, and everything else (PDF, audio,
video) reduced to extracted/transcribed text — or, for a PDF with no text
layer, rendered page images fed to that same vision-capable model —
appended to the turn's message. See protos/orchestrator/v1/chat.proto's
Attachment docstring for why images and everything else are handled
differently.

Nothing here is a general media pipeline: it exists to get exactly the
four MIME families a real channel (e.g. cusp_agent's own /chat endpoint)
already accepts — image/audio/video/application-pdf — into a shape
llm/ollama_client.py and llm/openai_compat_client.py can already send to
a model, and no further. Deliberately no Tesseract/poppler dependency:
`pymupdf` renders PDF pages and extracts text using one self-contained
wheel (bundles MuPDF, no system binary to install), and `imageio-ffmpeg`
does the same for video (bundles a static ffmpeg build) — orchestrator
never asks a deployment to apt-get install anything for this module to
work.
"""

import base64
import io
import logging
import os
import subprocess
import tempfile
from dataclasses import dataclass, field
from typing import Any

import httpx
import imageio_ffmpeg
import pymupdf

from llm.openai_compat_client import API_KEY as _OPENAI_API_KEY, BASE_URL as _OPENAI_BASE_URL

logger = logging.getLogger("orchestrator.attachments")

_TRANSCRIBE_TIMEOUT_SECONDS = 60.0
# Whisper's own model id on the OpenAI API; a self-hosted Whisper-
# compatible server on the same OPENAI_BASE_URL may expect a different
# name, hence overridable rather than hardcoded.
_TRANSCRIBE_MODEL = os.environ.get("OPENAI_TRANSCRIBE_MODEL", "whisper-1")
_SUBPROCESS_TIMEOUT_SECONDS = 120.0

# A scanned contract or bank statement is rarely more than a handful of
# pages; capping how many get rendered to images keeps one PDF attachment
# from ballooning a turn's request size (and a vision model's context)
# without a per-caller size limit having to exist yet.
_MAX_OCR_FALLBACK_PAGES = 5


@dataclass
class ImageAttachment:
    mime_type: str
    data_b64: str


@dataclass
class ProcessedAttachments:
    # Appended to the turn's user-visible text (extracted PDF text,
    # audio/video transcripts) — "" when there's nothing to add.
    text_note: str = ""
    # Forwarded to the model as vision input this turn only — see
    # llm/ollama_client.py / llm/openai_compat_client.py for how each
    # provider turns this into its own wire format.
    images: list[ImageAttachment] = field(default_factory=list)


def _b64(data: bytes) -> str:
    return base64.b64encode(data).decode("ascii")


async def _transcribe(audio_bytes: bytes, mime_type: str, filename: str) -> str:
    """Transcribes audio via an OpenAI-compatible `/audio/transcriptions`
    endpoint (Whisper's wire shape) — the same OPENAI_BASE_URL/
    OPENAI_API_KEY llm/openai_compat_client.py already uses, so this needs
    no separate credential or service to stand up: any backend already
    configured for chat completions that also serves this endpoint (OpenAI
    itself, or a self-hosted Whisper-compatible server on the same
    OPENAI_BASE_URL) works with zero extra configuration. Raises on a
    non-2xx response — surfaced to the caller as a failed attachment (see
    process_attachments' per-attachment try/except), not a silently empty
    transcript, so a misconfigured deployment is obvious immediately
    rather than producing confidently wrong answers."""
    headers = {"Authorization": f"Bearer {_OPENAI_API_KEY}"} if _OPENAI_API_KEY else {}
    files = {"file": (filename or "audio", io.BytesIO(audio_bytes), mime_type or "application/octet-stream")}
    data = {"model": _TRANSCRIBE_MODEL}
    async with httpx.AsyncClient(timeout=_TRANSCRIBE_TIMEOUT_SECONDS) as client:
        resp = await client.post(f"{_OPENAI_BASE_URL}/audio/transcriptions", headers=headers, data=data, files=files)
        resp.raise_for_status()
    return resp.json().get("text", "")


def _extract_pdf_text(doc: "pymupdf.Document") -> str:
    """Text-layer extraction — a scanned/image-only PDF yields "", the
    signal `process_attachments` uses to fall back to
    `_render_pdf_pages_as_images` below instead of giving up."""
    return "\n".join(page.get_text() for page in doc).strip()


def _render_pdf_pages_as_images(doc: "pymupdf.Document", max_pages: int = _MAX_OCR_FALLBACK_PAGES) -> list[ImageAttachment]:
    """OCR fallback for a PDF with no text layer: renders each of its
    first `max_pages` pages to a PNG and hands them to the model as
    ordinary vision input — the same path a native image attachment
    takes (llm/ollama_client.py / llm/openai_compat_client.py). This is
    "OCR" via the model's own vision capability rather than a dedicated
    OCR engine: it needs no Tesseract (or any other system binary) to be
    installed, at the cost of depending on the bot profile's configured
    model actually being vision-capable — a text-only model still gets
    the images, just can't read them, same as a human handed a photo of
    a page it can't see."""
    images = []
    for page in doc[:max_pages]:
        # 2x zoom: a 72dpi PDF page renders too small for a vision model
        # to read small print reliably at the default scale.
        pixmap = page.get_pixmap(matrix=pymupdf.Matrix(2, 2))
        images.append(ImageAttachment(mime_type="image/png", data_b64=_b64(pixmap.tobytes("png"))))
    return images


def _extract_audio_from_video(video_bytes: bytes, suffix: str) -> bytes:
    """Shells out to ffmpeg to pull a 16kHz mono WAV track out of a video
    file, then reuses the same transcription path as a native audio
    attachment. The ffmpeg binary comes from `imageio-ffmpeg` (a
    self-contained wheel bundling a static build per platform) rather
    than requiring one already on PATH — no separate host-level install
    step for video attachments to work.

    Both temp files are closed before ffmpeg touches them: on Windows, a
    file opened by this process (e.g. via `NamedTemporaryFile`'s default
    exclusive handle) can't also be opened by the ffmpeg subprocess for
    writing, so this uses `mkstemp` and hands ffmpeg a bare path instead."""
    ffmpeg_exe = imageio_ffmpeg.get_ffmpeg_exe()
    src_fd, src_path = tempfile.mkstemp(suffix=suffix or ".mp4")
    dst_fd, dst_path = tempfile.mkstemp(suffix=".wav")
    os.close(dst_fd)  # ffmpeg opens dst_path itself; this process must not hold it open
    try:
        with os.fdopen(src_fd, "wb") as src:
            src.write(video_bytes)
        subprocess.run(
            [ffmpeg_exe, "-y", "-i", src_path, "-vn", "-ar", "16000", "-ac", "1", "-f", "wav", dst_path],
            check=True,
            capture_output=True,
            timeout=_SUBPROCESS_TIMEOUT_SECONDS,
        )
        with open(dst_path, "rb") as dst:
            return dst.read()
    finally:
        os.unlink(src_path)
        os.unlink(dst_path)


async def process_attachments(attachments: list[Any]) -> ProcessedAttachments:
    """Processes every attachment on one turn. Each item needs only
    `.data` (bytes), `.mime_type` (str), `.filename` (str) attributes —
    a chat_pb2.Attachment in production, any matching object in a test.

    A single attachment's failure (corrupt PDF, unreachable transcription
    backend, unsupported MIME type, an ffmpeg run that errors out) is
    logged and noted in text_note rather than raising — one bad
    attachment shouldn't fail a turn that also asked a plain-text
    question alongside it."""
    result = ProcessedAttachments()
    notes: list[str] = []

    for att in attachments:
        label = att.filename or att.mime_type or "attachment"
        try:
            if att.mime_type.startswith("image/"):
                result.images.append(ImageAttachment(mime_type=att.mime_type, data_b64=_b64(att.data)))
            elif att.mime_type == "application/pdf":
                with pymupdf.open(stream=att.data, filetype="pdf") as doc:
                    text = _extract_pdf_text(doc)
                    if text:
                        notes.append(f"[Attached document {label!r} text]:\n{text}")
                    else:
                        page_images = _render_pdf_pages_as_images(doc)
                        result.images.extend(page_images)
                        notes.append(
                            f"[Attached document {label!r} has no text layer — its "
                            f"first {len(page_images)} page(s) were rendered as images below]"
                        )
            elif att.mime_type.startswith("audio/"):
                text = await _transcribe(att.data, att.mime_type, att.filename)
                notes.append(f"[Transcript of attached audio {label!r}]: {text}")
            elif att.mime_type.startswith("video/"):
                suffix = "." + att.mime_type.split("/")[-1]
                wav_bytes = _extract_audio_from_video(att.data, suffix)
                text = await _transcribe(wav_bytes, "audio/wav", label)
                notes.append(f"[Transcript of attached video {label!r}]: {text}")
            else:
                logger.warning("dropping attachment %r: unsupported mime_type %r", label, att.mime_type)
                notes.append(f"[Attachment {label!r} of unsupported type {att.mime_type!r} was dropped]")
        except Exception as exc:  # noqa: BLE001 — one attachment's failure must not fail the turn
            logger.error("failed to process attachment %r (%s): %s", label, att.mime_type, exc)
            notes.append(f"[Attachment {label!r} could not be processed: {exc}]")

    result.text_note = "\n\n".join(notes)
    return result
