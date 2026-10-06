package authn

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

func ValidateSecureURL(raw string) (*url.URL, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Scheme == "" || u.Host == "" {
		return nil, fmt.Errorf("URL must be absolute")
	}
	if u.User != nil || u.Fragment != "" {
		return nil, fmt.Errorf("URL must not contain user info or fragment")
	}
	if u.Scheme == "https" {
		return u, nil
	}
	if u.Scheme == "http" && isLoopbackHost(u.Hostname()) {
		return u, nil
	}
	return nil, fmt.Errorf("URL must use HTTPS unless it is loopback")
}

func isLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
