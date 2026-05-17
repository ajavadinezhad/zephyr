package core

import (
	"context"
	"errors"
	"io"
	"net"
	"strings"
	"sync/atomic"
	"time"
)

var errAllProfilesFailed = errors.New("all direct profiles failed")

// directEnabled controls whether Google domains skip the relay and use
// fragmented direct dialing instead. Atomic so the GUI can toggle it safely.
var directEnabled atomic.Bool

func init() { directEnabled.Store(true) }

func SetDirectEnabled(v bool) { directEnabled.Store(v) }
func GetDirectEnabled() bool  { return directEnabled.Load() }

// DirectFronts are Google-owned domains reachable in Iran (not IP-blocked).
// We TCP-connect to one of these so the fragmented ClientHello SNI points at
// an allowed domain, while the browser's inner TLS targets the actual service.
// Add entries here when a new Google domain becomes reachable.
var DirectFronts = []string{
	"www.google.com",
	"script.google.com",
}

// googleDomains are suffixes eligible for direct fragmented dialing instead of
// the relay. These are SNI-filtered in Iran but not IP-blocked — Google's
// infrastructure serves all of them from the same reachable IP ranges.
var googleDomains = []string{
	".google.com",
	".googleapis.com",
	".googleusercontent.com",
	".gstatic.com",
	".ggpht.com",
	".googletagmanager.com",
	".googletagservices.com",
	".googlesyndication.com",
	".gmail.com",
	".googlemail.com",
	".google-analytics.com",
	".googleadservices.com",
	".doubleclick.net",
	".android.com",
	".appspot.com",
	".withgoogle.com",
}

// sanctionedDomains are Google services geo-blocked for Iranian IPs.
// These are excluded from direct dialing and always routed through the relay.
var sanctionedDomains = []string{
	"gemini.google.com",
	"bard.google.com",
	"ai.google",
	"aistudio.google.com",
	"labs.google",
}

// IsGoogleDomain reports whether host is a Google domain eligible for direct
// fragmented dialing. Returns false for sanctioned domains so they always go
// through the relay.
func IsGoogleDomain(host string) bool {
	h := strings.ToLower(host)
	if idx := strings.LastIndex(h, ":"); idx != -1 {
		h = h[:idx]
	}
	for _, d := range sanctionedDomains {
		if h == d || strings.HasSuffix(h, "."+d) {
			return false
		}
	}
	for _, suffix := range googleDomains {
		if h == suffix[1:] || strings.HasSuffix(h, suffix) {
			return true
		}
	}
	return false
}

// IsDirectDomain reports whether host can skip the relay and be reached via
// fragmented direct dialing. Returns false when direct mode is disabled.
func IsDirectDomain(host string) bool {
	if !GetDirectEnabled() {
		return false
	}
	return IsGoogleDomain(host)
}

// handleDirectConnect is called by the proxy when the CONNECT target is a
// Google domain. We open a fragmented direct connection and pipe bytes through
// — the browser's TLS stack runs end-to-end, we never see the plaintext.
func handleDirectConnect(clientConn net.Conn, targetHost string) {
	serverConn, profileID, elapsed, err := dialFragment(targetHost, 15*time.Second)
	if err != nil {
		// One retry — covers transient network bursts where all race goroutines
		// hit congestion simultaneously.
		serverConn, profileID, elapsed, err = dialFragment(targetHost, 15*time.Second)
	}
	if err != nil {
		logf("error", "direct %s: all profiles failed", targetHost)
		_, _ = clientConn.Write([]byte("HTTP/1.1 502 Bad Gateway\r\n\r\n"))
		return
	}
	defer serverConn.Close()

	_, _ = clientConn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))
	logf("info", "direct %s profile=%s %s", targetHost, profileID, elapsed.Round(time.Millisecond))
	pipe(clientConn, serverConn)
}

// pipe bidirectionally copies between two connections until both directions close.
func pipe(a, b net.Conn) {
	done := make(chan struct{}, 2)
	cp := func(dst, src net.Conn) {
		_, _ = io.Copy(dst, src)
		_ = dst.SetDeadline(time.Now())
		_ = src.SetDeadline(time.Now())
		done <- struct{}{}
	}
	go cp(b, a)
	go cp(a, b)
	<-done
	<-done
}

// DialFragment is the external entry point for direct fragmented dialing
// (used by the SOCKS5 path and connectivity checks).
func DialFragment(addr string) (net.Conn, bool) {
	conn, profileID, elapsed, err := dialFragment(addr, 15*time.Second)
	if err != nil {
		logf("error", "direct %s: %v", addr, err)
		return nil, false
	}
	logf("info", "direct %s profile=%s %s", addr, profileID, elapsed.Round(time.Millisecond))
	return conn, true
}

// dialFragment connects to addr via a reachable Google front using TLS
// fragmentation. It tries the last successful (front, profile) pair first;
// if that fails it races all combinations and takes the first winner.
func dialFragment(addr string, timeout time.Duration) (net.Conn, string, time.Duration, error) {
	profiles := rankedProfiles()

	// Fast path: try the remembered (front, profile) pair with a short deadline.
	// Both front and profile come from the same remembered snapshot for consistency.
	c := remembered.Load()
	front := DirectFronts[0]
	if c != nil && c.front != "" {
		front = c.front
	}
	fastCtx, fastCancel := context.WithTimeout(context.Background(), 6*time.Second)
	conn, elapsed, err := dialDirect(fastCtx, addr, front, profiles[0].Config, timeout)
	fastCancel()
	if err == nil {
		// Refresh the cache file so it stays current even if the race never runs.
		rememberCandidate(front, profiles[0].ID)
		return conn, profiles[0].ID, elapsed, nil
	}

	// Fast path failed — race all (front × profile) combinations except the one
	// we just tried, and take the first winner.
	type result struct {
		conn      net.Conn
		front     string
		profileID string
		elapsed   time.Duration
	}

	// Skip the (front, profiles[0]) pair — just failed on fast path.
	skippedFront, skippedProfileID := front, profiles[0].ID

	candidates := len(DirectFronts)*len(profiles) - 1
	ch := make(chan result, candidates)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	for _, f := range DirectFronts {
		for _, p := range profiles {
			if f == skippedFront && p.ID == skippedProfileID {
				continue
			}
			f, p := f, p
			go func() {
				c, el, err := dialDirect(ctx, addr, f, p.Config, timeout)
				if err != nil {
					logf("debug", "direct %s front=%s profile=%s: %v", addr, f, p.ID, err)
					ch <- result{}
					return
				}
				ch <- result{conn: c, front: f, profileID: p.ID, elapsed: el}
			}()
		}
	}

	var winner result
	for i := 0; i < candidates; i++ {
		r := <-ch
		if r.conn == nil {
			continue
		}
		if winner.conn == nil {
			winner = r
			cancel()
		} else {
			r.conn.Close()
		}
	}

	if winner.conn == nil {
		return nil, "", 0, errAllProfilesFailed
	}
	rememberCandidate(winner.front, winner.profileID)
	return winner.conn, winner.profileID, winner.elapsed, nil
}
