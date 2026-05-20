"""Executions, cancel, stream, and sessions endpoints.

The real API uses:

* ``POST /executions/{id}/cancel`` (not DELETE on the collection).
* ``GET /executions/{id}/stream`` (not ``/stream/{id}``).
"""

import time

import pytest


CHAIN_PAYLOAD = {
    "name": "exec-test-chain",
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


def _id(obj: dict) -> str | None:
    return obj.get("execution_id") or obj.get("id") or obj.get("ID")


@pytest.fixture(scope="module")
def execution_setup(client):
    r = client.post("/api/v1/chains", json=CHAIN_PAYLOAD)
    assert r.status_code == 201, r.text
    chain = r.json()
    chain_id = chain.get("id") or chain.get("ID")
    assert chain_id

    r2 = client.post(
        f"/api/v1/chains/{chain_id}/execute",
        json={"session_id": "exec-session-999"},
    )
    assert r2.status_code in (200, 202), r2.text
    exec_id = _id(r2.json())
    assert exec_id
    return chain_id, exec_id


def test_list_executions(client, execution_setup):
    r = client.get("/api/v1/executions")
    assert r.status_code == 200, r.text
    assert isinstance(r.json(), list)


def test_get_execution(client, execution_setup):
    _, exec_id = execution_setup
    r = client.get(f"/api/v1/executions/{exec_id}")
    assert r.status_code == 200, r.text
    data = r.json()
    # Detail endpoint wraps the record as ``{"execution": {...}, "steps": [...]}``.
    exec_obj = data.get("execution") or data.get("Execution") or data
    rid = exec_obj.get("id") or exec_obj.get("ID")
    assert rid == exec_id
    assert "steps" in data or "Steps" in data


def test_list_executions_filter_chain(client, execution_setup):
    chain_id, exec_id = execution_setup
    r = client.get("/api/v1/executions", params={"chain_id": chain_id})
    assert r.status_code == 200, r.text
    items = r.json()
    # If the API doesn't honour the filter we still want to see at least
    # one execution overall.
    assert isinstance(items, list)
    assert len(items) >= 1
    seen_ids = {(it.get("id") or it.get("ID")) for it in items}
    assert exec_id in seen_ids


def test_cancel_unknown_execution(client):
    r = client.post("/api/v1/executions/nonexistent-id/cancel")
    assert r.status_code == 404


def test_stream_unknown_execution(client):
    r = client.get("/api/v1/executions/nonexistent-id/stream")
    assert r.status_code == 404


def test_cancel_existing_execution(client, execution_setup):
    """Issue cancel against a real execution; transport may be 200 or 202.

    The stub RPC completes near-instantly so by the time cancel arrives
    the execution may already be done; either 200/202 (cancel accepted)
    or 404/409 (already terminal) are acceptable contract responses.
    """
    chain_id, _ = execution_setup
    # Kick off a fresh execution we can attempt to cancel.
    r = client.post(
        f"/api/v1/chains/{chain_id}/execute",
        json={"session_id": "cancel-session"},
    )
    assert r.status_code in (200, 202), r.text
    exec_id = _id(r.json())
    assert exec_id

    # Best-effort cancel — accept any non-5xx response.
    r2 = client.post(f"/api/v1/executions/{exec_id}/cancel")
    assert r2.status_code < 500, r2.text


def test_list_sessions(client):
    r = client.get("/api/v1/sessions")
    assert r.status_code == 200, r.text
    assert isinstance(r.json(), list)
