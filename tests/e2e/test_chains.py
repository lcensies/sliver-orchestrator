"""Chain CRUD plus dry-run / execute flow.

Notes on the real API surface (verified against testserver):

* Create/Get/Update return lower_snake_case JSON (``{"id": ...}``).
* There is no dedicated ``/dryrun`` route. Dry-run is requested via
  ``POST /chains/{id}/execute`` with body ``{"session_id": "...",
  "dry_run": true}``.
* Execute always requires a JSON body containing ``session_id``.
"""

import pytest


CHAIN_PAYLOAD = {
    "name": "e2e-test-chain",
    "steps": [
        {
            "id": "s1",
            "technique_id": "T1082",
            "name": "System Information Discovery",
            "args": {},
            "depends_on": [],
            "on_fail": "abort",
        }
    ],
}


def _chain_id(payload: dict) -> str:
    return payload.get("id") or payload.get("ID")


@pytest.fixture(scope="module")
def chain_id(client):
    r = client.post("/api/v1/chains", json=CHAIN_PAYLOAD)
    assert r.status_code == 201, r.text
    cid = _chain_id(r.json())
    assert cid
    return cid


def test_list_chains_endpoint(client):
    r = client.get("/api/v1/chains")
    assert r.status_code == 200, r.text
    assert isinstance(r.json(), list)


def test_create_chain(chain_id):
    assert chain_id


def test_get_chain(client, chain_id):
    r = client.get(f"/api/v1/chains/{chain_id}")
    assert r.status_code == 200, r.text
    data = r.json()
    assert (data.get("name") or data.get("Name")) == "e2e-test-chain"


def test_update_chain(client, chain_id):
    updated = dict(CHAIN_PAYLOAD)
    updated["name"] = "e2e-updated"
    r = client.put(f"/api/v1/chains/{chain_id}", json=updated)
    assert r.status_code == 200, r.text
    # Confirm persistence.
    r2 = client.get(f"/api/v1/chains/{chain_id}")
    assert r2.status_code == 200
    assert (r2.json().get("name") or r2.json().get("Name")) == "e2e-updated"


def test_dryrun_chain(client, chain_id):
    """Dry-run is exposed via POST /execute with dry_run=true."""
    r = client.post(
        f"/api/v1/chains/{chain_id}/execute",
        json={"session_id": "dry-session", "dry_run": True},
    )
    assert r.status_code == 200, r.text
    data = r.json()
    assert data.get("dry_run") is True
    assert "order" in data or "Order" in data
    order = data.get("order") or data.get("Order")
    assert order == ["s1"]


def test_execute_missing_session(client, chain_id):
    """No session_id in body must yield 400."""
    r = client.post(f"/api/v1/chains/{chain_id}/execute", json={})
    assert r.status_code == 400, r.text


def test_execute_missing_body(client, chain_id):
    """A POST with no body should also yield 400 (invalid JSON)."""
    r = client.post(f"/api/v1/chains/{chain_id}/execute")
    assert r.status_code == 400, r.text


def test_execute_chain(client, chain_id):
    r = client.post(
        f"/api/v1/chains/{chain_id}/execute",
        json={"session_id": "test-session-123"},
    )
    assert r.status_code in (200, 202), r.text
    data = r.json()
    assert data.get("execution_id") or data.get("id") or data.get("ID")


def test_delete_chain(client, chain_id):
    r = client.delete(f"/api/v1/chains/{chain_id}")
    assert r.status_code in (200, 204), r.text


def test_get_deleted_chain(client, chain_id):
    r = client.get(f"/api/v1/chains/{chain_id}")
    assert r.status_code == 404


def test_get_unknown_chain(client):
    r = client.get("/api/v1/chains/does-not-exist")
    assert r.status_code == 404


def test_execute_unknown_chain(client):
    r = client.post(
        "/api/v1/chains/does-not-exist/execute",
        json={"session_id": "x"},
    )
    assert r.status_code == 404
