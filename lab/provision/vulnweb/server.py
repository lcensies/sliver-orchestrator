#!/usr/bin/env python3
# Deliberately vulnerable web app for lab initial-access testing.
#
# GET /ping?host=<h> runs `ping -c1 <h>` through the shell with NO sanitisation —
# a classic OS command-injection sink. It exists solely so an initial-access module
# can exploit it to stage a Sliver beacon in the lab. Do not ship this anywhere real.
import os
import subprocess
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from urllib.parse import urlparse, parse_qs


class Handler(BaseHTTPRequestHandler):
    def do_GET(self):
        parsed = urlparse(self.path)
        if parsed.path == "/ping":
            host = parse_qs(parsed.query).get("host", [""])[0]
            # VULNERABLE: unsanitised interpolation into a shell command.
            proc = subprocess.run(
                "ping -c1 " + host,
                shell=True, capture_output=True, text=True,
            )
            body = (proc.stdout + proc.stderr).encode()
            self.send_response(200)
            self.end_headers()
            self.wfile.write(body)
        else:
            self.send_response(200)
            self.end_headers()
            self.wfile.write(b"vulnweb: try /ping?host=127.0.0.1\n")

    def log_message(self, format, *args):  # quiet
        pass


if __name__ == "__main__":
    port = int(os.environ.get("PORT", "8080"))
    ThreadingHTTPServer(("0.0.0.0", port), Handler).serve_forever()
