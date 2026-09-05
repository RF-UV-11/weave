from gateway.http_signing import sign_user_identity


def test_signature_is_deterministic_for_same_inputs():
    a = sign_user_identity("secret", tenant_id="tnt_1", user_id="usr_1")
    b = sign_user_identity("secret", tenant_id="tnt_1", user_id="usr_1")
    assert a == b


def test_signature_differs_for_different_users():
    a = sign_user_identity("secret", tenant_id="tnt_1", user_id="usr_1")
    b = sign_user_identity("secret", tenant_id="tnt_1", user_id="usr_2")
    assert a != b


def test_signature_differs_for_different_tenants():
    a = sign_user_identity("secret", tenant_id="tnt_1", user_id="usr_1")
    b = sign_user_identity("secret", tenant_id="tnt_2", user_id="usr_1")
    assert a != b


def test_signature_differs_for_different_secrets():
    a = sign_user_identity("secret-a", tenant_id="tnt_1", user_id="usr_1")
    b = sign_user_identity("secret-b", tenant_id="tnt_1", user_id="usr_1")
    assert a != b


def test_signature_is_hex_sha256_length():
    sig = sign_user_identity("secret", tenant_id="tnt_1", user_id="usr_1")
    assert len(sig) == 64  # SHA-256 hex digest
    int(sig, 16)  # raises if not valid hex
