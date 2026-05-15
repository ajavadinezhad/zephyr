package core

import (
	"math/rand/v2"
	"net"
	"time"
)

// FragmentConfig controls how the TLS ClientHello is split into TCP segments.
// Based on gfw_resist_tls_proxy / gfw_resist_HTTPS_proxy parameters tuned for
// Iran's SNDPI (Hamrah-e Aval / Irancell ranges from the upstream research).
type FragmentConfig struct {
	// NumChunks is how many TCP segments the ClientHello is split into.
	// Iran SNDPI: 80–250 recommended; default 87 matches upstream.
	NumChunks int
	// Delay is the pause between each chunk send.
	// Iran SNDPI: 2–20 ms per chunk; default 5 ms.
	Delay time.Duration
	// Splits, if non-nil, is called on the first write to produce split points
	// instead of the default random split. Used by direct profiles.
	Splits func([]byte) []int
}

// DefaultFragmentConfig is calibrated for Iran's SNDPI based on
// gfw_resist_HTTPS_proxy parameters (num_fragment=87, fragment_sleep=5ms).
var DefaultFragmentConfig = FragmentConfig{
	NumChunks: 87,
	Delay:     5 * time.Millisecond,
}

type fragmentDialer struct {
	cfg FragmentConfig
}

var defaultFragmentDialer = &fragmentDialer{cfg: DefaultFragmentConfig}

func (d *fragmentDialer) DialTCP(addr string) (net.Conn, error) {
	conn, err := net.DialTimeout("tcp", addr, 15*time.Second)
	if err != nil {
		return nil, err
	}
	// TCP_NODELAY forces the kernel to send each Write as its own TCP segment
	// rather than coalescing them (Nagle's algorithm). Without this, the OS
	// would reassemble our fragments before they leave the machine.
	setTCPNoDelay(conn)
	return &fragmentConn{Conn: conn, cfg: d.cfg, firstWrite: true}, nil
}

// fragmentConn wraps a net.Conn and fragments only the very first Write call
// (which carries the TLS ClientHello) into NumChunks random-boundary segments
// with Delay between each. Subsequent writes go through unmodified.
type fragmentConn struct {
	net.Conn
	cfg        FragmentConfig
	firstWrite bool
}

func (c *fragmentConn) Write(b []byte) (int, error) {
	if !c.firstWrite {
		return c.Conn.Write(b)
	}
	c.firstWrite = false

	n := len(b)
	numChunks := c.cfg.numChunksFor(n)
	if numChunks <= 1 {
		return c.Conn.Write(b)
	}

	var splits []int
	if c.cfg.Splits != nil {
		splits = c.cfg.Splits(b)
	}
	if len(splits) == 0 {
		splits = randomSplits(n, numChunks-1)
	}

	written := 0
	prev := 0
	for _, s := range splits {
		nw, err := c.Conn.Write(b[prev:s])
		written += nw
		if err != nil {
			return written, err
		}
		prev = s
		time.Sleep(c.cfg.Delay)
	}
	nw, err := c.Conn.Write(b[prev:])
	written += nw
	return written, err
}

// numChunksFor returns the actual number of chunks to use given data length n.
func (cfg FragmentConfig) numChunksFor(n int) int {
	if n < 2 {
		return 1
	}
	if cfg.NumChunks > n {
		return n
	}
	return cfg.NumChunks
}

// equalSplits divides n bytes into numChunks equal parts and returns the split points.
func equalSplits(n, numChunks int) []int {
	if numChunks <= 1 || n < 2 {
		return nil
	}
	out := make([]int, 0, numChunks-1)
	for i := 1; i < numChunks; i++ {
		p := (n * i) / numChunks
		if p > 0 && p < n && (len(out) == 0 || out[len(out)-1] != p) {
			out = append(out, p)
		}
	}
	return out
}

// sniSplits returns split points around the SNI hostname in a TLS ClientHello,
// plus fixed points at bytes 1 and 5 to split the record/handshake headers.
// Falls back to nil if the SNI cannot be located.
func sniSplits(data []byte) []int {
	hostStart, hostEnd := tlsSNIHostRange(data)
	if hostStart <= 0 || hostEnd <= hostStart {
		return nil
	}
	candidates := []int{1, 5, hostStart - 8, hostStart - 1, hostStart + 1, hostStart + 7, hostEnd}
	out := make([]int, 0, len(candidates))
	seen := map[int]bool{}
	for _, p := range candidates {
		if p > 0 && p < len(data) && !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	sortInts(out)
	return out
}

// tlsSNIHostRange parses a raw TLS ClientHello and returns the byte range [start, end)
// of the SNI hostname within the data slice. Returns (0, 0) on any parse failure.
func tlsSNIHostRange(data []byte) (int, int) {
	if len(data) < 5 || data[0] != 0x16 {
		return 0, 0
	}
	i := 5
	if len(data) < i+4 || data[i] != 0x01 {
		return 0, 0
	}
	i += 4
	if len(data) < i+2+32+1 {
		return 0, 0
	}
	i += 2 + 32
	sessionLen := int(data[i])
	i++
	if len(data) < i+sessionLen+2 {
		return 0, 0
	}
	i += sessionLen
	cipherLen := int(data[i])<<8 | int(data[i+1])
	i += 2
	if len(data) < i+cipherLen+1 {
		return 0, 0
	}
	i += cipherLen
	compressionLen := int(data[i])
	i++
	if len(data) < i+compressionLen+2 {
		return 0, 0
	}
	i += compressionLen
	extLen := int(data[i])<<8 | int(data[i+1])
	i += 2
	extEnd := i + extLen
	if len(data) < extEnd {
		return 0, 0
	}
	for i+4 <= extEnd {
		typ := int(data[i])<<8 | int(data[i+1])
		l := int(data[i+2])<<8 | int(data[i+3])
		i += 4
		if i+l > extEnd {
			return 0, 0
		}
		if typ == 0x0000 {
			return sniHostRangeInExtension(data, i, i+l)
		}
		i += l
	}
	return 0, 0
}

func sniHostRangeInExtension(data []byte, start, end int) (int, int) {
	i := start
	if i+2 > end {
		return 0, 0
	}
	listLen := int(data[i])<<8 | int(data[i+1])
	i += 2
	if i+listLen > end {
		return 0, 0
	}
	for i+3 <= end {
		nameType := data[i]
		nameLen := int(data[i+1])<<8 | int(data[i+2])
		i += 3
		if i+nameLen > end {
			return 0, 0
		}
		if nameType == 0 {
			return i, i + nameLen
		}
		i += nameLen
	}
	return 0, 0
}

// randomSplits returns count sorted random positions in (0, n), all distinct.
func randomSplits(n, count int) []int {
	if count >= n-1 {
		out := make([]int, n-1)
		for i := range out {
			out[i] = i + 1
		}
		return out
	}
	pool := make([]int, n-1)
	for i := range pool {
		pool[i] = i + 1
	}
	for i := 0; i < count; i++ {
		j := i + rand.IntN(n-1-i)
		pool[i], pool[j] = pool[j], pool[i]
	}
	chosen := pool[:count]
	sortInts(chosen)
	return chosen
}

func sortInts(a []int) {
	for i := 1; i < len(a); i++ {
		v := a[i]
		j := i - 1
		for j >= 0 && a[j] > v {
			a[j+1] = a[j]
			j--
		}
		a[j+1] = v
	}
}
