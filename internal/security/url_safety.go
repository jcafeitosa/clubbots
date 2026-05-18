package security

import (
	"net"
	"net/url"
	"strings"
)

// SSRF-blocked addresses — cloud metadata endpoints + internal networks.
// Inspired by Hermes Agent's url_safety.py pattern.
var ssrfBlockedCIDRs = []string{
	"169.254.169.254/32", // AWS EC2 metadata
	"169.254.170.2/32",   // AWS ECS metadata
	"100.100.100.200/32", // Alibaba Cloud metadata
	"169.254.0.0/16",     // link-local (AWS/Azure/GCP metadata fallback)
	"100.64.0.0/10",      // CGNAT (RFC 6598)
	"0.0.0.0/8",          // current network
	"10.0.0.0/8",         // private A
	"172.16.0.0/12",      // private B
	"192.168.0.0/16",     // private C
	"127.0.0.0/8",        // loopback
	"::1/128",            // IPv6 loopback
	"fe80::/10",          // IPv6 link-local
	"fc00::/7",           // IPv6 unique local
}

var ssrfBlockedHosts = []string{
	"metadata.google.internal", // GCP metadata
	"169.254.169.254",          // bare IP
	"localhost",
	"0.0.0.0",
	"[::1]",
}

var blockedNetworks []*net.IPNet

func init() {
	for _, cidr := range ssrfBlockedCIDRs {
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			continue
		}
		blockedNetworks = append(blockedNetworks, network)
	}
}

// IsSSRFSafe checks if a URL targets a blocked internal/cloud-metadata address.
// Returns false if the URL is blocked (SSRF risk).
func IsSSRFSafe(rawURL string) (bool, string) {
	if rawURL == "" {
		return false, "empty URL"
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return false, "invalid URL: " + err.Error()
	}

	host := parsed.Hostname()
	if host == "" {
		return false, "missing host in URL"
	}

	// Check blocked hostnames
	lowerHost := strings.ToLower(host)
	for _, blocked := range ssrfBlockedHosts {
		if lowerHost == blocked {
			return false, "blocked host: " + blocked
		}
	}

	// Resolve DNS and check IP
	ips, err := net.LookupIP(host)
	if err != nil {
		// DNS resolution failure — allow (might be a valid external host)
		return true, ""
	}

	for _, ip := range ips {
		for _, network := range blockedNetworks {
			if network.Contains(ip) {
				return false, "blocked IP range: " + ip.String() + " in " + network.String()
			}
		}
	}

	return true, ""
}

// ValidateURLForFetch checks a URL against SSRF + scheme allowlists.
// Only allows http/https schemes. Returns nil if safe.
func ValidateURLForFetch(rawURL string) error {
	if safe, reason := IsSSRFSafe(rawURL); !safe {
		return &URLBlockedError{URL: rawURL, Reason: reason}
	}

	parsed, _ := url.Parse(rawURL)
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return &URLBlockedError{URL: rawURL, Reason: "blocked scheme: " + scheme}
	}

	return nil
}

// URLBlockedError indicates a URL was blocked by security policy.
type URLBlockedError struct {
	URL    string
	Reason string
}

func (e *URLBlockedError) Error() string {
	return "URL blocked: " + e.Reason + " (" + e.URL + ")"
}
