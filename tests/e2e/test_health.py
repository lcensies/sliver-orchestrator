"""Smoke tests for /health and CORS handling."""


def test_health(client):
    r = client.get("/api/v1/health")
    assert r.status_code == 200, r.text
    data = r.json()
    assert data.get("status") == "ok"


def test_cors_preflight(client):
    r = client.options(
        "/api/v1/health",
        headers={
            "Origin": "http://localhost:3000",
            "Access-Control-Request-Method": "GET",
        },
    )
    assert r.status_code in (200, 204), r.text


def test_cors_headers_on_response(client):
    r = client.get(
        "/api/v1/health",
        headers={"Origin": "http://localhost:3000"},
    )
    assert r.status_code == 200
    assert "access-control-allow-origin" in {k.lower() for k in r.headers.keys()}
