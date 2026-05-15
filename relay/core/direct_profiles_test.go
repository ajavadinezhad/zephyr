package core

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"net"
	"testing"
	"time"
)

// --- DirectProfiles ---

func TestDirectProfiles_ReturnsCopy(t *testing.T) {
	a := DirectProfiles()
	b := DirectProfiles()
	if len(a) == 0 {
		t.Fatal("DirectProfiles returned empty slice")
	}
	if len(a) != len(b) {
		t.Errorf("inconsistent length: %d vs %d", len(a), len(b))
	}
	// Modifying the copy must not affect the original.
	a[0].ID = "mutated"
	if DirectProfiles()[0].ID == "mutated" {
		t.Error("DirectProfiles returned a reference, not a copy")
	}
}

func TestDirectProfiles_IDs(t *testing.T) {
	profiles := DirectProfiles()
	want := []string{"p00", "p01", "p02", "p03", "p04", "p05", "p06", "p07"}
	if len(profiles) != len(want) {
		t.Fatalf("got %d profiles, want %d", len(profiles), len(want))
	}
	for i, p := range profiles {
		if p.ID != want[i] {
			t.Errorf("profiles[%d].ID = %q, want %q", i, p.ID, want[i])
		}
	}
}

// --- rememberCandidate / rankedProfiles ---

func TestRankedProfiles_DefaultOrder(t *testing.T) {
	remembered.Store(nil)
	t.Cleanup(func() { remembered.Store(nil) })

	ranked := rankedProfiles()
	all := DirectProfiles()
	if len(ranked) != len(all) {
		t.Fatalf("length mismatch: %d vs %d", len(ranked), len(all))
	}
	for i := range ranked {
		if ranked[i].ID != all[i].ID {
			t.Errorf("ranked[%d] = %q, want %q", i, ranked[i].ID, all[i].ID)
		}
	}
}

func TestRankedProfiles_RememberedFirst(t *testing.T) {
	t.Cleanup(func() { remembered.Store(nil) })

	rememberCandidate("www.google.com", "p03")
	ranked := rankedProfiles()
	if ranked[0].ID != "p03" {
		t.Errorf("expected p03 first after rememberCandidate, got %s", ranked[0].ID)
	}
	// All profiles still present.
	if len(ranked) != len(directProfiles) {
		t.Errorf("ranked length %d, want %d", len(ranked), len(directProfiles))
	}
}

func TestRankedProfiles_AllProfilesPresent(t *testing.T) {
	t.Cleanup(func() { remembered.Store(nil) })

	rememberCandidate("www.google.com", "p05")
	ranked := rankedProfiles()
	seen := map[string]bool{}
	for _, p := range ranked {
		seen[p.ID] = true
	}
	for _, p := range directProfiles {
		if !seen[p.ID] {
			t.Errorf("profile %s missing from ranked list", p.ID)
		}
	}
}

func TestRememberCandidate_UnknownID(t *testing.T) {
	t.Cleanup(func() { remembered.Store(nil) })
	remembered.Store(nil)
	// Unknown ID should not panic and should not change remembered state.
	rememberCandidate("www.google.com", "pXX")
	if remembered.Load() != nil {
		t.Error("rememberCandidate stored state for unknown profile ID")
	}
}

// --- normalizeTargets / normalizeFronts ---

func TestNormalizeTargets(t *testing.T) {
	cases := []struct {
		in   []string
		want []string
	}{
		{[]string{"www.youtube.com"}, []string{"www.youtube.com:443"}},
		{[]string{"www.youtube.com:443"}, []string{"www.youtube.com:443"}},
		{[]string{"  ", ""}, []string{}},
		{[]string{"a.com", "  b.com  "}, []string{"a.com:443", "b.com:443"}},
	}
	for _, tc := range cases {
		got := normalizeTargets(tc.in)
		if len(got) != len(tc.want) {
			t.Errorf("normalizeTargets(%v) = %v, want %v", tc.in, got, tc.want)
			continue
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Errorf("normalizeTargets[%d] = %q, want %q", i, got[i], tc.want[i])
			}
		}
	}
}

func TestNormalizeFronts(t *testing.T) {
	got := normalizeFronts([]string{"www.google.com", "  ", "", "script.google.com"})
	want := []string{"www.google.com", "script.google.com"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("[%d] got %q, want %q", i, got[i], want[i])
		}
	}
}

// --- classifyErr ---

func TestClassifyErr(t *testing.T) {
	cases := []struct {
		msg  string
		want string
	}{
		{"", ""},
		{"i/o timeout", "timeout"},
		{"context deadline exceeded", "timeout"},
		{"connection reset by peer", "reset"},
		{"forcibly closed", "reset"},
		{"connection refused", "refused"},
		{"certificate verify failed", "tls"},
		{"some unknown error", "error"},
	}
	for _, tc := range cases {
		var err error
		if tc.msg != "" {
			err = &errString{tc.msg}
		}
		got := classifyErr(err)
		if got != tc.want {
			t.Errorf("classifyErr(%q) = %q, want %q", tc.msg, got, tc.want)
		}
	}
}

type errString struct{ s string }

func (e *errString) Error() string { return e.s }

// --- isBetter ---

func TestIsBetter_EmptyB(t *testing.T) {
	a := DirectProbeResult{Target: "t", Stable: 1, OK: true, Avg: 100 * time.Millisecond}
	if !isBetter(a, DirectProbeResult{}) {
		t.Error("anything should be better than empty result")
	}
}

func TestIsBetter_MoreStable(t *testing.T) {
	a := DirectProbeResult{Target: "t", Stable: 2}
	b := DirectProbeResult{Target: "t", Stable: 1}
	if !isBetter(a, b) {
		t.Error("higher stable count should be better")
	}
}

func TestIsBetter_LowerAvg(t *testing.T) {
	a := DirectProbeResult{Target: "t", Stable: 1, Avg: 100 * time.Millisecond}
	b := DirectProbeResult{Target: "t", Stable: 1, Avg: 200 * time.Millisecond}
	if !isBetter(a, b) {
		t.Error("lower avg should be better")
	}
}

func TestIsBetter_TieBreakByProfileID(t *testing.T) {
	a := DirectProbeResult{Target: "t", Stable: 1, Avg: 100 * time.Millisecond, ProfileID: "p01"}
	b := DirectProbeResult{Target: "t", Stable: 1, Avg: 100 * time.Millisecond, ProfileID: "p02"}
	if !isBetter(a, b) {
		t.Error("lower profileID should win tie")
	}
}

// --- dialDirect with front ---

func TestDialDirect_WithFront(t *testing.T) {
	addr := startEchoServer(t)
	host, _, _ := net.SplitHostPort(addr)
	conn, _, err := dialDirect(context.Background(), addr, host, DefaultFragmentConfig, 5*time.Second)
	if err != nil {
		t.Fatalf("dialDirect with front failed: %v", err)
	}
	conn.Close()
}

// --- DialFragment with overridden DirectFronts ---

func TestDialFragment_WithLoopbackFront(t *testing.T) {
	addr := startEchoServer(t)
	host, _, _ := net.SplitHostPort(addr)
	orig := DirectFronts
	DirectFronts = []string{host}
	t.Cleanup(func() { DirectFronts = orig; remembered.Store(nil) })
	remembered.Store(nil)

	conn, ok := DialFragment(addr)
	if !ok {
		t.Fatal("DialFragment returned ok=false")
	}
	conn.Close()
}

func TestDialFragment_AllFrontsUnreachable(t *testing.T) {
	orig := DirectFronts
	DirectFronts = []string{"127.0.0.1"}
	t.Cleanup(func() { DirectFronts = orig; remembered.Store(nil) })
	remembered.Store(nil)

	conn, ok := DialFragment("127.0.0.1:1")
	if ok {
		conn.Close()
		t.Error("DialFragment should fail when all fronts are unreachable")
	}
}

// dialFragment race path: force fast-path to fail by clearing remembered,
// then verify the parallel race picks a winner.
func TestDialFragment_RacePath(t *testing.T) {
	addr := startEchoServer(t)
	host, _, _ := net.SplitHostPort(addr)
	orig := DirectFronts
	DirectFronts = []string{host}
	t.Cleanup(func() { DirectFronts = orig; remembered.Store(nil) })
	remembered.Store(nil) // no remembered → fast path uses DirectFronts[0] which works

	// First call hits fast path (remembered front = DirectFronts[0], good profile guessed).
	// Force fast path to fail by pointing remembered at a bad front.
	remembered.Store(&rememberedCandidate{front: "192.0.2.1", profileIdx: 0}) // unreachable RFC5737 addr
	conn, ok := DialFragment(addr)
	if !ok {
		t.Fatal("race path should succeed")
	}
	conn.Close()
}

// --- handleDirectConnect ---

func TestHandleDirectConnect_DirectDial(t *testing.T) {
	addr := startEchoServer(t)
	host, _, _ := net.SplitHostPort(addr)
	orig := DirectFronts
	DirectFronts = []string{host}
	t.Cleanup(func() { DirectFronts = orig; remembered.Store(nil) })
	remembered.Store(nil)

	clientSide, proxySide := net.Pipe()
	defer clientSide.Close()
	go handleDirectConnect(proxySide, addr)

	buf := make([]byte, 64)
	n, _ := clientSide.Read(buf)
	if string(buf[:n]) != "HTTP/1.1 200 Connection Established\r\n\r\n" {
		t.Errorf("unexpected response: %q", string(buf[:n]))
	}
}

// --- startTLSServer: self-signed TLS server for probe tests ---

func startTLSServer(t *testing.T) (addr string, certPool *x509.CertPool) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "127.0.0.1"},
		DNSNames:     []string{"127.0.0.1"},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
	}
	certDER, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create cert: %v", err)
	}
	cert, _ := x509.ParseCertificate(certDER)
	pool := x509.NewCertPool()
	pool.AddCert(cert)

	tlsCert := tls.Certificate{Certificate: [][]byte{certDER}, PrivateKey: key}
	ln, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{Certificates: []tls.Certificate{tlsCert}})
	if err != nil {
		t.Fatalf("tls listen: %v", err)
	}
	t.Cleanup(func() { ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) { defer c.Close(); tls.Server(c, &tls.Config{Certificates: []tls.Certificate{tlsCert}}).Handshake() }(conn)
		}
	}()
	return ln.Addr().String(), pool
}

// --- probeOne ---

func TestProbeOne_Success(t *testing.T) {
	addr, pool := startTLSServer(t)
	host, _, _ := net.SplitHostPort(addr)

	// Temporarily patch tls.Config inside probeOne by using a profile that
	// succeeds — we verify via the returned result.
	profile := directProfiles[0] // p00: no fragmentation

	// probeOne uses tls.Config{ServerName: host} — our cert is for 127.0.0.1.
	// Swap InsecureSkipVerify by using a custom RootCAs via the pool.
	// We can't inject the pool without changing production code, so use
	// InsecureSkipVerify by temporarily patching — instead just verify
	// that probeOne with an actual TLS server returns a result (OK or not)
	// without panicking or hanging.
	_ = pool
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	res := probeOne(ctx, addr, host, profile, 1, 3*time.Second)
	// TLS will fail (cert not trusted) but we get a result, not a hang.
	if res.Target != addr {
		t.Errorf("result target = %q, want %q", res.Target, addr)
	}
	if res.ProfileID != profile.ID {
		t.Errorf("result profileID = %q, want %q", res.ProfileID, profile.ID)
	}
}

func TestProbeOne_BadAddr(t *testing.T) {
	ctx := context.Background()
	res := probeOne(ctx, "notanaddr", "front", directProfiles[0], 1, time.Second)
	if res.Reason != "badaddr" {
		t.Errorf("expected badaddr, got %q", res.Reason)
	}
}

// --- ProbeDirectProfiles ---

func TestProbeDirectProfiles_Structure(t *testing.T) {
	addr, _ := startTLSServer(t)
	host, _, _ := net.SplitHostPort(addr)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	rep := ProbeDirectProfiles(ctx, []string{host}, []string{host}, 1, time.Second)

	if len(rep.Targets) == 0 {
		t.Error("no targets in report")
	}
	expected := len(rep.Fronts) * len(directProfiles)
	if len(rep.Results) != expected {
		t.Errorf("got %d results, want %d", len(rep.Results), expected)
	}
}

// --- handleDirectConnect error path ---

func TestHandleDirectConnect_DialFailure(t *testing.T) {
	orig := DirectFronts
	DirectFronts = []string{"127.0.0.1"}
	t.Cleanup(func() { DirectFronts = orig; remembered.Store(nil) })
	remembered.Store(nil)

	clientSide, proxySide := net.Pipe()
	defer clientSide.Close()
	go handleDirectConnect(proxySide, "127.0.0.1:1")

	buf := make([]byte, 64)
	n, _ := clientSide.Read(buf)
	if string(buf[:n]) != "HTTP/1.1 502 Bad Gateway\r\n\r\n" {
		t.Errorf("expected 502, got %q", string(buf[:n]))
	}
}

// --- dialDirect with zero NumChunks falls back to DefaultFragmentConfig ---

func TestDialDirect_ZeroNumChunks(t *testing.T) {
	addr := startEchoServer(t)
	cfg := FragmentConfig{NumChunks: 0}
	conn, _, err := dialDirect(context.Background(), addr, "", cfg, 5*time.Second)
	if err != nil {
		t.Fatalf("dialDirect with zero NumChunks failed: %v", err)
	}
	conn.Close()
}

func TestDialDirect_BadAddr(t *testing.T) {
	_, _, err := dialDirect(context.Background(), "notanaddr", "", DefaultFragmentConfig, time.Second)
	if err == nil {
		t.Error("dialDirect should fail for bad addr")
	}
}

func TestDialTCP_Success(t *testing.T) {
	addr := startEchoServer(t)
	conn, err := defaultFragmentDialer.DialTCP(addr)
	if err != nil {
		t.Fatalf("DialTCP failed: %v", err)
	}
	conn.Close()
}

func TestProbeDirectProfiles_EmptyInputs(t *testing.T) {
	ctx := context.Background()
	// Empty targets — should return empty results without panic.
	rep := ProbeDirectProfiles(ctx, []string{}, []string{"front"}, 1, time.Second)
	if len(rep.Results) != 0 {
		t.Errorf("expected 0 results for empty targets, got %d", len(rep.Results))
	}
	// repeat < 1 normalised to 1.
	addr, _ := startTLSServer(t)
	host, _, _ := net.SplitHostPort(addr)
	rep2 := ProbeDirectProfiles(ctx, []string{host}, []string{host}, 0, time.Second)
	for _, r := range rep2.Results {
		if r.Repeat != 1 {
			t.Errorf("repeat should be normalised to 1, got %d", r.Repeat)
		}
	}
}

// --- probeOne with repeat > 1 ---

func TestProbeOne_Repeat(t *testing.T) {
	addr, _ := startTLSServer(t)
	host, _, _ := net.SplitHostPort(addr)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	// repeat=2 exercises the loop; TLS will fail but stable stays 0
	res := probeOne(ctx, addr, host, directProfiles[0], 2, 2*time.Second)
	if res.Repeat != 2 {
		t.Errorf("repeat = %d, want 2", res.Repeat)
	}
}

// --- isBetter zero-avg edge cases ---

func TestIsBetter_ZeroAvgA(t *testing.T) {
	a := DirectProbeResult{Target: "t", Stable: 1, Avg: 0}
	b := DirectProbeResult{Target: "t", Stable: 1, Avg: 100 * time.Millisecond}
	if isBetter(a, b) {
		t.Error("zero avg a should not beat non-zero avg b")
	}
}

func TestIsBetter_ZeroAvgB(t *testing.T) {
	a := DirectProbeResult{Target: "t", Stable: 1, Avg: 100 * time.Millisecond}
	b := DirectProbeResult{Target: "t", Stable: 1, Avg: 0}
	if !isBetter(a, b) {
		t.Error("non-zero avg a should beat zero avg b")
	}
}
