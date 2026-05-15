# TODO

## Next Version: Full Tunnel Mode

The goal is to tunnel all phone traffic (not just Google) through the relay, removing the need for MITM proxy and CA certificates entirely.

- **Cherry-pick tunnel backend** from `feat/raw-tcp-tunnel`: `relay/core/tunnel.go`, `relay/vps/main.go` (`/tunnel` endpoint), `relay/apps-script/Code.gs` (`doTunnel_`). Skip the Android/mobile changes from that branch — main already has better versions.
- **Wire non-Google CONNECT through TunnelClient** in the proxy handler. Currently returns 502 for non-Google domains. Route those through `TunnelClient` instead (Apps Script → VPS → TCP to target).
- **Replace Google CIDR routes with `addRoute("0.0.0.0", 0)`** in `RelayVpnService`. Google domains still take the direct fragmentation path; everything else goes through the tunnel. One line change once the proxy wiring is done.
- **Remove MITM proxy mode**: delete CA generation, `Start()` with CA, and all proxy-mode UI. Nobody needs it once full tunnel works.
- **Simplify UI to two modes**: Direct-only (no config, Google services only) and Full-tunnel (relay URL + key, all traffic).

Direct mode and tunnel mode coexist in the same proxy on port 8085 — same `IsDirectDomain()` check, different else branch.

## Future: macOS Desktop Builds

Ship unsigned macOS desktop binaries for Apple Silicon (`darwin/arm64`) and Intel (`darwin/amd64`). The desktop GUI is already browser-based and uses embedded HTML assets, so the first release can be CLI/GUI binaries before a polished `.app` bundle.

