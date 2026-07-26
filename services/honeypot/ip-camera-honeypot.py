#!/usr/bin/env python3
# honeypot.py — fake IP-camera web front-end for the lab scenario.
#
# Serves a camera-looking login page and camera-identifying headers so an
# attacker's nmap/scan "finds a webcam" on the external-facing interface.
# Defensive lab use only — runs on linux_pivot, isolated range.
#
# Run:  sudo python3 honeypot.py            # binds 0.0.0.0:8080
#       sudo python3 honeypot.py 192.168.56.10 8080

import sys
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

HOST = sys.argv[1] if len(sys.argv) > 1 else "0.0.0.0"
PORT = int(sys.argv[2]) if len(sys.argv) > 2 else 8080

# Camera-looking login page (generic Hikvision/Dahua-style).
PAGE = b"""<!DOCTYPE html>
<html><head><meta charset="utf-8">
<title>Web Service - IP Camera Login</title>
<meta name="viewport" content="width=device-width, initial-scale=1">
<style>
 body{background:#1c2733;color:#c7d2dd;font-family:Arial,sans-serif;margin:0}
 .wrap{max-width:360px;margin:8% auto;background:#243444;border:1px solid #34495e;
       border-radius:6px;padding:28px 30px;box-shadow:0 6px 24px #0008}
 h1{font-size:17px;margin:0 0 4px;color:#e8eef4}
 .sub{font-size:12px;color:#7d94a8;margin-bottom:22px}
 label{display:block;font-size:12px;margin:12px 0 5px;color:#9db0c2}
 input{width:100%;box-sizing:border-box;padding:9px 10px;border:1px solid #3d5468;
       border-radius:4px;background:#1c2733;color:#e8eef4}
 button{width:100%;margin-top:20px;padding:10px;border:0;border-radius:4px;
        background:#2c7be5;color:#fff;font-weight:bold;cursor:pointer}
 .foot{font-size:11px;color:#5f7286;text-align:center;margin-top:18px}
</style></head>
<body><div class="wrap">
 <h1>IP CAMERA</h1>
 <div class="sub">Web Service v3.2.1 &mdash; DS-2CD Series</div>
 <label>User Name</label><input type="text" value="admin">
 <label>Password</label><input type="password">
 <button>Login</button>
 <div class="foot">&copy; Network Video Surveillance</div>
</div></body></html>
"""

class Cam(BaseHTTPRequestHandler):
    server_version = "Webs/2.5.0"          # camera-style HTTP server banner
    sys_version = ""

    def _hdr(self, code=200, ctype="text/html"):
        self.send_response(code)
        self.send_header("Content-Type", ctype)
        self.send_header("Server", "Webs/2.5.0")
        # camera-identifying headers nmap/whatweb pick up
        self.send_header("WWW-Authenticate", 'Basic realm="IP Camera"')
        self.send_header("X-Camera-Model", "DS-2CD2042WD")
        self.end_headers()

    def do_GET(self):
        if self.path in ("/", "/index.html", "/doc/page/login.asp"):
            self._hdr(200)
            self.wfile.write(PAGE)
        elif self.path.startswith("/onvif") or self.path.startswith("/Streaming"):
            # ONVIF / RTSP-ish endpoints a camera scanner probes
            self._hdr(401)
            self.wfile.write(b"Unauthorized")
        else:
            self._hdr(404)
            self.wfile.write(b"Not Found")

    def do_POST(self):
        # "login attempt" — always fail, but log it (attacker interaction)
        length = int(self.headers.get("Content-Length", 0))
        self.rfile.read(length)
        self._hdr(401)
        self.wfile.write(b"Login failed")

    def log_message(self, fmt, *args):
        # log attacker hits to stdout (visible for the demo)
        sys.stdout.write("[honeypot] %s - %s\n" % (self.address_string(), fmt % args))
        sys.stdout.flush()

if __name__ == "__main__":
    print(f"[honeypot] fake IP-camera on http://{HOST}:{PORT}")
    ThreadingHTTPServer((HOST, PORT), Cam).serve_forever()
