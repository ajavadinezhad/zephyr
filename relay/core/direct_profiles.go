package core

import (
	"context"
	"crypto/tls"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"
)

// DirectProfile is a named fragmentation configuration for direct Google-domain dialing.
type DirectProfile struct {
	ID     string
	Config FragmentConfig
}

// directProfiles is the ordered list of profiles tried by dialDirectWithFallback.
// Order matters: cheaper/faster profiles come first.
var directProfiles = [...]DirectProfile{
	{ID: "p00", Config: FragmentConfig{NumChunks: 1, Delay: 0}},
	{ID: "p01", Config: FragmentConfig{NumChunks: 8, Delay: 5 * time.Millisecond, Splits: func(b []byte) []int {
		pts := sniSplits(b)
		if len(pts) == 0 {
			return equalSplits(len(b), 8)
		}
		return append([]int{1, 5}, pts...)
	}}},
	{ID: "p02", Config: FragmentConfig{NumChunks: 16, Delay: 1 * time.Millisecond, Splits: func(b []byte) []int { return equalSplits(len(b), 16) }}},
	{ID: "p03", Config: FragmentConfig{NumChunks: 32, Delay: 3 * time.Millisecond, Splits: func(b []byte) []int { return equalSplits(len(b), 32) }}},
	{ID: "p04", Config: FragmentConfig{NumChunks: 64, Delay: 5 * time.Millisecond, Splits: func(b []byte) []int { return equalSplits(len(b), 64) }}},
	{ID: "p05", Config: FragmentConfig{NumChunks: 87, Delay: 5 * time.Millisecond, Splits: func(b []byte) []int { return equalSplits(len(b), 87) }}},
	{ID: "p06", Config: FragmentConfig{NumChunks: 120, Delay: 10 * time.Millisecond, Splits: func(b []byte) []int { return equalSplits(len(b), 120) }}},
	{ID: "p07", Config: FragmentConfig{NumChunks: 8, Delay: 25 * time.Millisecond, Splits: sniSplits}},
}

// rememberedCandidate stores the last successful (front, profile index) pair.
type rememberedCandidate struct {
	front      string
	profileIdx int32
}

var remembered atomic.Pointer[rememberedCandidate]

var cacheDir atomic.Pointer[string]

// SetCacheDir tells the core where to persist the remembered (front, profile)
// across restarts. Call this once at startup with the app's data directory.
func SetCacheDir(dir string) {
	cacheDir.Store(&dir)
	loadRemembered()
}

func candidateCacheFile() string {
	d := cacheDir.Load()
	if d == nil || *d == "" {
		return ""
	}
	return filepath.Join(*d, "direct_candidate.txt")
}

// rememberCandidate saves the winning (front, profileID) in memory and to disk.
func rememberCandidate(front, profileID string) {
	for i, p := range directProfiles {
		if p.ID == profileID {
			storeRemembered(front, int32(i))
			if f := candidateCacheFile(); f != "" {
				_ = os.WriteFile(f, []byte(front+"\n"+profileID), 0o600)
			}
			return
		}
	}
}

func storeRemembered(front string, idx int32) {
	remembered.Store(&rememberedCandidate{front: front, profileIdx: idx})
}

// loadRemembered reads the persisted candidate from disk into memory only —
// does not write back to disk.
func loadRemembered() {
	f := candidateCacheFile()
	if f == "" {
		return
	}
	data, err := os.ReadFile(f)
	if err != nil {
		return
	}
	parts := strings.SplitN(strings.TrimSpace(string(data)), "\n", 2)
	if len(parts) != 2 {
		return
	}
	front, profileID := parts[0], parts[1]
	for i, p := range directProfiles {
		if p.ID == profileID {
			storeRemembered(front, int32(i))
			return
		}
	}
}

// DirectProfiles returns a copy of the profile list for external inspection (e.g. probe reporting).
func DirectProfiles() []DirectProfile {
	out := make([]DirectProfile, len(directProfiles))
	copy(out, directProfiles[:])
	return out
}

// rankedProfiles returns profiles starting from the last successful one, wrapping around.
func rankedProfiles() []DirectProfile {
	c := remembered.Load()
	idx := -1
	if c != nil {
		idx = int(c.profileIdx)
	}
	if idx < 0 || idx >= len(directProfiles) {
		return directProfiles[:]
	}
	out := make([]DirectProfile, 0, len(directProfiles))
	out = append(out, directProfiles[idx])
	out = append(out, directProfiles[:idx]...)
	out = append(out, directProfiles[idx+1:]...)
	return out
}

// dialDirect opens a fragmented TCP connection to front (or addr's host if empty).
// No TLS handshake is performed — the caller decides the TLS SNI.
// Probe path: front = open Google domain, SNI = blocked target → tests DPI bypass.
// Live proxy path: front = "" (connects directly to target), SNI = target (browser's TLS).
func dialDirect(ctx context.Context, addr, front string, cfg FragmentConfig, timeout time.Duration) (net.Conn, time.Duration, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, 0, err
	}
	if front == "" {
		front = host
	}
	dialAddr := net.JoinHostPort(front, port)
	if cfg.NumChunks == 0 {
		cfg = DefaultFragmentConfig
	}

	var d net.Dialer
	if timeout > 0 {
		d.Timeout = timeout
	}
	start := time.Now()
	raw, err := d.DialContext(ctx, "tcp", dialAddr)
	if err != nil {
		return nil, 0, err
	}
	setTCPNoDelay(raw)
	return &fragmentConn{Conn: raw, cfg: cfg, firstWrite: true}, time.Since(start), nil
}

// --- probe ---

// DirectProbeResult holds the outcome of probing one (target, front, profile) triple.
type DirectProbeResult struct {
	Target    string
	Front     string
	ProfileID string
	OK        bool
	Stable    int
	Repeat    int
	Avg       time.Duration
	Reason    string
}

// DirectProbeReport is returned by ProbeDirectProfiles.
type DirectProbeReport struct {
	Targets []string
	Fronts  []string
	Results []DirectProbeResult
	Best    map[string]DirectProbeResult
}

// ProbeDirectProfiles probes every (target × front × profile) combination and
// returns a report with per-target best results. This mirrors the adaptive mode
// of tools/utls-frag-probe but uses stdlib TLS instead of utls.
func ProbeDirectProfiles(ctx context.Context, targets, fronts []string, repeat int, timeout time.Duration) DirectProbeReport {
	targets = normalizeTargets(targets)
	fronts = normalizeFronts(fronts)
	if repeat < 1 {
		repeat = 1
	}
	profiles := DirectProfiles()
	results := make([]DirectProbeResult, 0, len(targets)*len(fronts)*len(profiles))
	best := make(map[string]DirectProbeResult, len(targets))

	for _, target := range targets {
		for _, front := range fronts {
			for _, profile := range profiles {
				res := probeOne(ctx, target, front, profile, repeat, timeout)
				results = append(results, res)
				if res.OK && isBetter(res, best[target]) {
					best[target] = res
				}
			}
		}
	}
	return DirectProbeReport{Targets: targets, Fronts: fronts, Results: results, Best: best}
}

func probeOne(ctx context.Context, target, front string, profile DirectProfile, repeat int, timeout time.Duration) DirectProbeResult {
	host, _, err := net.SplitHostPort(target)
	if err != nil {
		return DirectProbeResult{Target: target, Front: front, ProfileID: profile.ID, Repeat: repeat, Reason: "badaddr"}
	}

	var total time.Duration
	stable := 0
	reason := ""

	for i := 0; i < repeat; i++ {
		attemptCtx, cancel := context.WithTimeout(ctx, timeout)
		// TCP connects to front (open IP), TLS SNI = target (blocked domain).
		// Tests whether fragmentation defeats SNI inspection for the real hostname.
		conn, elapsed, err := dialDirect(attemptCtx, target, front, profile.Config, timeout)
		cancel()
		if err != nil {
			reason = classifyErr(err)
			continue
		}
		// SNI = target: if DPI inspects the fragmented hello and blocks on the
		// target hostname, the handshake fails — exactly what we want to detect.
		tlsConn := tls.Client(conn, &tls.Config{ServerName: host, MinVersion: tls.VersionTLS12})
		_ = tlsConn.SetDeadline(time.Now().Add(timeout))
		herr := tlsConn.Handshake()
		_ = tlsConn.Close()
		if herr != nil {
			reason = classifyErr(herr)
			continue
		}
		stable++
		total += elapsed
	}

	res := DirectProbeResult{
		Target:    target,
		Front:     front,
		ProfileID: profile.ID,
		OK:        stable == repeat,
		Stable:    stable,
		Repeat:    repeat,
		Reason:    reason,
	}
	if stable > 0 {
		res.Avg = total / time.Duration(stable)
	}
	return res
}

func isBetter(a, b DirectProbeResult) bool {
	if b.Target == "" {
		return true
	}
	if a.Stable != b.Stable {
		return a.Stable > b.Stable
	}
	if a.Avg != b.Avg {
		if a.Avg == 0 {
			return false
		}
		if b.Avg == 0 {
			return true
		}
		return a.Avg < b.Avg
	}
	return a.ProfileID < b.ProfileID
}

func classifyErr(err error) string {
	if err == nil {
		return ""
	}
	s := strings.ToLower(err.Error())
	switch {
	case strings.Contains(s, "timeout"), strings.Contains(s, "deadline"):
		return "timeout"
	case strings.Contains(s, "reset"), strings.Contains(s, "forcibly closed"):
		return "reset"
	case strings.Contains(s, "refused"):
		return "refused"
	case strings.Contains(s, "certificate"):
		return "tls"
	default:
		return "error"
	}
}

func normalizeTargets(targets []string) []string {
	out := make([]string, 0, len(targets))
	for _, t := range targets {
		t = strings.TrimSpace(t)
		if t == "" {
			continue
		}
		if !strings.Contains(t, ":") {
			t = net.JoinHostPort(t, "443")
		}
		out = append(out, t)
	}
	return out
}

func normalizeFronts(fronts []string) []string {
	out := make([]string, 0, len(fronts))
	for _, f := range fronts {
		f = strings.TrimSpace(f)
		if f != "" {
			out = append(out, f)
		}
	}
	return out
}
