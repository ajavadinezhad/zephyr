package main

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"embed"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"zyrln/relay/core"
)

const defaultProxyAddress = "direct"
const appVersion = "1.5.1-pre9"

//go:embed gui/*
var embeddedGUI embed.FS

type probe struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Category    string            `json:"category"`
	Method      string            `json:"method"`
	URL         string            `json:"url"`
	Host        string            `json:"host,omitempty"`
	FrontDomain string            `json:"front_domain,omitempty"`
	Expectation string            `json:"expectation"`
	Headers     map[string]string `json:"headers,omitempty"`
	Body        string            `json:"body,omitempty"`
}

type result struct {
	Probe      probe  `json:"probe"`
	Attempt    int    `json:"attempt"`
	OK         bool   `json:"ok"`
	Status     string `json:"status,omitempty"`
	StatusCode int    `json:"status_code,omitempty"`
	Proto      string `json:"proto,omitempty"`
	Location   string `json:"location,omitempty"`
	Remote     string `json:"remote,omitempty"`
	DurationMS int64  `json:"duration_ms"`
	Bytes      int64  `json:"bytes"`
	Preview    string `json:"preview,omitempty"`
	Error      string `json:"error,omitempty"`
}

type report struct {
	GeneratedAt string   `json:"generated_at"`
	Proxy       string   `json:"proxy"`
	Guard       string   `json:"guard"`
	TimeoutMS   int64    `json:"timeout_ms"`
	Repeat      int      `json:"repeat"`
	Results     []result `json:"results"`
	Summary     summary  `json:"summary"`
}

type summary struct {
	Total      int            `json:"total"`
	Reachable  int            `json:"reachable"`
	Failed     int            `json:"failed"`
	Categories map[string]int `json:"reachable_by_category"`
}

type desktopProfile struct {
	ID     string            `json:"id"`
	Name   string            `json:"name"`
	Config map[string]string `json:"config"`
}

type proxyConfig struct {
	label     string
	guard     string
	proxyFunc func(*http.Request) (*url.URL, error)
	proxyHost string
}

func parseProxyConfig(raw string) (proxyConfig, error) {
	value := strings.TrimSpace(raw)
	if value == "" || strings.EqualFold(value, "direct") || strings.EqualFold(value, "none") {
		return proxyConfig{
			label: "direct",
			guard: "direct dialing enabled for real in-country use",
		}, nil
	}

	proxyURL, err := url.Parse(value)
	if err != nil {
		return proxyConfig{}, err
	}
	if proxyURL.Scheme == "" || proxyURL.Host == "" {
		return proxyConfig{}, fmt.Errorf("expected proxy URL like http://host:port, or 'direct'")
	}

	proxyHost := proxyURL.Host
	if !strings.Contains(proxyHost, ":") {
		proxyHost = net.JoinHostPort(proxyHost, "80")
	}

	return proxyConfig{
		label:     proxyURL.String(),
		guard:     fmt.Sprintf("direct dialing disabled; only %s may be dialed", proxyHost),
		proxyFunc: http.ProxyURL(proxyURL),
		proxyHost: proxyHost,
	}, nil
}

func (p proxyConfig) dialContext(timeout time.Duration) func(context.Context, string, string) (net.Conn, error) {
	if p.proxyHost == "" {
		return (&net.Dialer{Timeout: timeout}).DialContext
	}
	return proxyOnlyDialer(p.proxyHost, timeout)
}

var (
	caCertFlag = flag.String("ca-cert", "certs/zyrln-ca.pem", "local CA certificate path for HTTPS proxy interception")
	caKeyFlag  = flag.String("ca-key", "certs/zyrln-ca-key.pem", "local CA private key path for HTTPS proxy interception")
)

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `Zyrln — domain-fronting reachability tool

Modes:
  (default)          run reachability probes and print a table
  -gen-key           generate a random auth key and print it
  -init-ca           generate a local CA cert for HTTPS proxy interception
  -serve-proxy       start local HTTP+HTTPS and SOCKS5 proxies backed by the relay
  -relay-fetch-url   fetch one URL through the full relay chain
  -google-direct-probe
                     probe Google Direct Mode fragmentation profiles
  -export-config     print config as JSON for importing into the Android app

Config: flags can be set in config.env (one key=value per line, flag name as key).

Flags:
`)
		flag.PrintDefaults()
	}

	configFlag := flag.String("config", "config.env", "path to config file (key=value, flag names as keys)")
	proxyFlag := flag.String("proxy", defaultProxyAddress, "HTTP proxy URL for lab testing, or 'direct'/'none' for real in-country use")
	timeoutFlag := flag.Duration("timeout", 30*time.Second, "per-probe timeout")
	repeatFlag := flag.Int("repeat", 1, "number of times to run each probe")
	formatFlag := flag.String("format", "table", "output format: table or json")
	outFlag := flag.String("out", "", "optional path to write the full JSON report")
	categoryFlag := flag.String("category", "", "optional comma-separated category filter")
	appScriptURLFlag := flag.String("appscript-url", "", "optional deployed Apps Script web app URL to probe with GET and POST")
	frontedAppScriptURLFlag := flag.String("fronted-appscript-url", "", "optional deployed Apps Script URL to probe using domain fronting")
	frontDomainFlag := flag.String("front-domain", "www.google.com", "front domain for domain-fronted probes")
	authKeyFlag := flag.String("auth-key", "", "auth key for the relay")
	targetURLFlag := flag.String("target-url", "https://www.gstatic.com/generate_204", "target URL for relay probe and relay-fetch")
	relayFetchURLFlag := flag.String("relay-fetch-url", "", "fetch this target URL through the full relay chain and print the decoded response")
	bodyOutFlag := flag.String("body-out", "", "optional path to write the decoded relay response body")
	serveProxyFlag := flag.Bool("serve-proxy", false, "start local HTTP and SOCKS5 proxies backed by the relay")
	listenFlag := flag.String("listen", "127.0.0.1:8085", "listen address for -serve-proxy")
	socksListenFlag := flag.String("socks-listen", "127.0.0.1:1080", "SOCKS5 listen address for -serve-proxy")
	noDirectFlag := flag.Bool("no-direct", false, "disable direct TLS-fragmentation bypass for Google domains (route all traffic through relay)")
	directEnabledFlag := flag.Bool("direct-enabled", true, "enable direct TLS-fragmentation bypass for Google domains (saved to config.env by GUI)")
	directProbeFlag := flag.Bool("google-direct-probe", false, "probe Google Direct Mode fragmentation profiles")
	exportConfigFlag := flag.Bool("export-config", false, "print config as JSON for importing into the Android app")
	initCAFlag := flag.Bool("init-ca", false, "generate a local CA certificate for HTTPS proxy interception")
	genKeyFlag := flag.Bool("gen-key", false, "generate a random 32-byte base64 auth key and print it")
	frontRedirectsFlag := flag.Bool("front-redirects", false, "when a fronted probe gets a redirect, retry the Location using the front domain and encrypted Host override")
	followRedirectsFlag := flag.Bool("follow-redirects", true, "follow HTTP redirects")
	guiFlag := flag.Bool("gui", false, "start the browser-based GUI")
	guiListenFlag := flag.String("gui-listen", "127.0.0.1:8086", "listen address for the GUI")
	flag.Parse()

	if shouldStartGUIByDefault(runtime.GOOS, os.Args) {
		*guiFlag = true
	}

	// Apply config file values for flags not set on the CLI.
	setCLI := map[string]bool{}
	flag.Visit(func(f *flag.Flag) { setCLI[f.Name] = true })
	for key, value := range loadConfig(*configFlag) {
		if !setCLI[key] {
			_ = flag.Set(key, value)
		}
	}

	if *repeatFlag < 1 {
		fmt.Fprintln(os.Stderr, "repeat must be at least 1")
		os.Exit(1)
	}

	core.SetDirectEnabled(*directEnabledFlag)
	if *noDirectFlag {
		core.SetDirectEnabled(false)
	}
	core.SetCacheDir(filepath.Dir(*configFlag))

	if *guiFlag {
		startGUIServer(*guiListenFlag, *configFlag, *caCertFlag, *caKeyFlag)
		return
	}

	if *exportConfigFlag {
		rawURL := strings.TrimSpace(*frontedAppScriptURLFlag)
		key := strings.TrimSpace(*authKeyFlag)
		if rawURL == "" || key == "" {
			fmt.Fprintln(os.Stderr, "-export-config requires -fronted-appscript-url and -auth-key (or config.env)")
			os.Exit(1)
		}
		out, _ := json.Marshal(map[string]string{"url": rawURL, "key": key})
		fmt.Println(string(out))
		return
	}

	if *genKeyFlag {
		key := make([]byte, 32)
		if _, err := rand.Read(key); err != nil {
			fmt.Fprintf(os.Stderr, "failed to generate key: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(base64.StdEncoding.EncodeToString(key))
		return
	}

	if *initCAFlag {
		if err := core.GenerateCA(*caCertFlag, *caKeyFlag); err != nil {
			fmt.Fprintf(os.Stderr, "failed to generate CA: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("generated CA certificate: %s\n", *caCertFlag)
		fmt.Printf("generated CA private key: %s\n", *caKeyFlag)
		fmt.Printf("install the certificate, not the key, as a trusted CA on the test device\n")
		return
	}

	proxyCfg, err := parseProxyConfig(*proxyFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid proxy: %v\n", err)
		os.Exit(1)
	}

	client := &http.Client{
		Timeout: *timeoutFlag,
		Transport: &http.Transport{
			Proxy:           proxyCfg.proxyFunc,
			TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12},
			DialContext:     proxyCfg.dialContext(*timeoutFlag),
		},
	}
	if !*followRedirectsFlag {
		client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		}
	}

	appScriptURLs := parseURLList(strings.TrimSpace(*frontedAppScriptURLFlag))

	if strings.TrimSpace(*relayFetchURLFlag) != "" {
		if len(appScriptURLs) == 0 {
			fmt.Fprintln(os.Stderr, "-relay-fetch-url requires -fronted-appscript-url")
			os.Exit(1)
		}
		if strings.TrimSpace(*authKeyFlag) == "" {
			fmt.Fprintln(os.Stderr, "-relay-fetch-url requires -auth-key")
			os.Exit(1)
		}
		if err := relayFetch(client, appScriptURLs, *frontDomainFlag, *authKeyFlag, *relayFetchURLFlag, *bodyOutFlag, *timeoutFlag); err != nil {
			fmt.Fprintf(os.Stderr, "relay fetch failed: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if *directProbeFlag {
		targets := []string{"www.googleapis.com"}
		fronts := core.DirectFronts
		total := len(targets) * len(fronts) * len(core.DirectProfiles()) * *repeatFlag
		if total < 1 {
			total = 1
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(total+1)**timeoutFlag)
		defer cancel()
		rep := core.ProbeDirectProfiles(ctx, targets, fronts, *repeatFlag, *timeoutFlag)
		printDirectAdaptiveReport(rep, *repeatFlag, *timeoutFlag)
		return
	}

	if *serveProxyFlag {
		if len(appScriptURLs) == 0 {
			fmt.Fprintln(os.Stderr, "-serve-proxy requires -fronted-appscript-url")
			os.Exit(1)
		}
		if strings.TrimSpace(*authKeyFlag) == "" {
			fmt.Fprintln(os.Stderr, "-serve-proxy requires -auth-key")
			os.Exit(1)
		}
		ca, err := core.LoadCA(*caCertFlag, *caKeyFlag)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to load CA: %v\nrun -init-ca first\n", err)
			os.Exit(1)
		}
		fmt.Printf("relay HTTP proxy listening on http://%s\n", *listenFlag)
		fmt.Printf("relay SOCKS5 proxy listening on socks5://%s\n", *socksListenFlag)
		fmt.Printf("mode: HTTP and HTTPS via local CA MITM; install %s as trusted CA for browsers\n", *caCertFlag)
		if len(appScriptURLs) > 1 {
			fmt.Printf("fallback: %d Apps Script URLs configured\n", len(appScriptURLs))
		}
		if err := core.ServeProxyWithSOCKS(*listenFlag, *socksListenFlag, appScriptURLs, *frontDomainFlag, *authKeyFlag, ca, client, *timeoutFlag); err != nil {
			fmt.Fprintf(os.Stderr, "proxy failed: %v\n", err)
			os.Exit(1)
		}
		return
	}

	probes := filterProbes(defaultProbes(), *categoryFlag)
	if strings.TrimSpace(*appScriptURLFlag) != "" {
		probes = append(probes, appScriptProbes(strings.TrimSpace(*appScriptURLFlag))...)
	}
	if len(appScriptURLs) > 0 {
		fp, err := frontedAppScriptProbes(
			appScriptURLs[0],
			strings.TrimSpace(*frontDomainFlag),
			strings.TrimSpace(*authKeyFlag),
			strings.TrimSpace(*targetURLFlag),
		)
		if err != nil {
			fmt.Fprintf(os.Stderr, "invalid fronted Apps Script URL: %v\n", err)
			os.Exit(1)
		}
		probes = append(probes, fp...)
	}
	if len(probes) == 0 {
		fmt.Fprintln(os.Stderr, "no probes selected")
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "running %d probe(s)...\n", len(probes)**repeatFlag)
	results := make([]result, 0, len(probes)**repeatFlag)
	for attempt := 1; attempt <= *repeatFlag; attempt++ {
		for _, p := range probes {
			results = append(results, runProbe(client, p, attempt, *timeoutFlag, *frontRedirectsFlag))
		}
	}

	rep := report{
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Proxy:       proxyCfg.label,
		Guard:       proxyCfg.guard,
		TimeoutMS:   timeoutFlag.Milliseconds(),
		Repeat:      *repeatFlag,
		Results:     results,
		Summary:     summarize(results),
	}

	if *outFlag != "" {
		if err := writeJSONReport(*outFlag, rep); err != nil {
			fmt.Fprintf(os.Stderr, "failed to write report: %v\n", err)
			os.Exit(1)
		}
	}

	switch strings.ToLower(*formatFlag) {
	case "table":
		printTable(rep)
	case "json":
		if err := json.NewEncoder(os.Stdout).Encode(rep); err != nil {
			fmt.Fprintf(os.Stderr, "failed to encode JSON: %v\n", err)
			os.Exit(1)
		}
	default:
		fmt.Fprintf(os.Stderr, "unknown format %q; use table or json\n", *formatFlag)
		os.Exit(1)
	}
}

func parseURLList(raw string) []string {
	return core.ParseURLList(raw)
}

func printDirectAdaptiveReport(rep core.DirectProbeReport, repeat int, timeout time.Duration) {
	fmt.Printf("ZPROBE adaptive (direct) fa-IR field test\n")
	fmt.Printf("targets=%d fronts=%d profiles=%d repeat=%d timeout=%s\n",
		len(rep.Targets), len(rep.Fronts), len(core.DirectProfiles()), repeat, timeout)
	fmt.Printf("profile details redacted; exact local result stays on-device\n\n")

	for _, target := range rep.Targets {
		tried := 0
		fmt.Printf("target %s\n", strings.TrimSuffix(target, ":443"))
		for _, r := range rep.Results {
			if r.Target != target {
				continue
			}
			tried++
			if r.OK {
				fmt.Printf("  ok   front=%-20s profile=%s stable=%d/%d avg=%.1f ms\n",
					r.Front, r.ProfileID, r.Stable, r.Repeat, ms(r.Avg))
			} else {
				reason := r.Reason
				if reason == "" {
					reason = "error"
				}
				fmt.Printf("  fail front=%-20s profile=%s stable=%d/%d reason=%s\n",
					r.Front, r.ProfileID, r.Stable, r.Repeat, reason)
			}
		}
		if best, ok := rep.Best[target]; ok {
			fmt.Printf("  BEST target=%s front=%s profile=%s avg=%.1f ms tried=%d\n",
				strings.TrimSuffix(best.Target, ":443"), best.Front, best.ProfileID, ms(best.Avg), tried)
		} else {
			fmt.Printf("  BEST target=%s none tried=%d\n", strings.TrimSuffix(target, ":443"), tried)
		}
		fmt.Println()
	}
}

func ms(d time.Duration) float64 {
	return float64(d.Microseconds()) / 1000
}

func relayFetch(client *http.Client, appScriptURLs []string, frontDomain, authKey, targetURL, bodyOut string, timeout time.Duration) error {
	resp, err := core.RelayRequestMulti(client, appScriptURLs, frontDomain, authKey, "GET", targetURL, map[string]string{"User-Agent": "zyrln/0.1"}, nil, timeout)
	if err != nil {
		return err
	}
	if bodyOut != "" {
		if err := writeBody(bodyOut, resp.Body); err != nil {
			return err
		}
	}
	fmt.Printf("relay fetch ok\ntarget: %s\nstatus: %d\nheaders: %d\nbody bytes: %d\n", targetURL, resp.Status, len(resp.Headers), len(resp.Body))
	if bodyOut != "" {
		fmt.Printf("body written: %s\n", bodyOut)
	}
	if len(resp.Body) > 0 {
		fmt.Printf("preview: %s\n", preview(resp.Body, 1200))
	}
	return nil
}

func defaultProbes() []probe {
	return []probe{
		{ID: "google-home", Name: "Google search edge", Category: "baseline", Method: http.MethodHead, URL: "https://www.google.com/", Expectation: "baseline HTTPS reachability"},
		{ID: "android-204", Name: "Android connectivity check", Category: "baseline", Method: http.MethodGet, URL: "https://clients3.google.com/generate_204", Expectation: "small Google HTTPS response used by Android captive-portal checks"},
		{ID: "gstatic-204", Name: "Gstatic static edge", Category: "baseline", Method: http.MethodGet, URL: "https://www.gstatic.com/generate_204", Expectation: "Google static/CDN hostname"},
		{ID: "googleapis-discovery", Name: "Google APIs root", Category: "api", Method: http.MethodGet, URL: "https://www.googleapis.com/discovery/v1/apis", Expectation: "Google API surface without app-specific backend"},
		{ID: "google-doh", Name: "Google DoH JSON", Category: "api", Method: http.MethodGet, URL: "https://dns.google/resolve?name=google.com&type=A", Expectation: "DNS-over-HTTPS reachability through Google"},
		{ID: "apps-script", Name: "Apps Script", Category: "serverless", Method: http.MethodHead, URL: "https://script.google.com/", Expectation: "possible serverless web-app front door"},
		{ID: "apps-script-content", Name: "Apps Script content host", Category: "serverless", Method: http.MethodHead, URL: "https://script.googleusercontent.com/", Expectation: "Apps Script web apps often redirect here for execution output"},
		{ID: "firebase-hosting", Name: "Firebase hosting", Category: "serverless", Method: http.MethodHead, URL: "https://firebase.google.com/", Expectation: "Firebase-hosted HTTPS surface"},
		{ID: "cloud-run-api", Name: "Cloud Run API", Category: "serverless", Method: http.MethodGet, URL: "https://run.googleapis.com/", Expectation: "Cloud Run control/API hostname reachability"},
		{ID: "cloud-functions-api", Name: "Cloud Functions API", Category: "serverless", Method: http.MethodGet, URL: "https://cloudfunctions.googleapis.com/", Expectation: "Cloud Functions API hostname reachability"},
		{ID: "storage-api", Name: "Google storage API", Category: "serverless", Method: http.MethodGet, URL: "https://storage.googleapis.com/", Expectation: "Google Cloud Storage public edge"},
		{
			ID: "websocket-shape", Name: "WebSocket upgrade shape", Category: "transport",
			Method: http.MethodGet, URL: "https://www.google.com/",
			Headers: map[string]string{
				"Connection": "Upgrade", "Upgrade": "websocket",
				"Sec-WebSocket-Key": "dGhlIHNhbXBsZSBub25jZQ==", "Sec-WebSocket-Version": "13",
			},
			Expectation: "checks whether upgrade-shaped HTTPS reaches the edge; 101 is not expected from Google",
		},
	}
}

func appScriptProbes(rawURL string) []probe {
	return []probe{
		{ID: "appscript-deployed-get", Name: "Apps Script deployed GET", Category: "serverless-live", Method: http.MethodGet, URL: addQuery(rawURL, "mode=probe&size=small"), Expectation: "deployed Apps Script web app accepts small GET messages"},
		{ID: "appscript-deployed-post", Name: "Apps Script deployed POST", Category: "serverless-live", Method: http.MethodPost, URL: rawURL, Headers: map[string]string{"Content-Type": "application/json"}, Body: `{"mode":"probe","size":"small","message":"zyrln probe"}`, Expectation: "deployed Apps Script web app accepts small POST messages"},
	}
}

func frontedAppScriptProbes(rawURL, frontDomain, authKey, targetURL string) ([]probe, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	if parsed.Scheme != "https" {
		return nil, fmt.Errorf("expected https URL, got %q", parsed.Scheme)
	}
	if parsed.Host == "" {
		return nil, fmt.Errorf("missing host")
	}
	if frontDomain == "" {
		frontDomain = "www.google.com"
	}

	frontedBase := *parsed
	frontedBase.Host = frontDomain

	probes := []probe{
		{ID: "fronted-appscript-get", Name: "Fronted Apps Script GET", Category: "domain-front", Method: http.MethodGet, URL: addQuery(frontedBase.String(), "mode=probe&size=small"), Host: parsed.Host, FrontDomain: frontDomain, Expectation: "domain-fronted GET"},
		{ID: "fronted-appscript-post", Name: "Fronted Apps Script POST", Category: "domain-front", Method: http.MethodPost, URL: frontedBase.String(), Host: parsed.Host, FrontDomain: frontDomain, Headers: map[string]string{"Content-Type": "application/json"}, Body: `{"mode":"probe","size":"small","message":"zyrln domain-front probe"}`, Expectation: "domain-fronted POST"},
	}

	if strings.TrimSpace(authKey) != "" {
		payload := map[string]any{
			"k": authKey, "m": "GET", "u": targetURL,
			"h":  map[string]string{"User-Agent": "zyrln/0.1"},
			"ct": nil, "r": true,
		}
		encoded, _ := json.Marshal(payload)
		probes = append(probes, probe{
			ID: "fronted-relay-post", Name: "Fronted relay POST", Category: "domain-front",
			Method: http.MethodPost, URL: frontedBase.String(), Host: parsed.Host, FrontDomain: frontDomain,
			Headers: map[string]string{"Content-Type": "application/json"}, Body: string(encoded),
			Expectation: "relay payload through fronted Apps Script",
		})
	}

	return probes, nil
}

func filterProbes(probes []probe, categoryCSV string) []probe {
	if strings.TrimSpace(categoryCSV) == "" {
		return probes
	}
	allowed := map[string]bool{}
	for _, raw := range strings.Split(categoryCSV, ",") {
		if c := strings.ToLower(strings.TrimSpace(raw)); c != "" {
			allowed[c] = true
		}
	}
	filtered := make([]probe, 0, len(probes))
	for _, p := range probes {
		if allowed[strings.ToLower(p.Category)] {
			filtered = append(filtered, p)
		}
	}
	return filtered
}

func proxyOnlyDialer(proxyHost string, timeout time.Duration) func(context.Context, string, string) (net.Conn, error) {
	dialer := &net.Dialer{Timeout: timeout}
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		if address != proxyHost {
			return nil, fmt.Errorf("blocked direct dial to %s; only proxy %s is allowed", address, proxyHost)
		}
		return dialer.DialContext(ctx, network, address)
	}
}

func runProbe(client *http.Client, p probe, attempt int, timeout time.Duration, frontRedirects bool) result {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return runProbeCtx(ctx, client, p, attempt, frontRedirects)
}

func runProbeCtx(ctx context.Context, client *http.Client, p probe, attempt int, frontRedirects bool) result {
	res := executeProbe(ctx, client, p, attempt)

	if frontRedirects && p.FrontDomain != "" && isRedirect(res.StatusCode) && res.Location != "" {
		next, err := frontedRedirectProbe(p, res.Location)
		if err != nil {
			res.Error = "front redirect build failed: " + err.Error()
			return res
		}
		nextRes := runProbeCtx(ctx, client, next, attempt, false)
		nextRes.Probe.ID = p.ID + "-front-redirect"
		nextRes.Probe.Name = p.Name + " redirect"
		nextRes.Probe.Expectation = p.Expectation + " redirected with fronting"
		return nextRes
	}
	return res
}

func executeProbe(ctx context.Context, client *http.Client, p probe, attempt int) result {
	var remote string
	trace := &httptrace.ClientTrace{
		GotConn: func(info httptrace.GotConnInfo) {
			if info.Conn != nil {
				remote = info.Conn.RemoteAddr().String()
			}
		},
	}

	var body io.Reader
	if p.Body != "" {
		body = strings.NewReader(p.Body)
	}

	req, err := http.NewRequestWithContext(httptrace.WithClientTrace(ctx, trace), p.Method, p.URL, body)
	if err != nil {
		return result{Probe: p, Attempt: attempt, Error: err.Error()}
	}
	for k, v := range p.Headers {
		req.Header.Set(k, v)
	}
	if p.Host != "" {
		req.Host = p.Host
	}

	start := time.Now()
	resp, err := client.Do(req)
	elapsed := time.Since(start).Round(time.Millisecond)
	if err != nil {
		return result{Probe: p, Attempt: attempt, DurationMS: elapsed.Milliseconds(), Remote: remote, Error: compactError(err)}
	}
	defer resp.Body.Close()

	limited, _ := io.ReadAll(io.LimitReader(resp.Body, 16*1024*1024))
	return result{
		Probe:      p,
		Attempt:    attempt,
		OK:         resp.StatusCode >= 200 && resp.StatusCode < 500,
		Status:     resp.Status,
		StatusCode: resp.StatusCode,
		Proto:      resp.Proto,
		Location:   resp.Header.Get("Location"),
		Remote:     remote,
		DurationMS: elapsed.Milliseconds(),
		Bytes:      int64(len(limited)),
		Preview:    preview(limited, 512),
	}
}

func frontedRedirectProbe(original probe, location string) (probe, error) {
	redirectURL, err := url.Parse(location)
	if err != nil {
		return probe{}, err
	}
	if redirectURL.Scheme == "" || redirectURL.Host == "" {
		base, err := url.Parse(original.URL)
		if err != nil {
			return probe{}, err
		}
		redirectURL = base.ResolveReference(redirectURL)
	}

	frontedURL := *redirectURL
	frontedURL.Host = original.FrontDomain

	return probe{
		ID: original.ID + "-front-redirect", Name: original.Name + " redirect",
		Category: original.Category, Method: http.MethodGet,
		URL: frontedURL.String(), Host: redirectURL.Host, FrontDomain: original.FrontDomain,
		Expectation: "fronted follow-up to " + location,
	}, nil
}

func isRedirect(statusCode int) bool {
	switch statusCode {
	case http.StatusMovedPermanently, http.StatusFound, http.StatusSeeOther,
		http.StatusTemporaryRedirect, http.StatusPermanentRedirect:
		return true
	}
	return false
}

func addQuery(rawURL, query string) string {
	if strings.Contains(rawURL, "?") {
		return rawURL + "&" + query
	}
	return rawURL + "?" + query
}

func summarize(results []result) summary {
	s := summary{Total: len(results), Categories: map[string]int{}}
	for _, r := range results {
		if r.OK {
			s.Reachable++
			s.Categories[r.Probe.Category]++
		} else {
			s.Failed++
		}
	}
	return s
}

func writeJSONReport(path string, rep report) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	enc := json.NewEncoder(file)
	enc.SetIndent("", "  ")
	return enc.Encode(rep)
}

func writeBody(path string, body []byte) error {
	if dir := filepath.Dir(path); dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}
	return os.WriteFile(path, body, 0644)
}

func printTable(rep report) {
	fmt.Printf("proxy: %s\nguard: %s\ngenerated: %s\nsummary: %d reachable, %d failed, %d total\n\n",
		rep.Proxy, rep.Guard, rep.GeneratedAt, rep.Summary.Reachable, rep.Summary.Failed, rep.Summary.Total)

	fmt.Printf("%-4s %-24s %-10s %-5s %-12s %-8s %-8s %s\n", "TRY", "PROBE", "CATEGORY", "OK", "STATUS", "PROTO", "TIME", "REMOTE/ERROR")
	fmt.Printf("%s\n", strings.Repeat("-", 120))

	for _, r := range rep.Results {
		ok := "no"
		if r.OK {
			ok = "yes"
		}
		status := r.Status
		if status == "" {
			status = "-"
		}
		proto := r.Proto
		if proto == "" {
			proto = "-"
		}
		remoteOrError := r.Remote
		if r.Error != "" {
			remoteOrError = r.Error
		} else if r.Location != "" {
			remoteOrError = "redirect " + r.Location
		}
		fmt.Printf("%-4d %-24s %-10s %-5s %-12s %-8s %-8s %s\n",
			r.Attempt, truncate(r.Probe.Name, 24), truncate(r.Probe.Category, 10),
			ok, truncate(status, 12), proto, fmt.Sprintf("%dms", r.DurationMS), remoteOrError)
	}

	fmt.Printf("\nNotes:\n")
	fmt.Printf("- Any HTTP status below 500 counts as reachable.\n")
	fmt.Printf("- HTTP/3/QUIC is not tested here (TCP-based path only).\n")
	fmt.Printf("- WebSocket support needs a real WebSocket backend.\n")
}

func preview(body []byte, max int) string {
	if len(body) == 0 {
		return ""
	}
	value := strings.TrimSpace(string(body))
	value = strings.ReplaceAll(value, "\n", " ")
	value = strings.ReplaceAll(value, "\r", " ")
	if len(value) <= max {
		return value
	}
	return value[:max-3] + "..."
}

func compactError(err error) string {
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

func truncate(value string, max int) string {
	if len(value) <= max {
		return value
	}
	if max <= 3 {
		return value[:max]
	}
	return value[:max-3] + "..."
}

func loadConfig(path string) map[string]string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	values := map[string]string{}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		if key != "" {
			values[key] = value
		}
	}
	return values
}

func saveConfig(path string, values map[string]string) error {
	keys := make([]string, 0, len(values))
	for k := range values {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	lines := make([]string, 0, len(keys))
	for _, k := range keys {
		if strings.TrimSpace(k) == "" {
			continue
		}
		lines = append(lines, fmt.Sprintf("%s = %s", k, strings.TrimSpace(values[k])))
	}
	return os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0644)
}

func profilesPath(configPath string) string {
	return configPath + ".profiles.json"
}

func loadProfiles(configPath string) []desktopProfile {
	data, err := os.ReadFile(profilesPath(configPath))
	if err != nil {
		return []desktopProfile{}
	}
	var profiles []desktopProfile
	if err := json.Unmarshal(data, &profiles); err != nil {
		return []desktopProfile{}
	}
	return profiles
}

func saveProfiles(configPath string, profiles []desktopProfile) error {
	if profiles == nil {
		profiles = []desktopProfile{}
	}
	data, err := json.MarshalIndent(profiles, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(profilesPath(configPath), append(data, '\n'), 0644)
}

var globalDesktopConfigKeys = map[string]bool{
	"listen":       true,
	"socks-listen": true,
}

func profileConfig(values map[string]string) map[string]string {
	cfg := map[string]string{}
	for k, v := range values {
		key := strings.TrimSpace(k)
		if key == "" || globalDesktopConfigKeys[key] {
			continue
		}
		cfg[key] = strings.TrimSpace(v)
	}
	return cfg
}

func applyProfileConfig(base, profile map[string]string) map[string]string {
	merged := map[string]string{}
	for k, v := range base {
		merged[k] = v
	}
	for k, v := range profileConfig(profile) {
		merged[k] = v
	}
	return merged
}

func profileDisplayName(name string, cfg map[string]string) string {
	name = strings.TrimSpace(name)
	if name != "" {
		return name
	}
	rawURL := strings.TrimSpace(cfg["fronted-appscript-url"])
	if rawURL == "" {
		return "Profile"
	}
	first := strings.TrimSpace(strings.Split(rawURL, ",")[0])
	u, err := url.Parse(first)
	if err == nil && u.Hostname() != "" {
		return u.Hostname()
	}
	return "Profile"
}

func shouldStartGUIByDefault(goos string, args []string) bool {
	return goos == "windows" && len(args) <= 1
}

const maxGUILogEntries = 500

type guiLogEntry struct {
	Level string
	Msg   string
}

var (
	guiProxyServer    *http.Server
	guiProxyLn        net.Listener
	guiSOCKSServer    *core.SOCKSServer
	guiSOCKSLn        net.Listener
	guiCoalescer      *core.Coalescer
	guiRequestCount   int64
	guiProxyStartTime time.Time
	guiMu             sync.Mutex

	guiLogMu  sync.Mutex
	guiLogBuf []guiLogEntry
	guiLogSeq int
)

func guiEmitLog(level, msg string) {
	guiLogMu.Lock()
	guiLogBuf = append(guiLogBuf, guiLogEntry{Level: level, Msg: msg})
	if len(guiLogBuf) > maxGUILogEntries {
		guiLogBuf = guiLogBuf[1:]
	}
	guiLogSeq++
	guiLogMu.Unlock()
}

type guiProxyStarter func(listen, socksListen string, urls []string, key string, ca *core.CertAuthority) (*http.Server, net.Listener, *core.SOCKSServer, net.Listener, error)

func defaultGUIProxyStarter(listen, socksListen string, urls []string, key string, ca *core.CertAuthority) (*http.Server, net.Listener, *core.SOCKSServer, net.Listener, error) {
	client := core.NewHTTPClient(30 * time.Second)
	srv, ln, socksSrv, socksLn, coal, err := core.StartProxyWithSOCKSAndCoalescer(listen, socksListen, urls, "www.google.com", key, ca, client, 30*time.Second)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	// Note: guiMu is held by the caller — store coalescer directly without re-locking.
	guiCoalescer = coal
	return srv, ln, socksSrv, socksLn, nil
}

func startGUIServer(listenAddr, configPath, caCertPath, caKeyPath string) {
	fmt.Printf("Starting Zyrln GUI at http://%s\n", listenAddr)

	// Automatically open the browser
	go func() {
		time.Sleep(500 * time.Millisecond)
		openBrowser("http://" + listenAddr)
	}()

	core.OnRequest = func(method, url string) {
		atomic.AddInt64(&guiRequestCount, 1)
	}
	core.SetLogFunc(func(level, msg string) {
		guiEmitLog(level, msg)
	})

	handler := newGUIHandler(configPath, caCertPath, caKeyPath, defaultGUIProxyStarter, openPath)
	if err := http.ListenAndServe(listenAddr, handler); err != nil {
		fmt.Fprintf(os.Stderr, "GUI server failed: %v\n", err)
		os.Exit(1)
	}
}

func newGUIHandler(configPath, caCertPath, caKeyPath string, startProxy guiProxyStarter, open func(string)) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/api/status", func(w http.ResponseWriter, r *http.Request) {
		guiMu.Lock()
		running := guiProxyServer != nil
		started := guiProxyStartTime
		guiMu.Unlock()

		uptime := "00:00:00"
		if !started.IsZero() {
			d := time.Since(started).Round(time.Second)
			h := d / time.Hour
			d -= h * time.Hour
			m := d / time.Minute
			d -= m * time.Minute
			s := d / time.Second
			uptime = fmt.Sprintf("%02d:%02d:%02d", h, m, s)
		}
		json.NewEncoder(w).Encode(map[string]any{
			"running":  running,
			"uptime":   uptime,
			"requests": atomic.LoadInt64(&guiRequestCount),
			"version":  appVersion,
			"os":       runtime.GOOS,
			"arch":     runtime.GOARCH,
		})
	})

	mux.HandleFunc("/api/config", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			var cfg map[string]string
			if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			if err := saveConfig(configPath, cfg); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusOK)
			return
		}
		json.NewEncoder(w).Encode(loadConfig(configPath))
	})

	mux.HandleFunc("/api/profiles", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			json.NewEncoder(w).Encode(loadProfiles(configPath))
		case http.MethodPost:
			var req struct {
				ID     string            `json:"id"`
				Name   string            `json:"name"`
				Config map[string]string `json:"config"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			if req.Config == nil {
				req.Config = map[string]string{}
			}
			req.Config = profileConfig(req.Config)
			profiles := loadProfiles(configPath)
			id := strings.TrimSpace(req.ID)
			if id == "__direct__" {
				http.Error(w, "reserved profile id", http.StatusBadRequest)
				return
			}
			if id == "" {
				id = fmt.Sprintf("profile-%d", time.Now().UnixNano())
			}
			profile := desktopProfile{
				ID:     id,
				Name:   profileDisplayName(req.Name, req.Config),
				Config: req.Config,
			}
			replaced := false
			for i := range profiles {
				if profiles[i].ID == id {
					profiles[i] = profile
					replaced = true
					break
				}
			}
			if !replaced {
				profiles = append(profiles, profile)
			}
			if err := saveProfiles(configPath, profiles); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(profile)
		case http.MethodDelete:
			id := strings.TrimSpace(r.URL.Query().Get("id"))
			if id == "" {
				http.Error(w, "id is required", http.StatusBadRequest)
				return
			}
			if id == "__direct__" {
				http.Error(w, "cannot delete built-in profile", http.StatusBadRequest)
				return
			}
			profiles := loadProfiles(configPath)
			filtered := profiles[:0]
			for _, p := range profiles {
				if p.ID != id {
					filtered = append(filtered, p)
				}
			}
			if err := saveProfiles(configPath, filtered); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusOK)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/profiles/activate", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		guiMu.Lock()
		running := guiProxyServer != nil
		guiMu.Unlock()
		if running {
			http.Error(w, "stop proxy before switching profiles", http.StatusConflict)
			return
		}
		var req struct {
			ID string `json:"id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		for _, p := range loadProfiles(configPath) {
			if p.ID == strings.TrimSpace(req.ID) {
				cfg := applyProfileConfig(loadConfig(configPath), p.Config)
				if err := saveConfig(configPath, cfg); err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
				w.WriteHeader(http.StatusOK)
				return
			}
		}
		http.Error(w, "profile not found", http.StatusNotFound)
	})

	mux.HandleFunc("/api/init-ca", func(w http.ResponseWriter, r *http.Request) {
		// Stop proxy if running — old CA in memory would cause signature mismatch
		guiMu.Lock()
		if guiProxyServer != nil {
			if guiCoalescer != nil {
				guiCoalescer.Stop()
				guiCoalescer = nil
			}
			_ = guiProxyLn.Close()
			_ = guiProxyServer.Close()
			if guiSOCKSLn != nil {
				_ = guiSOCKSLn.Close()
			}
			guiProxyServer = nil
			guiProxyLn = nil
			guiSOCKSServer = nil
			guiSOCKSLn = nil
			guiProxyStartTime = time.Time{}
		}
		guiMu.Unlock()

		if err := core.GenerateCA(caCertPath, caKeyPath); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		ca, _ := core.LoadCA(caCertPath, caKeyPath)
		serial := "unknown"
		if ca != nil && ca.GetCertificate() != nil {
			serial = ca.GetCertificate().SerialNumber.String()
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"status":  "ok",
			"serial":  serial,
			"message": "CA regenerated successfully.",
		})
	})

	mux.HandleFunc("/api/start", func(w http.ResponseWriter, r *http.Request) {
		guiMu.Lock()
		if guiProxyServer != nil {
			guiMu.Unlock()
			http.Error(w, "already running", http.StatusConflict)
			return
		}
		defer guiMu.Unlock()

		var startReq struct {
			DirectOnly bool `json:"directOnly"`
		}
		if r.Body != nil {
			json.NewDecoder(r.Body).Decode(&startReq)
		}

		cfg := loadConfig(configPath)
		urls := parseURLList(cfg["fronted-appscript-url"])
		key := cfg["auth-key"]
		listen := cfg["listen"]
		if listen == "" {
			listen = "127.0.0.1:8085"
		}
		socksListen := cfg["socks-listen"]
		if socksListen == "" {
			socksListen = "127.0.0.1:1080"
		}
		directOnly := startReq.DirectOnly || (len(urls) == 0 && core.GetDirectEnabled())
		if directOnly {
			urls = nil
			key = ""
		}
		if !directOnly && len(urls) == 0 {
			http.Error(w, "fronted-appscript-url is required", http.StatusBadRequest)
			return
		}
		if !directOnly && strings.TrimSpace(key) == "" {
			http.Error(w, "auth-key is required", http.StatusBadRequest)
			return
		}

		var ca *core.CertAuthority
		if !directOnly {
			var err error
			ca, err = core.LoadCA(caCertPath, caKeyPath)
			if err != nil {
				http.Error(w, "CA certificate missing or invalid. Install certificate first.", http.StatusBadRequest)
				return
			}
		}

		srv, ln, socksSrv, socksLn, err := startProxy(listen, socksListen, urls, key, ca)
		if err != nil {
			http.Error(w, "start failed: "+err.Error(), http.StatusInternalServerError)
			return
		}

		guiLogMu.Lock()
		guiLogBuf = guiLogBuf[:0]
		guiLogSeq = 0
		guiLogMu.Unlock()

		guiProxyServer = srv
		guiProxyLn = ln
		guiSOCKSServer = socksSrv
		guiSOCKSLn = socksLn
		guiProxyStartTime = time.Now()
		w.WriteHeader(http.StatusOK)
	})

	mux.HandleFunc("/api/stop", func(w http.ResponseWriter, r *http.Request) {
		guiMu.Lock()
		if guiProxyServer == nil {
			guiMu.Unlock()
			w.WriteHeader(http.StatusOK)
			return
		}
		if guiCoalescer != nil {
			guiCoalescer.Stop()
			guiCoalescer = nil
		}
		_ = guiProxyLn.Close()
		_ = guiProxyServer.Close()
		if guiSOCKSLn != nil {
			_ = guiSOCKSLn.Close()
		}
		guiProxyServer = nil
		guiProxyLn = nil
		guiSOCKSServer = nil
		guiSOCKSLn = nil
		guiProxyStartTime = time.Time{}
		guiMu.Unlock()
		w.WriteHeader(http.StatusOK)
	})

	mux.HandleFunc("/api/logs", func(w http.ResponseWriter, r *http.Request) {
		clientSeq := 0
		if s := r.URL.Query().Get("seq"); s != "" {
			fmt.Sscanf(s, "%d", &clientSeq)
			if clientSeq < 0 {
				clientSeq = 0
			}
		}
		guiLogMu.Lock()
		total := len(guiLogBuf)
		seq := guiLogSeq
		missed := seq - clientSeq
		var entries []guiLogEntry
		if missed > 0 {
			if missed > total {
				missed = total
			}
			entries = make([]guiLogEntry, missed)
			copy(entries, guiLogBuf[total-missed:])
		}
		guiLogMu.Unlock()

		var sb strings.Builder
		for _, e := range entries {
			sb.WriteString(e.Level)
			sb.WriteByte('\t')
			sb.WriteString(e.Msg)
			sb.WriteByte('\n')
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("X-Log-Seq", fmt.Sprintf("%d", seq))
		w.Write([]byte(sb.String()))
	})

	mux.HandleFunc("/api/ping", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Direct bool `json:"direct"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)

		guiMu.Lock()
		coal := guiCoalescer
		guiMu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		start := time.Now()
		var err error
		if !req.Direct && coal != nil {
			_, err = coal.Submit("HEAD", "https://www.gstatic.com/generate_204", map[string]string{}, nil)
		} else if req.Direct {
			// Direct mode — always use fragmented connection, never relay
			conn, ok := core.DialFragment("www.gstatic.com:443")
			if ok {
				conn.Close()
			} else {
				err = fmt.Errorf("direct connection failed")
			}
		} else {
			cfg := loadConfig(configPath)
			urls := parseURLList(cfg["fronted-appscript-url"])
			key := cfg["auth-key"]
			if len(urls) == 0 || strings.TrimSpace(key) == "" {
				// No relay config — ping via direct fragmented connection to gstatic
				conn, ok := core.DialFragment("www.gstatic.com:443")
				if ok {
					conn.Close()
				} else {
					err = fmt.Errorf("direct connection failed")
				}
			} else {
				client := core.NewHTTPClient(15 * time.Second)
				_, err = core.RelayRequestMulti(client, urls, "www.google.com", key,
					"HEAD", "https://www.gstatic.com/generate_204",
					map[string]string{}, nil, 15*time.Second)
			}
		}
		ms := time.Since(start).Milliseconds()
		if err != nil {
			json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": err.Error()})
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"ok": true, "ms": ms})
	})

	mux.HandleFunc("/api/probes", func(w http.ResponseWriter, r *http.Request) {
		cfg := loadConfig(configPath)
		urls := parseURLList(cfg["fronted-appscript-url"])
		key := cfg["auth-key"]
		target := cfg["target-url"]
		if target == "" {
			target = "https://www.gstatic.com/generate_204"
		}

		client := &http.Client{Timeout: 30 * time.Second}
		probes := filterProbes(defaultProbes(), "")
		if len(urls) > 0 {
			fp, _ := frontedAppScriptProbes(urls[0], "www.google.com", key, target)
			probes = append(probes, fp...)
		}

		results := make([]result, 0, len(probes))
		for _, p := range probes {
			results = append(results, runProbe(client, p, 1, 30*time.Second, false))
		}
		json.NewEncoder(w).Encode(results)
	})

	mux.HandleFunc("/api/export", func(w http.ResponseWriter, r *http.Request) {
		cfg := loadConfig(configPath)
		urls := cfg["fronted-appscript-url"]
		key := cfg["auth-key"]
		json.NewEncoder(w).Encode(map[string]string{
			"url": urls,
			"key": key,
		})
	})

	mux.HandleFunc("/api/download-ca", func(w http.ResponseWriter, r *http.Request) {
		data, err := os.ReadFile(caCertPath)
		if err != nil {
			http.Error(w, "CA certificate not found. Click Init first.", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/x-x509-ca-cert")
		w.Header().Set("Content-Disposition", "attachment; filename=\"zyrln-ca.pem\"")
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "0")
		w.Write(data)
	})
	mux.HandleFunc("/api/open-certs-dir", func(w http.ResponseWriter, r *http.Request) {
		dir := filepath.Dir(caCertPath)
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			os.MkdirAll(dir, 0755)
		}
		open(dir)
		w.WriteHeader(http.StatusOK)
	})

	mux.HandleFunc("/api/settings", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			var req struct {
				DirectEnabled *bool `json:"directEnabled"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			if req.DirectEnabled != nil {
				core.SetDirectEnabled(*req.DirectEnabled)
				cfg := loadConfig(configPath)
				cfg["direct-enabled"] = fmt.Sprintf("%v", *req.DirectEnabled)
				_ = saveConfig(configPath, cfg)
			}
			w.WriteHeader(http.StatusOK)
			return
		}
		json.NewEncoder(w).Encode(map[string]any{
			"directEnabled": core.GetDirectEnabled(),
		})
	})

	// Serve frontend from /platforms/desktop/gui
	guiFS, err := fs.Sub(embeddedGUI, "gui")
	if err != nil {
		mux.Handle("/", http.FileServer(http.Dir(filepath.Join("platforms", "desktop", "gui"))))
	} else {
		mux.Handle("/", http.FileServer(http.FS(guiFS)))
	}
	return mux
}

func openBrowser(url string) {
	openPath(url)
}

func openPath(p string) {
	var err error
	switch runtime.GOOS {
	case "linux":
		err = exec.Command("xdg-open", p).Start()
	case "windows":
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", p).Start()
	case "darwin":
		err = exec.Command("open", p).Start()
	default:
		err = fmt.Errorf("unsupported platform")
	}
	if err != nil {
		fmt.Printf("Failed to open %s: %v\n", p, err)
	}
}
