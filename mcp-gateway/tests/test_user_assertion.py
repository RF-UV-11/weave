import time

import jwt
import pytest

import gateway.user_assertion as user_assertion_module
from gateway.user_assertion import InvalidUserAssertionError, verify_user_assertion

SECRET = "test-secret-not-for-prod-but-long-enough-for-hs256"


@pytest.fixture(autouse=True)
def _secret(monkeypatch):
    monkeypatch.setattr(user_assertion_module, "JWT_SECRET", SECRET)


def make_token(**overrides):
    now = int(time.time())
    payload = {"typ": "user_assertion", "tenant_id": "tnt_1", "user_id": "usr_1", "iat": now, "exp": now + 60}
    payload.update(overrides)
    return jwt.encode(payload, SECRET, algorithm="HS256")


def test_accepts_valid_assertion_for_expected_tenant():
    user_id = verify_user_assertion(make_token(), expected_tenant_id="tnt_1")
    assert user_id == "usr_1"


def test_rejects_assertion_for_a_different_tenant():
    with pytest.raises(InvalidUserAssertionError, match="different tenant"):
        verify_user_assertion(make_token(tenant_id="tnt_1"), expected_tenant_id="tnt_OTHER")


def test_rejects_expired_assertion():
    with pytest.raises(InvalidUserAssertionError):
        verify_user_assertion(make_token(exp=int(time.time()) - 10), expected_tenant_id="tnt_1")


def test_rejects_wrong_typ():
    token = jwt.encode(
        {"typ": "access", "tenant_id": "tnt_1", "user_id": "usr_1", "iat": int(time.time()), "exp": int(time.time()) + 60},
        SECRET,
        algorithm="HS256",
    )
    with pytest.raises(InvalidUserAssertionError, match="not a user assertion"):
        verify_user_assertion(token, expected_tenant_id="tnt_1")


def test_rejects_wrong_secret():
    token = jwt.encode(
        {"typ": "user_assertion", "tenant_id": "tnt_1", "user_id": "usr_1"},
        "a-completely-different-secret-of-sufficient-length",
        algorithm="HS256",
    )
    with pytest.raises(InvalidUserAssertionError):
        verify_user_assertion(token, expected_tenant_id="tnt_1")


def test_rejects_missing_claims():
    now = int(time.time())
    token = jwt.encode({"typ": "user_assertion", "iat": now, "exp": now + 60}, SECRET, algorithm="HS256")
    with pytest.raises(InvalidUserAssertionError, match="missing required claims"):
        verify_user_assertion(token, expected_tenant_id="tnt_1")


def test_requires_jwt_secret_configured(monkeypatch):
    monkeypatch.setattr(user_assertion_module, "JWT_SECRET", "")
    with pytest.raises(InvalidUserAssertionError, match="JWT_SECRET"):
        verify_user_assertion(make_token(), expected_tenant_id="tnt_1")
