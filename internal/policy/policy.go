package policy

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
)

type Policy struct {
	Mode     string
	Domains  []string
	Networks []*net.IPNet
}

func New(mode string, domains, cidrs []string) (Policy, error) {
	p := Policy{Mode: mode, Domains: domains}
	for _, raw := range cidrs {
		_, network, err := net.ParseCIDR(raw)
		if err != nil {
			return p, fmt.Errorf("invalid allowlist CIDR")
		}
		p.Networks = append(p.Networks, network)
	}
	return p, nil
}

func (p Policy) Validate(ctx context.Context, raw string) (string, error) {
	u, err := parseTarget(raw)
	if err != nil {
		return "", err
	}
	host := strings.TrimSuffix(strings.ToLower(u.Hostname()), ".")
	if host == "" {
		return "", fmt.Errorf("target host is required")
	}
	if ip := net.ParseIP(host); ip != nil {
		if !p.allowedIP(ip) {
			return "", fmt.Errorf("target is not allowed")
		}
		return u.String(), nil
	}
	if strings.Contains(host, "localhost") || !p.allowedDomain(host) {
		return "", fmt.Errorf("target is not allowed")
	}
	ips, err := net.DefaultResolver.LookupIP(ctx, "ip", host)
	if err != nil || len(ips) == 0 {
		return "", fmt.Errorf("target could not be resolved")
	}
	for _, ip := range ips {
		if blockedIP(ip) || (!p.allowedDomain(host) && !p.allowedIP(ip)) {
			return "", fmt.Errorf("target resolves to a disallowed address")
		}
	}
	return u.String(), nil
}

func parseTarget(raw string) (*url.URL, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.ContainsAny(raw, "\r\n") {
		return nil, fmt.Errorf("invalid target")
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.Host == "" || u.Path != "" && strings.Contains(u.Path, "@") {
		return nil, fmt.Errorf("invalid target")
	}
	if u.Port() != "" {
		port, parseErr := strconv.Atoi(u.Port())
		if parseErr != nil || port < 1 || port > 65535 {
			return nil, fmt.Errorf("invalid port")
		}
	}
	return u, nil
}

func (p Policy) allowedDomain(host string) bool {
	if strings.EqualFold(p.Mode, "open") {
		return true
	}
	for _, domain := range p.Domains {
		domain = strings.TrimPrefix(strings.ToLower(domain), ".")
		if host == domain || strings.HasSuffix(host, "."+domain) {
			return true
		}
	}
	return false
}
func (p Policy) allowedIP(ip net.IP) bool {
	for _, network := range p.Networks {
		if network.Contains(ip) {
			return true
		}
	}
	return strings.EqualFold(p.Mode, "open") && !blockedIP(ip)
}
func blockedIP(ip net.IP) bool {
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() || ip.IsMulticast() || ip.Equal(net.ParseIP("169.254.169.254"))
}
