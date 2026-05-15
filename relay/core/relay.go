package core

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

const maxRelayBody = 16 * 1024 * 1024

// socketProtectFunc, if set, is called with each new TCP socket file descriptor
// before the connection is established. On Android this should be wired to
// VpnService.protect() so relay sockets bypass the TUN and don't loop back
// through the local MITM proxy.
var socketProtectFunc func(fd int)

// SetSocketProtectFunc registers the per-socket protect callback.
// Pass nil to clear it. Thread-safe.
func SetSocketProtectFunc(fn func(fd int)) {
	socketProtectFunc = fn
}

// ParseURLList splits a comma-separated URL string and strips all whitespace
// from each entry, including embedded newlines from copy-paste artifacts.
func ParseURLList(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		u := strings.Map(func(r rune) rune {
			if r == ' ' || r == '\n' || r == '\r' || r == '\t' {
				return -1
			}
			return r
		}, p)
		if u != "" {
			out = append(out, u)
		}
	}
	return out
}

var activeURLIdx atomic.Int64

// perURLTimeout splits the total timeout budget evenly across n URLs so that
// a single slow or unreachable URL does not burn the whole deadline before
// failover kicks in. A minimum of 8s is enforced so each attempt has enough
// time to complete a normal relay call.
func perURLTimeout(total time.Duration, n int) time.Duration {
	if n <= 1 {
		return total
	}
	per := total / time.Duration(n)
	const min = 8 * time.Second
	if per < min {
		return min
	}
	return per
}

type workerResponse struct {
	Status  int            `json:"s"`
	Headers map[string]any `json:"h"`
	Body    string         `json:"b"`
	Error   string         `json:"e"`
}

// RelayResponse is the decoded response from the relay chain.
type RelayResponse struct {
	Status  int
	Headers map[string][]string
	Body    []byte
}

// NewHTTPClient returns an http.Client configured for relay use.
func NewHTTPClient(timeout time.Duration) *http.Client {
	dialer := &net.Dialer{
		Timeout:   10 * time.Second,
		KeepAlive: 30 * time.Second,
		Control: func(network, address string, c syscall.RawConn) error {
			if fn := socketProtectFunc; fn != nil {
				return c.Control(func(fd uintptr) { fn(int(fd)) })
			}
			return nil
		},
	}
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			TLSClientConfig:     &tls.Config{MinVersion: tls.VersionTLS12},
			DialContext:         dialer.DialContext,
			MaxIdleConns:        128,
			MaxIdleConnsPerHost: 32,
			IdleConnTimeout:     120 * time.Second,
			TLSHandshakeTimeout: 10 * time.Second,
			ForceAttemptHTTP2:   true,
		},
	}
}

// RelayRequest sends method+targetURL through the domain-fronted Apps Script relay chain.
// It is a convenience wrapper around RelayRequestMulti with a single URL.
func RelayRequest(
	client *http.Client,
	appScriptURL, frontDomain, authKey,
	method, targetURL string,
	headers map[string]string,
	body []byte,
	timeout time.Duration,
) (RelayResponse, error) {
	return RelayRequestMulti(client, []string{appScriptURL}, frontDomain, authKey, method, targetURL, headers, body, timeout)
}

// RelayRequestMulti uses a sticky circular failover across appScriptURLs.
// It sticks to the current URL until it fails (e.g. quota exhausted), then
// advances to the next one and sticks there. When the last URL fails it wraps
// back to the first, which will have had its quota reset by then.
func RelayRequestMulti(
	client *http.Client,
	appScriptURLs []string,
	frontDomain, authKey,
	method, targetURL string,
	headers map[string]string,
	body []byte,
	timeout time.Duration,
) (RelayResponse, error) {
	n := len(appScriptURLs)
	if n == 0 {
		return RelayResponse{}, fmt.Errorf("no Apps Script URLs configured")
	}
	payload := buildRelayPayload(authKey, method, targetURL, headers, body)
	start := int(activeURLIdx.Load()) % n
	// Each parallel goroutine gets the full timeout since they race simultaneously

	// Race all URLs in parallel — first success wins
	type raceResult struct {
		resp RelayResponse
		idx  int
		err  error
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch := make(chan raceResult, n)
	for i := 0; i < n; i++ {
		idx := (start + i) % n
		go func(idx int) {
			resp, err := tryOneURL(ctx, client, appScriptURLs[idx], frontDomain, payload, timeout)
			ch <- raceResult{resp, idx, err}
		}(idx)
	}

	var lastErr error
	for i := 0; i < n; i++ {
		r := <-ch
		if r.err == nil {
			cancel() // cancel losing goroutines
			if r.idx != start {
				activeURLIdx.Store(int64(r.idx))
			}
			return r.resp, nil
		}
		lastErr = r.err
	}
	return RelayResponse{}, lastErr
}

// Coalescer batches concurrent relay requests into a single Apps Script call
// using the existing doBatch_ / fetchAll support in Code.gs.
// Requests that arrive within window of each other are grouped (up to maxBatch).
type Coalescer struct {
	client        *http.Client
	appScriptURLs []string
	frontDomain   string
	authKey       string
	timeout       time.Duration
	window        time.Duration
	maxBatch      int
	ch            chan *coalescerItem
	cache         *responseCache
	stopCh        chan struct{}
	stopOnce      sync.Once
}

const keepaliveInterval = 2*time.Minute + 30*time.Second

type coalescerItem struct {
	method    string
	targetURL string
	headers   map[string]string
	body      []byte
	result    chan coalescerResult
}

type coalescerResult struct {
	resp RelayResponse
	err  error
}

type batchPayloadItem struct {
	Method   string            `json:"m"`
	URL      string            `json:"u"`
	Headers  map[string]string `json:"h"`
	Body     string            `json:"b,omitempty"`
	Redirect bool              `json:"r"`
}

type batchEnvelope struct {
	Key      string             `json:"k"`
	Items    []batchPayloadItem `json:"q"`
	Compress int                `json:"gz"`
}

type batchResponseEnvelope struct {
	Items []workerResponse `json:"q"`
}

// NewCoalescer creates and starts a request coalescer.
func NewCoalescer(client *http.Client, appScriptURLs []string, frontDomain, authKey string, timeout time.Duration) *Coalescer {
	c := &Coalescer{
		client:        client,
		appScriptURLs: appScriptURLs,
		frontDomain:   frontDomain,
		authKey:       authKey,
		timeout:       timeout,
		window:        3 * time.Millisecond,
		maxBatch:      20,
		ch:            make(chan *coalescerItem, 512),
		cache:         newResponseCache(),
		stopCh:        make(chan struct{}),
	}
	go c.run()
	go c.keepaliveLoop()
	return c
}

// Stop shuts down the keepalive loop. Safe to call multiple times.
func (c *Coalescer) Stop() {
	c.stopOnce.Do(func() { close(c.stopCh) })
}

// Warmup fires a background relay request to pre-warm the Apps Script instance
// so the first real request does not pay the cold-start cost.
func (c *Coalescer) Warmup() {
	go func() {
		_, _ = c.Submit("HEAD", "https://www.google.com/generate_204", map[string]string{}, nil)
	}()
}

// keepaliveLoop sends a lightweight ping through every configured script URL
// every keepaliveInterval to prevent Apps Script cold starts.
func (c *Coalescer) keepaliveLoop() {
	ticker := time.NewTicker(keepaliveInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			for _, u := range c.appScriptURLs {
				u := u
				go func() {
					ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
					defer cancel()
					payload := buildRelayPayload(c.authKey, "HEAD", "https://www.gstatic.com/generate_204", map[string]string{}, nil)
					_, _ = tryOneURL(ctx, c.client, u, c.frontDomain, payload, 15*time.Second)
				}()
			}
		case <-c.stopCh:
			return
		}
	}
}

// Submit queues a relay request and blocks until the response is ready.
// GET responses that carry a positive Cache-Control max-age are served from an
// in-memory cache on subsequent calls, bypassing the relay entirely.
func (c *Coalescer) Submit(method, targetURL string, headers map[string]string, body []byte) (RelayResponse, error) {
	if OnRequest != nil {
		OnRequest(method, targetURL)
	}
	if method == "GET" && len(body) == 0 {
		if e := c.cache.get(targetURL); e != nil {
			return RelayResponse{Status: e.status, Headers: e.headers, Body: e.body}, nil
		}
	}

	item := &coalescerItem{
		method:    method,
		targetURL: targetURL,
		headers:   headers,
		body:      body,
		result:    make(chan coalescerResult, 1),
	}
	c.ch <- item
	r := <-item.result
	if r.err != nil {
		return r.resp, r.err
	}

	if ttl := cacheableMaxAge(method, headers, r.resp.Headers, r.resp.Status, targetURL); ttl > 0 {
		bodyCopy := make([]byte, len(r.resp.Body))
		copy(bodyCopy, r.resp.Body)
		headersCopy := make(map[string][]string, len(r.resp.Headers))
		for k, v := range r.resp.Headers {
			headersCopy[k] = v
		}
		c.cache.set(targetURL, &cacheEntry{
			status:  r.resp.Status,
			headers: headersCopy,
			body:    bodyCopy,
			expiry:  time.Now().Add(ttl),
		})
	}

	return r.resp, r.err
}

const burstWindow = 10 * time.Millisecond

func (c *Coalescer) run() {
	for {
		var first *coalescerItem
		select {
		case first = <-c.ch:
		case <-c.stopCh:
			return
		}
		batch := []*coalescerItem{first}

		// If requests are already queued behind the first one, widen the
		// collection window so the whole burst is captured in one batch.
		w := c.window
		if len(c.ch) > 0 {
			w = burstWindow
		}

		timer := time.NewTimer(w)
	collect:
		for len(batch) < c.maxBatch {
			select {
			case item := <-c.ch:
				batch = append(batch, item)
			case <-timer.C:
				break collect
			case <-c.stopCh:
				timer.Stop()
				return
			}
		}
		timer.Stop()

		prioritizeBatch(batch)

		if len(batch) == 1 {
			go c.flushSingle(batch[0])
			continue
		}

		// Multiple requests — send as one batch call.
		go c.flush(batch)
	}
}

func (c *Coalescer) flushSingle(item *coalescerItem) {
	resp, err := RelayRequestMulti(c.client, c.appScriptURLs, c.frontDomain, c.authKey,
		item.method, item.targetURL, item.headers, item.body, c.timeout)
	item.result <- coalescerResult{resp, err}
}

func prioritizeBatch(batch []*coalescerItem) {
	sort.SliceStable(batch, func(i, j int) bool {
		return requestPriority(batch[i]) < requestPriority(batch[j])
	})
}

func requestPriority(item *coalescerItem) int {
	method := strings.ToUpper(item.method)
	target := strings.ToLower(item.targetURL)
	accept := strings.ToLower(headerValue(item.headers, "Accept"))
	dest := strings.ToLower(headerValue(item.headers, "Sec-Fetch-Dest"))

	if method == "GET" {
		switch {
		case strings.Contains(accept, "text/html") || dest == "document":
			return 0
		case strings.Contains(accept, "text/css") || hasURLPathSuffix(target, ".css") || dest == "style":
			return 5
		case strings.Contains(accept, "javascript") || hasURLPathSuffix(target, ".js") || dest == "script":
			return 10
		case dest == "font" || hasURLPathSuffix(target, ".woff") || hasURLPathSuffix(target, ".woff2") || hasURLPathSuffix(target, ".ttf"):
			return 20
		case dest == "image" || isImageURL(target):
			return 30
		case isTelemetryURL(target):
			return 80
		default:
			return 40
		}
	}

	if isTelemetryURL(target) {
		return 80
	}
	return 40
}

func headerValue(headers map[string]string, key string) string {
	for k, v := range headers {
		if strings.EqualFold(k, key) {
			return v
		}
	}
	return ""
}

func hasURLPathSuffix(raw, suffix string) bool {
	path := raw
	if i := strings.IndexByte(path, '?'); i >= 0 {
		path = path[:i]
	}
	return strings.HasSuffix(path, suffix)
}

func isImageURL(raw string) bool {
	for _, ext := range []string{".png", ".jpg", ".jpeg", ".gif", ".svg", ".ico", ".webp", ".avif"} {
		if hasURLPathSuffix(raw, ext) {
			return true
		}
	}
	return false
}

func isTelemetryURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	host := strings.ToLower(u.Hostname())
	path := strings.ToLower(u.EscapedPath())

	if strings.Contains(path, "gen_204") ||
		strings.Contains(path, "generate_204") ||
		strings.Contains(path, "domainreliability/upload") ||
		strings.Contains(path, "/collect") ||
		strings.Contains(path, "/g/collect") ||
		strings.Contains(path, "/log") {
		return true
	}

	return strings.Contains(host, "analytics") ||
		strings.Contains(host, "doubleclick.net") ||
		strings.Contains(host, "googletagmanager.com") ||
		strings.Contains(host, "googleadservices.com")
}

func (c *Coalescer) flush(batch []*coalescerItem) {
	if len(c.appScriptURLs) == 0 {
		c.failAll(batch, fmt.Errorf("no Apps Script URLs configured"))
		return
	}

	items := make([]batchPayloadItem, len(batch))
	for i, item := range batch {
		pi := batchPayloadItem{
			Method:   strings.ToUpper(item.method),
			URL:      item.targetURL,
			Headers:  item.headers,
			Redirect: false,
		}
		if len(item.body) > 0 {
			pi.Body = base64.StdEncoding.EncodeToString(item.body)
		}
		items[i] = pi
	}

	env := batchEnvelope{Key: c.authKey, Items: items, Compress: 1}
	payload, err := json.Marshal(env)
	if err != nil {
		c.failAll(batch, fmt.Errorf("batch marshal: %w", err))
		return
	}

	n := len(c.appScriptURLs)
	start := int(activeURLIdx.Load()) % n
	// Each parallel goroutine gets the full timeout since they race simultaneously

	// Try all URLs in parallel — use the first successful response
	type raceResult struct {
		raw []byte
		idx int
		err error
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	results := make(chan raceResult, n)
	for i := 0; i < n; i++ {
		idx := (start + i) % n
		go func(idx int) {
			raw, err := appsScriptRoundTrip(ctx, c.client, c.appScriptURLs[idx], c.frontDomain, string(payload), c.timeout)
			results <- raceResult{raw, idx, err}
		}(idx)
	}

	var raw []byte
	var lastErr error
	for i := 0; i < n; i++ {
		r := <-results
		if r.err == nil {
			if raw == nil {
				raw = r.raw
				cancel() // cancel losing goroutines
				if r.idx != start {
					activeURLIdx.Store(int64(r.idx))
				}
			}
		} else {
			// Only capture the error if we haven't succeeded yet and it's not a cancellation
			if raw == nil && !errors.Is(r.err, context.Canceled) {
				lastErr = r.err
			}
		}
	}
	if raw == nil && lastErr != nil {
		c.failAll(batch, lastErr)
		return
	}
	if raw == nil {
		c.failAll(batch, fmt.Errorf("relay failed: all workers returned without data"))
		return
	}

	var env2 batchResponseEnvelope
	if err := json.Unmarshal(raw, &env2); err != nil || len(env2.Items) != len(batch) {
		// Fallback: retry each individually.
		var wg sync.WaitGroup
		for _, item := range batch {
			wg.Add(1)
			go func(it *coalescerItem) {
				defer wg.Done()
				resp, err := RelayRequestMulti(c.client, c.appScriptURLs, c.frontDomain, c.authKey,
					it.method, it.targetURL, it.headers, it.body, c.timeout)
				it.result <- coalescerResult{resp, err}
			}(item)
		}
		wg.Wait()
		return
	}

	for i, wr := range env2.Items {
		if wr.Error != "" {
			batch[i].result <- coalescerResult{err: fmt.Errorf("relay error: %s", wr.Error)}
			continue
		}
		decoded, err := base64.StdEncoding.DecodeString(wr.Body)
		if err != nil {
			batch[i].result <- coalescerResult{err: fmt.Errorf("invalid base64: %w", err)}
			continue
		}
		batch[i].result <- coalescerResult{resp: RelayResponse{Status: wr.Status, Headers: normalizeHeaders(wr.Headers), Body: decoded}}
	}
}

func (c *Coalescer) failAll(batch []*coalescerItem, err error) {
	for _, item := range batch {
		item.result <- coalescerResult{err: err}
	}
}

func tryOneURL(ctx context.Context, client *http.Client, appScriptURL, frontDomain, payload string, timeout time.Duration) (RelayResponse, error) {
	raw, err := appsScriptRoundTrip(ctx, client, appScriptURL, frontDomain, payload, timeout)
	if err != nil {
		return RelayResponse{}, err
	}

	var workerResp workerResponse
	if err := json.Unmarshal(raw, &workerResp); err != nil {
		if strings.HasPrefix(strings.TrimSpace(string(raw)), "<") {
			return RelayResponse{}, fmt.Errorf("Apps Script returned HTML instead of JSON (quota/deploy issue): %s", previewBytes(raw, 512))
		}
		return RelayResponse{}, fmt.Errorf("invalid relay JSON: %w; body=%s", err, previewBytes(raw, 256))
	}
	if workerResp.Error != "" {
		return RelayResponse{}, fmt.Errorf("relay error: %s", workerResp.Error)
	}

	decoded, err := base64.StdEncoding.DecodeString(workerResp.Body)
	if err != nil {
		return RelayResponse{}, fmt.Errorf("invalid base64 body: %w", err)
	}

	return RelayResponse{Status: workerResp.Status, Headers: normalizeHeaders(workerResp.Headers), Body: decoded}, nil
}

func normalizeHeaders(raw map[string]any) map[string][]string {
	out := make(map[string][]string, len(raw))
	for k, v := range raw {
		lk := strings.ToLower(k)
		switch val := v.(type) {
		case string:
			out[lk] = []string{val}
		case []any:
			strs := make([]string, 0, len(val))
			for _, item := range val {
				if s, ok := item.(string); ok {
					strs = append(strs, s)
				}
			}
			out[lk] = strs
		case []string:
			out[lk] = val
		}
	}
	return out
}

// appsScriptRoundTrip posts payload to the fronted Apps Script URL, following one redirect if needed.
func appsScriptRoundTrip(ctx context.Context, client *http.Client, appScriptURL, frontDomain, payload string, timeout time.Duration) ([]byte, error) {
	noRedir := noRedirectClient(client)

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := newFrontedPOST(ctx, appScriptURL, frontDomain, payload)
	if err != nil {
		return nil, err
	}

	status, location, body, errStr := doHTTP(noRedir, req)
	if errStr != "" {
		return nil, fmt.Errorf("relay POST failed: %s", errStr)
	}

	if isRedirect(status) && location != "" {
		req2, err := newFrontedGET(ctx, frontDomain, location, appScriptURL)
		if err != nil {
			return nil, err
		}
		status, _, body, errStr = doHTTP(noRedir, req2)
		if errStr != "" {
			return nil, fmt.Errorf("relay redirect failed: %s", errStr)
		}
	}

	if status < 200 || status >= 500 {
		return nil, fmt.Errorf("relay returned %d: %s", status, previewBytes(body, 256))
	}
	return decompressRelayResponse(body), nil
}

// decompressRelayResponse unwraps {"z":"<base64-gzip>"} responses from Apps Script.
// Returns the original body unchanged if it is not in compressed envelope form.
func decompressRelayResponse(body []byte) []byte {
	var envelope struct {
		Z string `json:"z"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil || envelope.Z == "" {
		return body
	}
	compressed, err := base64.StdEncoding.DecodeString(envelope.Z)
	if err != nil {
		return body
	}
	r, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		return body
	}
	defer r.Close()
	out, err := io.ReadAll(io.LimitReader(r, maxRelayBody))
	if err != nil {
		return body
	}
	return out
}

func newFrontedPOST(ctx context.Context, appScriptURL, frontDomain, payload string) (*http.Request, error) {
	parsed, err := url.Parse(appScriptURL)
	if err != nil {
		return nil, err
	}
	if parsed.Scheme != "https" || parsed.Host == "" {
		return nil, fmt.Errorf("expected https Apps Script URL")
	}
	front := effectiveFrontDomain(frontDomain)

	fronted := *parsed
	fronted.Host = front

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fronted.String(), strings.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Host = parsed.Host
	req.Header.Set("Content-Type", "application/json")
	return req, nil
}

func newFrontedGET(ctx context.Context, frontDomain, location, baseURL string) (*http.Request, error) {
	loc, err := url.Parse(location)
	if err != nil {
		return nil, err
	}
	if loc.Scheme == "" || loc.Host == "" {
		base, err := url.Parse(baseURL)
		if err != nil {
			return nil, fmt.Errorf("invalid base url: %w", err)
		}
		loc = base.ResolveReference(loc)
	}

	originalHost := loc.Host
	front := effectiveFrontDomain(frontDomain)
	fronted := *loc
	fronted.Host = front

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fronted.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Host = originalHost
	return req, nil
}

func doHTTP(client *http.Client, req *http.Request) (status int, location string, body []byte, errStr string) {
	resp, err := client.Do(req)
	if err != nil {
		return 0, "", nil, compactErr(err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, maxRelayBody))
	return resp.StatusCode, resp.Header.Get("Location"), data, ""
}

func buildRelayPayload(authKey, method, targetURL string, headers map[string]string, body []byte) string {
	payload := map[string]any{
		"k":  authKey,
		"m":  strings.ToUpper(method),
		"u":  targetURL,
		"h":  headers,
		"r":  false,
		"gz": 1,
	}
	if len(body) > 0 {
		payload["b"] = base64.StdEncoding.EncodeToString(body)
	}
	if ct := headers["Content-Type"]; ct != "" {
		payload["ct"] = ct
	}
	out, err := json.Marshal(payload)
	if err != nil {
		return "{}"
	}
	return string(out)
}

func noRedirectClient(src *http.Client) *http.Client {
	c := *src
	c.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}
	return &c
}

func isRedirect(status int) bool {
	switch status {
	case http.StatusMovedPermanently, http.StatusFound, http.StatusSeeOther,
		http.StatusTemporaryRedirect, http.StatusPermanentRedirect:
		return true
	}
	return false
}

func effectiveFrontDomain(frontDomain string) string {
	if strings.TrimSpace(frontDomain) == "" {
		return "www.google.com"
	}
	return frontDomain
}

func previewBytes(b []byte, max int) string {
	s := strings.TrimSpace(string(b))
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

func compactErr(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	var urlErr *url.Error
	if errors.As(err, &urlErr) && urlErr.Err != nil {
		msg = urlErr.Err.Error()
	}
	return strings.ReplaceAll(msg, "\n", " ")
}
