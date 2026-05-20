"""Shared fixtures for the sliver-orchestrator E2E test suite.

Spawns the `testserver` binary against a per-session sqlite DB and the
fixture atomics directory under /tmp/test-atomics, then exposes an httpx
Client bound to its base URL.
"""

import os
import socket
import subprocess
import time

import httpx
import pytest


TESTSERVER_BIN = os.environ.get("TESTSERVER_BIN", "/tmp/testserver")
ATOMICS_DIR = os.environ.get("TEST_ATOMICS_DIR", "/tmp/test-atomics")


def _free_port() -> int:
    with socket.socket() as s:
        s.bind(("", 0))
        return s.getsockname()[1]


@pytest.fixture(scope="session")
def server(tmp_path_factory):
    port = _free_port()
    db = str(tmp_path_factory.mktemp("db") / "test.db")
    proc = subprocess.Popen(
        [
            TESTSERVER_BIN,
            f"-listen=:{port}",
            f"-db={db}",
            f"-atomics={ATOMICS_DIR}",
        ],
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
    )
    base = f"http://localhost:{port}"
    for _ in range(50):
        if proc.poll() is not None:
            out, err = proc.communicate(timeout=2)
            raise RuntimeError(
                f"testserver exited early rc={proc.returncode}\n"
                f"stdout={out.decode(errors='replace')}\n"
                f"stderr={err.decode(errors='replace')}"
            )
        try:
            r = httpx.get(f"{base}/api/v1/health", timeout=1)
            if r.status_code == 200:
                break
        except Exception:
            pass
        time.sleep(0.2)
    else:
        proc.terminate()
        raise RuntimeError("testserver did not become healthy in time")

    yield base

    proc.terminate()
    try:
        proc.wait(timeout=5)
    except subprocess.TimeoutExpired:
        proc.kill()
        proc.wait()


@pytest.fixture(scope="session")
def client(server):
    with httpx.Client(base_url=server, timeout=10) as c:
        yield c
