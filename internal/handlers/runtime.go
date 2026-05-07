package handlers

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type RuntimeConfig struct {
	MaxRequests          int
	MaxRequestBodyBytes  int64
	MaxResponseBodyBytes int64
	AllowedWSOrigins     []string
	RedactionEnabled     bool
	RedactionHeaders     []string
	RedactionFields      []string
	AlertWebhookURL      string
	AlertMinSentStatus   int
	AlertOnSentError     bool
}

var (
	runtimeMu                 sync.RWMutex
	runtimeMaxRequests              = 10000
	runtimeMaxReqBody         int64 = 1024 * 1024
	runtimeMaxRespBody        int64 = 2 * 1024 * 1024
	runtimeWSOriginLookup           = map[string]struct{}{}
	runtimeRedactionEnabled         = true
	runtimeRedactionHeaders         = toLookupMap([]string{"authorization", "cookie", "set-cookie", "x-api-key", "api-key", "proxy-authorization"})
	runtimeRedactionFields          = toLookupMap([]string{"password", "passwd", "secret", "token", "api_key", "apikey", "access_token", "refresh_token", "client_secret"})
	runtimeAlertWebhookURL          = ""
	runtimeAlertMinSentStatus       = 500
	runtimeAlertOnSentError         = true
)

func ConfigureRuntime(cfg RuntimeConfig) {
	runtimeMu.Lock()
	defer runtimeMu.Unlock()

	if cfg.MaxRequests > 0 {
		runtimeMaxRequests = cfg.MaxRequests
	}
	if cfg.MaxRequestBodyBytes > 0 {
		runtimeMaxReqBody = cfg.MaxRequestBodyBytes
	}
	if cfg.MaxResponseBodyBytes > 0 {
		runtimeMaxRespBody = cfg.MaxResponseBodyBytes
	}
	runtimeRedactionEnabled = cfg.RedactionEnabled

	if len(cfg.RedactionHeaders) > 0 {
		runtimeRedactionHeaders = toLookupMap(cfg.RedactionHeaders)
	}
	if len(cfg.RedactionFields) > 0 {
		runtimeRedactionFields = toLookupMap(cfg.RedactionFields)
	}

	alertWebhookURL := strings.TrimSpace(cfg.AlertWebhookURL)
	if alertWebhookURL != "" {
		if err := ValidateHTTPOutboundURL(alertWebhookURL); err != nil {
			log.Printf("warning: alert webhook disabled: %v", err)
			alertWebhookURL = ""
		}
	}
	runtimeAlertWebhookURL = alertWebhookURL
	if cfg.AlertMinSentStatus >= 100 && cfg.AlertMinSentStatus <= 599 {
		runtimeAlertMinSentStatus = cfg.AlertMinSentStatus
	}
	runtimeAlertOnSentError = cfg.AlertOnSentError

	allowlist := make(map[string]struct{})
	for _, origin := range cfg.AllowedWSOrigins {
		normalizedHost, normalizedName := normalizeOriginEntry(origin)
		if normalizedHost != "" {
			allowlist[normalizedHost] = struct{}{}
		}
		if normalizedName != "" {
			allowlist[normalizedName] = struct{}{}
		}
	}
	runtimeWSOriginLookup = allowlist
}

func maxRequestBodyBytes() int64 {
	runtimeMu.RLock()
	defer runtimeMu.RUnlock()
	return runtimeMaxReqBody
}

func maxResponseBodyBytes() int64 {
	runtimeMu.RLock()
	defer runtimeMu.RUnlock()
	return runtimeMaxRespBody
}

func maxRequestsRetention() int {
	runtimeMu.RLock()
	defer runtimeMu.RUnlock()
	return runtimeMaxRequests
}

func redactionSettingsSnapshot() (bool, map[string]struct{}, map[string]struct{}) {
	runtimeMu.RLock()
	defer runtimeMu.RUnlock()

	headers := make(map[string]struct{}, len(runtimeRedactionHeaders))
	for k := range runtimeRedactionHeaders {
		headers[k] = struct{}{}
	}
	fields := make(map[string]struct{}, len(runtimeRedactionFields))
	for k := range runtimeRedactionFields {
		fields[k] = struct{}{}
	}

	return runtimeRedactionEnabled, headers, fields
}

func alertSettingsSnapshot() (string, int, bool) {
	runtimeMu.RLock()
	defer runtimeMu.RUnlock()
	return runtimeAlertWebhookURL, runtimeAlertMinSentStatus, runtimeAlertOnSentError
}

func toLookupMap(items []string) map[string]struct{} {
	lookup := make(map[string]struct{})
	for _, item := range items {
		normalized := strings.ToLower(strings.TrimSpace(item))
		if normalized == "" {
			continue
		}
		lookup[normalized] = struct{}{}
	}
	return lookup
}

func isAllowedWebSocketOrigin(r *http.Request) bool {
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" {
		// Non-browser websocket clients often omit Origin.
		return true
	}
	return isAllowedRequestOrigin(origin, r.Host)
}

func isAllowedSSEOrigin(origin string, r *http.Request) bool {
	origin = strings.TrimSpace(origin)
	if origin == "" {
		return false
	}
	if r == nil {
		return false
	}
	return isAllowedRequestOrigin(origin, r.Host)
}

func isAllowedRequestOrigin(origin, requestHost string) bool {
	originURL, err := url.Parse(origin)
	if err != nil || originURL.Host == "" {
		return false
	}

	originHost := strings.ToLower(strings.TrimSpace(originURL.Host))
	originName := hostnameFromHostPort(originHost)
	requestHost = strings.ToLower(strings.TrimSpace(requestHost))
	requestName := hostnameFromHostPort(requestHost)

	if originHost == requestHost || (originName != "" && originName == requestName) {
		return true
	}

	runtimeMu.RLock()
	defer runtimeMu.RUnlock()
	if _, ok := runtimeWSOriginLookup[originHost]; ok {
		return true
	}
	if originName != "" {
		if _, ok := runtimeWSOriginLookup[originName]; ok {
			return true
		}
	}
	return false
}

func newOutboundHTTPClient(timeout time.Duration) *http.Client {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	transport := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           outboundDialContext,
		ForceAttemptHTTP2:     true,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 30 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		IdleConnTimeout:       90 * time.Second,
		MaxIdleConns:          100,
		TLSClientConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
		},
	}

	return &http.Client{
		Timeout: timeout,
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("too many redirects")
			}
			if err := ValidateHTTPOutboundURL(req.URL.String()); err != nil {
				return fmt.Errorf("blocked redirect target: %w", err)
			}
			return nil
		},
	}
}

func outboundDialContext(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, fmt.Errorf("invalid target address")
	}
	host = strings.Trim(strings.TrimSpace(host), "[]")
	if host == "" {
		return nil, fmt.Errorf("invalid target host")
	}

	dialer := &net.Dialer{Timeout: 5 * time.Second, KeepAlive: 30 * time.Second}

	if ip := net.ParseIP(host); ip != nil {
		if isPrivateOrLocalIP(ip) {
			return nil, fmt.Errorf("target IP is blocked for security reasons")
		}
		return dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
	}

	normalizedHost := strings.ToLower(host)
	if isBlockedHostname(normalizedHost) {
		return nil, fmt.Errorf("target host is blocked for security reasons")
	}

	ips, err := net.DefaultResolver.LookupIP(ctx, "ip", host)
	if err != nil || len(ips) == 0 {
		return nil, fmt.Errorf("failed to resolve target host")
	}
	for _, ip := range ips {
		if isPrivateOrLocalIP(ip) {
			return nil, fmt.Errorf("target resolves to a blocked private/local address")
		}
	}

	var dialErr error
	for _, ip := range ips {
		conn, err := dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
		if err == nil {
			return conn, nil
		}
		dialErr = err
	}
	if dialErr != nil {
		return nil, dialErr
	}

	return nil, fmt.Errorf("failed to connect to target host")
}

func normalizeOriginEntry(raw string) (string, string) {
	trimmed := strings.ToLower(strings.TrimSpace(raw))
	if trimmed == "" {
		return "", ""
	}

	if !strings.Contains(trimmed, "://") {
		return trimmed, hostnameFromHostPort(trimmed)
	}

	parsed, err := url.Parse(trimmed)
	if err != nil || parsed.Host == "" {
		return "", ""
	}
	return strings.ToLower(parsed.Host), hostnameFromHostPort(parsed.Host)
}

func hostnameFromHostPort(raw string) string {
	raw = strings.TrimSpace(strings.ToLower(raw))
	if raw == "" {
		return ""
	}

	host, _, err := net.SplitHostPort(raw)
	if err == nil {
		return strings.Trim(host, "[]")
	}

	if strings.HasPrefix(raw, "[") && strings.HasSuffix(raw, "]") {
		return strings.Trim(raw, "[]")
	}

	return strings.Trim(raw, "[]")
}

func requestBaseURL(c *gin.Context) string {
	scheme := "http"
	if c.Request.TLS != nil || strings.EqualFold(c.GetHeader("X-Forwarded-Proto"), "https") {
		scheme = "https"
	}
	return scheme + "://" + c.Request.Host
}

func requestWSBaseURL(c *gin.Context) string {
	scheme := "ws"
	if c.Request.TLS != nil || strings.EqualFold(c.GetHeader("X-Forwarded-Proto"), "https") {
		scheme = "wss"
	}
	return scheme + "://" + c.Request.Host
}

func validateOutboundURL(rawURL string, allowedSchemes map[string]bool) error {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return fmt.Errorf("invalid URL")
	}
	if parsed.Scheme == "" || !allowedSchemes[strings.ToLower(parsed.Scheme)] {
		return fmt.Errorf("unsupported URL scheme")
	}
	if parsed.Host == "" {
		return fmt.Errorf("URL host is required")
	}

	host := strings.ToLower(parsed.Hostname())
	if host == "" {
		return fmt.Errorf("invalid URL host")
	}

	if isBlockedHostname(host) {
		return fmt.Errorf("target host is blocked for security reasons")
	}

	if ip := net.ParseIP(host); ip != nil {
		if isPrivateOrLocalIP(ip) {
			return fmt.Errorf("target IP is blocked for security reasons")
		}
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	ips, err := net.DefaultResolver.LookupIP(ctx, "ip", host)
	if err != nil || len(ips) == 0 {
		return fmt.Errorf("failed to resolve target host")
	}
	for _, ip := range ips {
		if isPrivateOrLocalIP(ip) {
			return fmt.Errorf("target resolves to a blocked private/local address")
		}
	}

	return nil
}

func ValidateHTTPOutboundURL(rawURL string) error {
	return validateOutboundURL(rawURL, map[string]bool{"http": true, "https": true})
}

func ValidateWSOutboundURL(rawURL string) error {
	return validateOutboundURL(rawURL, map[string]bool{"ws": true, "wss": true})
}

func isBlockedHostname(host string) bool {
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return true
	}
	if host == "metadata.google.internal" || host == "metadata" {
		return true
	}
	return false
}

func isPrivateOrLocalIP(ip net.IP) bool {
	if ip == nil {
		return false
	}
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast() || ip.IsUnspecified() {
		return true
	}

	if ip4 := ip.To4(); ip4 != nil {
		if ip4[0] == 169 && ip4[1] == 254 {
			return true
		}
		if ip4[0] == 100 && ip4[1] >= 64 && ip4[1] <= 127 {
			return true
		}
		if ip4[0] == 127 {
			return true
		}
	}

	return false
}
