# Decoy / vulnerable services

Deliberately-vulnerable and decoy services deployed onto lab targets. **Educational,
isolated-range use only — never expose to untrusted networks.**

- `resolver-service.go` — "ResolvTech IP Resolver": a Go web service on `:8080` with a
  command-injection sink at `GET /ping?host=`. Run: `GOCACHE=/tmp/go-cache go run resolver-service.go`
- `ip-camera-honeypot.py` — a fake IP-camera login page + camera-identifying headers on
  `:8080`, so a scan "finds a webcam". Run: `sudo python3 ip-camera-honeypot.py`
