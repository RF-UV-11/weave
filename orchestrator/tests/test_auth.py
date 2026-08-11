import time

import jwt
import pytest

import server.auth as auth_module
from server.auth import InvalidTokenError, bearer_token_from_metadata, verify_access_token

SECRET = "test-secret-not-for-prod-but-long-enough-for-hs256"


@pytest.fixture(autouse=True)
def _secret(monkeypatch):
    monkeypatch.setattr(auth_module, "JWT_SECRET", SECRET)


def make_token(**overrides):
    now = int(time.time())
    payload = {
        "tenant_id": "tnt_1",
        "user_id": "usr_1",
        "role": "owner",
        "typ": "access",
        "iat": now,
        "exp": now + 900,
    }
    payload.update(overrides)
    return jwt.encode(payload, SECRET, algorithm="HS256")


def test_verify_access_token_accepts_valid_token():
    claims = verify_access_token(make_token())
    assert claims.tenant_id == "tnt_1"
    assert claims.user_id == "usr_1"
    assert claims.role == "owner"


def test_verify_access_token_rejects_refresh_token():
    with pytest.raises(InvalidTokenError, match="not an access token"):
        verify_access_token(make_token(typ="refresh"))


def test_verify_access_token_rejects_expired_token():
    with pytest.raises(InvalidTokenError):
        verify_access_token(make_token(exp=int(time.time()) - 10))


def test_verify_access_token_rejects_wrong_secret():
    token = jwt.encode(
        {"tenant_id": "tnt_1", "user_id": "usr_1", "role": "owner", "typ": "access"},
        "a-completely-different-secret-of-sufficient-length",
        algorithm="HS256",
    )
    with pytest.raises(InvalidTokenError):
        verify_access_token(token)


def test_verify_access_token_rejects_missing_claims():
    now = int(time.time())
    token = jwt.encode({"typ": "access", "iat": now, "exp": now + 900}, SECRET, algorithm="HS256")
    with pytest.raises(InvalidTokenError, match="missing required claims"):
        verify_access_token(token)


def test_bearer_token_from_metadata_extracts_token():
    token = bearer_token_from_metadata((("authorization", "Bearer abc123"),))
    assert token == "abc123"


def test_bearer_token_from_metadata_missing_raises():
    with pytest.raises(InvalidTokenError):
        bearer_token_from_metadata((("x-other", "value"),))


def test_bearer_token_from_metadata_case_insensitive_key():
    token = bearer_token_from_metadata((("Authorization", "Bearer xyz"),))
    assert token == "xyz"
