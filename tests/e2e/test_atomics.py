"""Tests for the atomics catalogue endpoints."""


def test_list_atomics(client):
    r = client.get("/api/v1/atomics")
    assert r.status_code == 200, r.text
    data = r.json()
    assert isinstance(data, list)
    assert len(data) >= 1
    ids = [item.get("id") or item.get("ID") for item in data]
    assert "T1082" in ids


def test_list_atomics_filter_tactic(client):
    # Filter is supported but the fixture atomic has no tactic, so an
    # unmatched filter must still respond 200 with a (possibly empty) list.
    r = client.get("/api/v1/atomics", params={"tactic": "discovery"})
    assert r.status_code == 200, r.text
    assert isinstance(r.json(), list)


def test_get_atomic_found(client):
    r = client.get("/api/v1/atomics/T1082")
    assert r.status_code == 200, r.text
    data = r.json()
    # The detail endpoint returns Go-style capitalized fields.
    assert data.get("ID") == "T1082" or data.get("id") == "T1082"
    tests = data.get("Tests") or data.get("tests")
    assert tests and len(tests) >= 1


def test_get_atomic_not_found(client):
    r = client.get("/api/v1/atomics/T9999")
    assert r.status_code == 404
