import subprocess
from types import SimpleNamespace

import httpx
import pymupdf

from attachments import process
from attachments.process import process_attachments


def _attachment(mime_type: str, data: bytes, filename: str = "") -> SimpleNamespace:
    return SimpleNamespace(mime_type=mime_type, data=data, filename=filename)


def _blank_pdf_bytes(num_pages: int = 1) -> bytes:
    """A PDF with pages but no text layer — the scanned-document case
    process_attachments falls back to rendering as images for."""
    doc = pymupdf.open()
    for _ in range(num_pages):
        doc.new_page(width=200, height=200)
    return doc.tobytes()


def _text_pdf_bytes(text: str) -> bytes:
    doc = pymupdf.open()
    page = doc.new_page(width=300, height=300)
    page.insert_text((50, 50), text)
    return doc.tobytes()


async def test_no_attachments_returns_empty_result():
    result = await process_attachments([])
    assert result.text_note == ""
    assert result.images == []


async def test_image_attachment_is_base64_encoded_and_not_in_text_note():
    result = await process_attachments([_attachment("image/jpeg", b"fake-jpeg-bytes", "photo.jpg")])
    assert result.text_note == ""
    assert len(result.images) == 1
    assert result.images[0].mime_type == "image/jpeg"
    assert result.images[0].data_b64 == "ZmFrZS1qcGVnLWJ5dGVz"


async def test_pdf_with_a_text_layer_is_extracted_as_text_not_images():
    result = await process_attachments([_attachment("application/pdf", _text_pdf_bytes("hello weave"), "note.pdf")])
    assert result.images == []
    assert "'note.pdf'" in result.text_note
    assert "hello weave" in result.text_note


async def test_scanned_pdf_falls_back_to_rendering_pages_as_images():
    result = await process_attachments([_attachment("application/pdf", _blank_pdf_bytes(2), "statement.pdf")])
    assert len(result.images) == 2
    assert all(img.mime_type == "image/png" for img in result.images)
    assert "'statement.pdf'" in result.text_note
    assert "no text layer" in result.text_note
    assert "2 page(s)" in result.text_note


async def test_ocr_fallback_caps_pages_rendered():
    result = await process_attachments(
        [_attachment("application/pdf", _blank_pdf_bytes(process._MAX_OCR_FALLBACK_PAGES + 3), "big.pdf")]
    )
    assert len(result.images) == process._MAX_OCR_FALLBACK_PAGES


async def test_audio_attachment_is_transcribed_via_openai_compatible_endpoint(monkeypatch):
    def handler(request: httpx.Request) -> httpx.Response:
        assert request.url.path.endswith("/audio/transcriptions")
        return httpx.Response(200, json={"text": "please check my balance"})

    real_async_client = httpx.AsyncClient
    monkeypatch.setattr(httpx, "AsyncClient", lambda **kw: real_async_client(transport=httpx.MockTransport(handler)))

    result = await process_attachments([_attachment("audio/ogg", b"fake-audio", "note.ogg")])
    assert result.images == []
    assert "please check my balance" in result.text_note
    assert "'note.ogg'" in result.text_note


async def test_video_attachment_is_transcoded_with_bundled_ffmpeg_then_transcribed(monkeypatch):
    def fake_run(cmd, **kwargs):
        # ffmpeg's real output path is the last argument; write real WAV
        # header bytes there so the "transcription" step downstream has
        # something to read, without needing a real video file or a real
        # ffmpeg invocation in this test.
        with open(cmd[-1], "wb") as f:
            f.write(b"RIFF....WAVEfmt ")
        return subprocess.CompletedProcess(cmd, 0)

    monkeypatch.setattr(process.subprocess, "run", fake_run)

    async def fake_transcribe(audio_bytes, mime_type, filename):
        assert mime_type == "audio/wav"
        return "transcribed video audio"

    monkeypatch.setattr(process, "_transcribe", fake_transcribe)

    result = await process_attachments([_attachment("video/mp4", b"fake-video-bytes", "clip.mp4")])
    assert result.images == []
    assert "transcribed video audio" in result.text_note
    assert "'clip.mp4'" in result.text_note


async def test_video_attachment_failure_is_noted_without_raising(monkeypatch):
    def fake_run(cmd, **kwargs):
        raise subprocess.CalledProcessError(1, cmd, stderr=b"invalid data found")

    monkeypatch.setattr(process.subprocess, "run", fake_run)

    result = await process_attachments([_attachment("video/mp4", b"not-really-a-video", "clip.mp4")])
    assert result.images == []
    assert "could not be processed" in result.text_note
    assert "'clip.mp4'" in result.text_note


async def test_unsupported_mime_type_is_dropped_with_a_note():
    result = await process_attachments([_attachment("application/zip", b"PK\x03\x04", "archive.zip")])
    assert result.images == []
    assert "unsupported type" in result.text_note


async def test_one_bad_attachment_does_not_block_a_good_one():
    result = await process_attachments(
        [
            _attachment("application/zip", b"PK\x03\x04", "archive.zip"),
            _attachment("image/png", b"fake-png", "ok.png"),
        ]
    )
    assert len(result.images) == 1
    assert result.images[0].mime_type == "image/png"
    assert "archive.zip" in result.text_note


async def test_multiple_attachments_join_notes_with_blank_line():
    result = await process_attachments(
        [
            _attachment("application/pdf", _text_pdf_bytes("first doc"), "a.pdf"),
            _attachment("application/pdf", _text_pdf_bytes("second doc"), "b.pdf"),
        ]
    )
    assert result.text_note.count("\n\n") == 1
    assert "'a.pdf'" in result.text_note
    assert "'b.pdf'" in result.text_note
