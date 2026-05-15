package core

import (
	"context"
	"io"
	"net"
	"sync"
	"testing"
	"time"
)

func startEchoServer(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("echo server listen: %v", err)
	}
	t.Cleanup(func() { ln.Close() })
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		io.Copy(conn, conn)
	}()
	return ln.Addr().String()
}

// --- pipe ---

func TestPipe_BidirectionalCopy(t *testing.T) {
	a, b := net.Pipe()
	defer a.Close()
	defer b.Close()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		pipe(a, b)
	}()

	msg := []byte("hello from b")
	b.Write(msg)
	b.Close()

	got := make([]byte, len(msg))
	io.ReadFull(a, got)
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Error("pipe did not finish after both sides closed")
	}
}

func TestPipe_BothGoroutinesFinish(t *testing.T) {
	a, b := net.Pipe()
	start := time.Now()
	go func() {
		time.Sleep(20 * time.Millisecond)
		a.Close()
		b.Close()
	}()
	pipe(a, b)
	if time.Since(start) < 15*time.Millisecond {
		t.Error("pipe returned too fast — likely only waited for one goroutine")
	}
}

// --- dialDirect ---

func TestDialDirect_Success(t *testing.T) {
	addr := startEchoServer(t)
	conn, _, err := dialDirect(context.Background(), addr, "", DefaultFragmentConfig, 5*time.Second)
	if err != nil {
		t.Fatalf("dialDirect failed: %v", err)
	}
	defer conn.Close()

	msg := []byte("ping")
	conn.Write(msg)
	if tc, ok := conn.(*fragmentConn).Conn.(*net.TCPConn); ok {
		tc.CloseWrite()
	}
	got, _ := io.ReadAll(conn)
	if string(got) != string(msg) {
		t.Errorf("echo mismatch: got %q, want %q", got, msg)
	}
}

func TestDialDirect_Failure(t *testing.T) {
	conn, _, err := dialDirect(context.Background(), "127.0.0.1:1", "", DefaultFragmentConfig, 2*time.Second)
	if err == nil {
		conn.Close()
		t.Error("dialDirect should fail for unreachable address")
	}
}

// --- handleDirectConnect ---

func TestHandleDirectConnect_PipesData(t *testing.T) {
	// Bypass dialFragment by testing pipe directly with pre-connected conns.
	clientSide, proxySide := net.Pipe()
	serverSide, echoSide := net.Pipe()
	defer clientSide.Close()

	go io.Copy(echoSide, echoSide)
	go func() {
		defer proxySide.Close()
		defer serverSide.Close()
		proxySide.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))
		pipe(proxySide, serverSide)
	}()

	resp := make([]byte, len("HTTP/1.1 200 Connection Established\r\n\r\n"))
	io.ReadFull(clientSide, resp)
	if string(resp) != "HTTP/1.1 200 Connection Established\r\n\r\n" {
		t.Errorf("unexpected response: %q", resp)
	}

	msg := []byte("hello direct")
	clientSide.Write(msg)
	got := make([]byte, len(msg))
	io.ReadFull(clientSide, got)
	if string(got) != string(msg) {
		t.Errorf("echo mismatch: got %q, want %q", got, msg)
	}
}

// --- SetDirectEnabled ---

func TestSetDirectEnabled_ConcurrentToggle(t *testing.T) {
	orig := GetDirectEnabled()
	defer SetDirectEnabled(orig)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(2)
		go func() { defer wg.Done(); SetDirectEnabled(true) }()
		go func() { defer wg.Done(); _ = GetDirectEnabled() }()
	}
	wg.Wait()
}
