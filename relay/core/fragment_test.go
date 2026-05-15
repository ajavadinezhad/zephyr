package core

import (
	"bytes"
	"crypto/tls"
	"net"
	"sync"
	"testing"
	"time"
)

// recordConn records each Write call as a separate chunk.
type recordConn struct {
	net.Conn
	mu     sync.Mutex
	chunks [][]byte
	times  []time.Time
}

func newRecordConn() *recordConn { return &recordConn{} }

func (r *recordConn) Write(b []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := make([]byte, len(b))
	copy(cp, b)
	r.chunks = append(r.chunks, cp)
	r.times = append(r.times, time.Now())
	return len(b), nil
}

func (r *recordConn) allBytes() []byte {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []byte
	for _, c := range r.chunks {
		out = append(out, c...)
	}
	return out
}

func (r *recordConn) numChunks() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.chunks)
}

// Close and addr stubs so recordConn satisfies net.Conn.
func (r *recordConn) Close() error                       { return nil }
func (r *recordConn) LocalAddr() net.Addr                { return &net.TCPAddr{} }
func (r *recordConn) RemoteAddr() net.Addr               { return &net.TCPAddr{} }
func (r *recordConn) SetDeadline(_ time.Time) error      { return nil }
func (r *recordConn) SetReadDeadline(_ time.Time) error  { return nil }
func (r *recordConn) SetWriteDeadline(_ time.Time) error { return nil }
func (r *recordConn) Read(_ []byte) (int, error)         { return 0, nil }

// newTestFragmentConn returns a fragmentConn backed by a recordConn.
func newTestFragmentConn(cfg FragmentConfig) (*fragmentConn, *recordConn) {
	rc := newRecordConn()
	fc := &fragmentConn{Conn: rc, cfg: cfg, firstWrite: true}
	return fc, rc
}

// --- randomSplits ---

func TestRandomSplits_Count(t *testing.T) {
	for _, n := range []int{10, 100, 300} {
		for _, count := range []int{1, 5, n - 1} {
			got := randomSplits(n, count)
			if len(got) != count {
				t.Errorf("randomSplits(%d,%d): got %d splits, want %d", n, count, len(got), count)
			}
		}
	}
}

func TestRandomSplits_Sorted(t *testing.T) {
	splits := randomSplits(300, 86)
	for i := 1; i < len(splits); i++ {
		if splits[i] <= splits[i-1] {
			t.Errorf("not sorted at index %d: %d <= %d", i, splits[i], splits[i-1])
		}
	}
}

func TestRandomSplits_Bounds(t *testing.T) {
	n := 300
	splits := randomSplits(n, 86)
	for _, s := range splits {
		if s < 1 || s >= n {
			t.Errorf("split %d out of [1,%d)", s, n)
		}
	}
}

func TestRandomSplits_Distinct(t *testing.T) {
	splits := randomSplits(300, 86)
	seen := map[int]bool{}
	for _, s := range splits {
		if seen[s] {
			t.Errorf("duplicate split point %d", s)
		}
		seen[s] = true
	}
}

func TestRandomSplits_AllBoundaries(t *testing.T) {
	// When count >= n-1, we should get every position 1..n-1.
	n := 5
	splits := randomSplits(n, n) // count > n-1
	if len(splits) != n-1 {
		t.Fatalf("got %d splits, want %d", len(splits), n-1)
	}
	for i, s := range splits {
		if s != i+1 {
			t.Errorf("splits[%d]=%d, want %d", i, s, i+1)
		}
	}
}

// --- fragmentConn ---

func TestFragmentConn_DataIntegrity(t *testing.T) {
	data := bytes.Repeat([]byte("ABCDEFGHIJ"), 30) // 300 bytes
	cfg := FragmentConfig{NumChunks: 87, Delay: 0}
	fc, rc := newTestFragmentConn(cfg)

	n, err := fc.Write(data)
	if err != nil {
		t.Fatalf("Write error: %v", err)
	}
	if n != len(data) {
		t.Errorf("wrote %d bytes, want %d", n, len(data))
	}
	if got := rc.allBytes(); !bytes.Equal(got, data) {
		t.Errorf("reassembled data mismatch")
	}
}

func TestFragmentConn_NumChunks(t *testing.T) {
	data := bytes.Repeat([]byte("X"), 300)
	cfg := FragmentConfig{NumChunks: 87, Delay: 0}
	fc, rc := newTestFragmentConn(cfg)

	_, _ = fc.Write(data)
	got := rc.numChunks()
	if got != 87 {
		t.Errorf("got %d chunks, want 87", got)
	}
}

func TestFragmentConn_OnlyFirstWriteFragmented(t *testing.T) {
	data := bytes.Repeat([]byte("X"), 300)
	cfg := FragmentConfig{NumChunks: 87, Delay: 0}
	fc, rc := newTestFragmentConn(cfg)

	_, _ = fc.Write(data) // first write — fragmented
	beforeCount := rc.numChunks()

	_, _ = fc.Write(data) // second write — must be a single chunk
	afterCount := rc.numChunks()

	if afterCount != beforeCount+1 {
		t.Errorf("second write produced %d extra chunks, want 1", afterCount-beforeCount)
	}
}

func TestFragmentConn_SmallData(t *testing.T) {
	// Data smaller than NumChunks — should still fragment but capped to len(data).
	data := []byte("hi") // 2 bytes
	cfg := FragmentConfig{NumChunks: 87, Delay: 0}
	fc, rc := newTestFragmentConn(cfg)

	_, _ = fc.Write(data)
	if got := rc.allBytes(); !bytes.Equal(got, data) {
		t.Errorf("small data mismatch")
	}
}

func TestFragmentConn_SingleByte(t *testing.T) {
	data := []byte("Z")
	cfg := FragmentConfig{NumChunks: 87, Delay: 0}
	fc, rc := newTestFragmentConn(cfg)

	_, _ = fc.Write(data)
	// 1 byte: cannot split, should be single write
	if got := rc.numChunks(); got != 1 {
		t.Errorf("single byte: got %d chunks, want 1", got)
	}
	if got := rc.allBytes(); !bytes.Equal(got, data) {
		t.Errorf("single byte data mismatch")
	}
}

func TestFragmentConn_DelayApplied(t *testing.T) {
	data := bytes.Repeat([]byte("X"), 10)
	cfg := FragmentConfig{NumChunks: 3, Delay: 10 * time.Millisecond}
	fc, rc := newTestFragmentConn(cfg)

	start := time.Now()
	_, _ = fc.Write(data)
	elapsed := time.Since(start)

	_ = rc
	// 3 chunks → 3 delays of 10ms each → at least 20ms total (conservative)
	if elapsed < 20*time.Millisecond {
		t.Errorf("delay too short: %v, expected >= 20ms", elapsed)
	}
}

// --- FragmentConfig.numChunksFor ---

func TestNumChunksFor(t *testing.T) {
	cfg := FragmentConfig{NumChunks: 87}
	cases := []struct {
		n    int
		want int
	}{
		{0, 1},
		{1, 1},
		{2, 2},
		{87, 87},
		{300, 87},
		{50, 50}, // n < NumChunks → cap to n
	}
	for _, tc := range cases {
		got := cfg.numChunksFor(tc.n)
		if got != tc.want {
			t.Errorf("numChunksFor(%d) = %d, want %d", tc.n, got, tc.want)
		}
	}
}

// --- equalSplits ---

func TestEqualSplits_Count(t *testing.T) {
	cases := []struct{ n, chunks, want int }{
		{100, 4, 3},
		{100, 1, 0},
		{1, 4, 0},
		{10, 10, 9},
	}
	for _, tc := range cases {
		got := equalSplits(tc.n, tc.chunks)
		if len(got) != tc.want {
			t.Errorf("equalSplits(%d,%d): got %d points, want %d", tc.n, tc.chunks, len(got), tc.want)
		}
	}
}

func TestEqualSplits_Bounds(t *testing.T) {
	splits := equalSplits(100, 8)
	for _, s := range splits {
		if s <= 0 || s >= 100 {
			t.Errorf("split %d out of (0,100)", s)
		}
	}
}

func TestEqualSplits_Sorted(t *testing.T) {
	splits := equalSplits(100, 8)
	for i := 1; i < len(splits); i++ {
		if splits[i] <= splits[i-1] {
			t.Errorf("not sorted at %d: %d <= %d", i, splits[i], splits[i-1])
		}
	}
}

// --- sniSplits / tlsSNIHostRange ---

func TestSNISplits_NoSNI(t *testing.T) {
	// Random data — not a TLS ClientHello, should return nil.
	data := bytes.Repeat([]byte{0x01, 0x02, 0x03}, 100)
	if got := sniSplits(data); got != nil {
		t.Errorf("expected nil for non-TLS data, got %v", got)
	}
}

func TestTLSSNIHostRange_NotClientHello(t *testing.T) {
	data := []byte{0x17, 0x03, 0x03, 0x00, 0x05} // TLS application data, not handshake
	s, e := tlsSNIHostRange(data)
	if s != 0 || e != 0 {
		t.Errorf("expected (0,0) for non-ClientHello, got (%d,%d)", s, e)
	}
}

func TestTLSSNIHostRange_TooShort(t *testing.T) {
	s, e := tlsSNIHostRange([]byte{0x16, 0x03})
	if s != 0 || e != 0 {
		t.Errorf("expected (0,0) for short data, got (%d,%d)", s, e)
	}
}

func TestTLSSNIHostRange_TruncatedPaths(t *testing.T) {
	// Each case triggers a different early-return in tlsSNIHostRange.
	cases := []struct {
		name string
		data []byte
	}{
		{
			// Valid record+handshake header but truncated before version+random+session.
			name: "truncated after handshake header",
			data: func() []byte {
				b := make([]byte, 9)
				b[0] = 0x16 // TLS handshake
				b[5] = 0x01 // ClientHello
				return b
			}(),
		},
		{
			// Truncated inside session length field.
			name: "truncated session",
			data: func() []byte {
				b := make([]byte, 5+4+2+32+1) // just enough for version+random, sessionLen=0 but no cipher bytes
				b[0] = 0x16
				b[5] = 0x01
				b[5+4+2+32] = 0x01 // sessionLen=1 but no session data follows
				return b
			}(),
		},
		{
			// No extensions present (extLen area missing).
			name: "no SNI extension",
			data: func() []byte {
				// Build a minimal ClientHello with one non-SNI extension.
				// type=0x0001 (not SNI), len=0
				hello := captureClientHello(t, "www.youtube.com")
				if len(hello) == 0 {
					return hello
				}
				// Truncate after extensions block starts — forces loop exit with no SNI found.
				start, _ := tlsSNIHostRange(hello)
				if start <= 0 {
					return hello
				}
				// Return data truncated just before the SNI extension body.
				return hello[:start-4]
			}(),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s, e := tlsSNIHostRange(tc.data)
			if s != 0 || e != 0 {
				t.Errorf("expected (0,0), got (%d,%d)", s, e)
			}
		})
	}
}

func TestSNIHostRangeInExtension_Truncated(t *testing.T) {
	// listLen > available data.
	data := []byte{0x00, 0x10, 0x00} // listLen=16 but only 1 byte follows
	s, e := sniHostRangeInExtension(data, 0, len(data))
	if s != 0 || e != 0 {
		t.Errorf("expected (0,0) for truncated list, got (%d,%d)", s, e)
	}

	// nameType != 0 (skip non-host_name entries).
	data2 := []byte{
		0x00, 0x05, // listLen = 5
		0x01,       // nameType = 1 (not host_name)
		0x00, 0x02, // nameLen = 2
		0x41, 0x42, // "AB"
	}
	s, e = sniHostRangeInExtension(data2, 0, len(data2))
	if s != 0 || e != 0 {
		t.Errorf("expected (0,0) for non-host_name type, got (%d,%d)", s, e)
	}
}

func TestTLSSNIHostRange_RealClientHello(t *testing.T) {
	// Capture a real TLS ClientHello by intercepting the first Write.
	hello := captureClientHello(t, "www.youtube.com")
	if len(hello) == 0 {
		t.Fatal("no ClientHello captured")
	}
	start, end := tlsSNIHostRange(hello)
	if start <= 0 || end <= start {
		t.Fatalf("SNI range not found in real ClientHello: start=%d end=%d", start, end)
	}
	sni := string(hello[start:end])
	if sni != "www.youtube.com" {
		t.Errorf("SNI = %q, want www.youtube.com", sni)
	}
}

func TestSNISplits_RealClientHello(t *testing.T) {
	hello := captureClientHello(t, "www.youtube.com")
	splits := sniSplits(hello)
	if len(splits) == 0 {
		t.Fatal("sniSplits returned nil for real ClientHello")
	}
	for _, s := range splits {
		if s <= 0 || s >= len(hello) {
			t.Errorf("split point %d out of bounds [1,%d)", s, len(hello))
		}
	}
}

// captureClientHello dials a loopback TCP server, initiates a TLS handshake
// with the given SNI, and returns the raw bytes of the first Write (ClientHello).
func captureClientHello(t *testing.T, serverName string) []byte {
	t.Helper()

	// Start a raw TCP server that accepts one connection and reads the first write.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	ch := make(chan []byte, 1)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			ch <- nil
			return
		}
		defer conn.Close()
		buf := make([]byte, 4096)
		n, _ := conn.Read(buf)
		ch <- buf[:n]
	}()

	// Connect and start TLS — the first Write will be the ClientHello.
	raw, err := net.DialTimeout("tcp", ln.Addr().String(), 2*time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	tlsConn := tls.Client(raw, &tls.Config{ServerName: serverName, InsecureSkipVerify: true})
	go tlsConn.Handshake() //nolint — we only care about the write, not the result
	hello := <-ch
	raw.Close()
	return hello
}

// --- fragmentConn with custom Splits func ---

func TestFragmentConn_CustomSplits(t *testing.T) {
	data := bytes.Repeat([]byte("X"), 100)
	// Splits func that always returns one split at position 10.
	cfg := FragmentConfig{
		NumChunks: 87,
		Delay:     0,
		Splits:    func(b []byte) []int { return []int{10} },
	}
	fc, rc := newTestFragmentConn(cfg)
	fc.Write(data)
	if rc.numChunks() != 2 {
		t.Errorf("expected 2 chunks from custom splits, got %d", rc.numChunks())
	}
	if !bytes.Equal(rc.allBytes(), data) {
		t.Error("data integrity failed with custom splits")
	}
}

func TestFragmentConn_SplitsFallbackToRandom(t *testing.T) {
	data := bytes.Repeat([]byte("X"), 100)
	// Splits func returns nil — should fall back to random splits.
	cfg := FragmentConfig{
		NumChunks: 10,
		Delay:     0,
		Splits:    func(b []byte) []int { return nil },
	}
	fc, rc := newTestFragmentConn(cfg)
	fc.Write(data)
	if rc.numChunks() != 10 {
		t.Errorf("expected 10 chunks from random fallback, got %d", rc.numChunks())
	}
}

// --- IsDirectDomain ---

func TestIsDirectDomain(t *testing.T) {
	SetDirectEnabled(true)
	cases := []struct {
		host string
		want bool
	}{
		{"youtube.com", true},
		{"www.youtube.com", true},
		{"i.ytimg.com", true},
		{"googleapis.com", true},
		{"maps.googleapis.com", true},
		{"mail.google.com", true},
		{"gstatic.com", true},
		{"www.gstatic.com", true},
		{"instagram.com", false},
		{"twitter.com", false},
		{"example.com", false},
		{"notgoogle.com", false},
		// with port
		{"youtube.com:443", true},
		{"instagram.com:443", false},
	}
	for _, tc := range cases {
		got := IsDirectDomain(tc.host)
		if got != tc.want {
			t.Errorf("IsDirectDomain(%q) = %v, want %v", tc.host, got, tc.want)
		}
	}
}

func TestIsDirectDomain_DisabledFlag(t *testing.T) {
	orig := GetDirectEnabled()
	defer SetDirectEnabled(orig)

	SetDirectEnabled(false)
	if IsDirectDomain("youtube.com") {
		t.Error("IsDirectDomain should return false when DirectEnabled=false")
	}
}
