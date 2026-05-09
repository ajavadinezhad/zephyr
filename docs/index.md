---
# https://vitepress.dev/reference/default-theme-home-page
layout: home

hero:
  name: "Zyrln"
  text: "Bypass DPI-Based Censorship"
  tagline: Route traffic through Google infrastructure to stay undetectable
  image:
    src: /public/logo.png
    alt: Zyrln
  actions:
    - theme: brand
      text: Get Started
      link: /getting-started
    - theme: alt
      text: GitHub
      link: https://github.com/ajavadinezhad/zyrln

features:
  - title: 🕵️ DPI-Undetectable
    details: All traffic exits from Google's IP ranges and is completely indistinguishable from normal Google traffic. No VPN fingerprint, unusual port, or dedicated server IP to block.
  - title: ⚡ Request Coalescing
    details: Concurrent browser requests are batched into a single Apps Script call. A page with 30 requests uses only 1-3 executions instead of 30, dramatically extending daily quota.
  - title: 💾 In-Proxy Cache
    details: Static assets (JS, CSS, fonts, images) are served from memory on repeat visits. Cached responses skip the relay entirely, making subsequent loads significantly faster.
  - title: 🔄 Multi-URL Failover
    details: Configure multiple Apps Script deployments across different Google accounts. The relay races configured URLs in parallel and uses the first successful response.
  - title: 🔒 Full HTTPS Support
    details: The proxy performs local TLS termination so blocked HTTPS sites work transparently. No plaintext data leaves your device.
  - title: 📱 Android VPN - No Root
    details: One tap routes all browser traffic through the relay at the system level. No per-app configuration, no ADB, no root required.
  - title: 🖥️ Cross-Platform Desktop
    details: Works on Windows, Linux, and macOS with a simple GUI or CLI mode. Perfect for developers and power users.
  - title: 🚀 Parallel Probes
    details: Built-in reachability testing tool checks which endpoints work from your network before setup. Know exactly what works and what doesn't.
---