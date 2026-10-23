package httpapi

import (
	"errors"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
)

// 固定错误不包含原始配置、请求头、Referer 或客户端信息。
var (
	ErrOriginConfig  = errors.New("invalid origin configuration")
	ErrRequestSource = errors.New("invalid request source")
	ErrBrowserOrigin = errors.New("invalid browser origin")
)

// OriginPolicy 只能由构造器建立；配置复制并保存在私有字段中，不接受请求覆盖。
type OriginPolicy struct {
	origin  string
	proxies []netip.Prefix
}

// RequestSource 仅在校验成功时返回；失败总是返回零值。
type RequestSource struct {
	ConfiguredOrigin string
	ExternalOrigin   string
	HTTPS            bool
	Secure           bool
	DirectPeer       netip.Addr
	ClientIP         netip.Addr
	Forwarded        bool
}

// NewOriginPolicy 接收单个 HTTPS Origin 和显式可信代理 IP/CIDR；空列表不信任代理。
func NewOriginPolicy(externalOrigin string, trustedProxies []string) (*OriginPolicy, error) {
	origin, ok := parseOrigin(externalOrigin, false)
	if !ok {
		return nil, ErrOriginConfig
	}
	p := &OriginPolicy{origin: origin}
	for _, raw := range trustedProxies {
		var prefix netip.Prefix
		if ip, err := netip.ParseAddr(raw); err == nil && ip.Zone() == "" {
			ip = ip.Unmap()
			prefix = netip.PrefixFrom(ip, ip.BitLen())
		} else {
			var err error
			prefix, err = netip.ParsePrefix(raw)
			if err != nil || prefix.Bits() == 0 {
				return nil, ErrOriginConfig
			}
			// mapped IPv6 与直接 peer 使用同一 IPv4 表示；不扩大到整个 IPv4 空间。
			if prefix.Addr().Is4In6() {
				if prefix.Bits() <= 96 {
					return nil, ErrOriginConfig
				}
				prefix = netip.PrefixFrom(prefix.Addr().Unmap(), prefix.Bits()-96)
			}
		}
		p.proxies = append(p.proxies, prefix.Masked())
	}
	return p, nil
}

// Resolve 只校验传输来源，供包括 Android Bearer 在内的调用方使用，不要求浏览器头。
func (p *OriginPolicy) Resolve(r *http.Request) (RequestSource, error) {
	if p == nil || p.origin == "" || r == nil {
		return RequestSource{}, ErrRequestSource
	}
	host, port, err := net.SplitHostPort(r.RemoteAddr)
	peer, ipErr := netip.ParseAddr(host)
	if err != nil || ipErr != nil || peer.Zone() != "" || !validPort(port) {
		return RequestSource{}, ErrRequestSource
	}
	peer = peer.Unmap()
	trusted := false
	for _, prefix := range p.proxies {
		trusted = trusted || prefix.Contains(peer)
	}
	client, authority, secure, forwarded := peer, r.Host, r.TLS != nil, false
	if trusted {
		_, hasForwarded, _ := singleHeader(r.Header, "Forwarded")
		fh, hh, vh := singleHeader(r.Header, "X-Forwarded-Host")
		fp, hp, vp := singleHeader(r.Header, "X-Forwarded-Proto")
		ff, hf, vf := singleHeader(r.Header, "X-Forwarded-For")
		if hasForwarded {
			return RequestSource{}, ErrRequestSource
		}
		if hh || hp || hf {
			if !hh || !hp || !hf || !vh || !vp || !vf || fp != "https" {
				return RequestSource{}, ErrRequestSource
			}
			client, err = netip.ParseAddr(ff)
			if err != nil || client.Zone() != "" {
				return RequestSource{}, ErrRequestSource
			}
			client, authority, secure, forwarded = client.Unmap(), fh, true, true
		}
	}
	// 不读取 URL.Scheme 或其它转发字段；任何失败都没有直连回退路径。
	origin, ok := parseOrigin("https://"+authority, false)
	if !secure || !ok || origin != p.origin {
		return RequestSource{}, ErrRequestSource
	}
	return RequestSource{
		ConfiguredOrigin: p.origin,
		ExternalOrigin:   origin,
		HTTPS:            true,
		Secure:           true,
		DirectPeer:       peer,
		ClientIP:         client,
		Forwarded:        forwarded,
	}, nil
}

// ValidateLogin 要求 Origin；Referer 不能代替登录 Origin。
func (p *OriginPolicy) ValidateLogin(r *http.Request) (RequestSource, error) {
	return p.validateBrowser(r, true)
}

// ValidateWrite 在 Origin 缺失时才允许 Referer 回退；调用方仍须执行会话 CSRF。
func (p *OriginPolicy) ValidateWrite(r *http.Request) (RequestSource, error) {
	return p.validateBrowser(r, false)
}

func (p *OriginPolicy) validateBrowser(r *http.Request, login bool) (RequestSource, error) {
	source, err := p.Resolve(r)
	if err != nil {
		return RequestSource{}, err
	}
	o, ho, vo := singleHeader(r.Header, "Origin")
	f, hf, vf := singleHeader(r.Header, "Referer")
	if (!ho && (login || !hf)) || (ho && !vo) || (hf && !vf) {
		return RequestSource{}, ErrBrowserOrigin
	}
	for _, candidate := range []struct {
		value   string
		present bool
		referer bool
	}{{o, ho, false}, {f, hf, true}} {
		if candidate.present {
			origin, ok := parseOrigin(candidate.value, candidate.referer)
			if !ok || origin != p.origin {
				return RequestSource{}, ErrBrowserOrigin
			}
		}
	}
	return source, nil
}

// 遍历大小写等价键，覆盖手工 Request 中非规范键名和重复字段的边界。
func singleHeader(h http.Header, name string) (string, bool, bool) {
	var values []string
	present := false
	for key, v := range h {
		if strings.EqualFold(key, name) {
			present = true
			values = append(values, v...)
		}
	}
	if !present || len(values) != 1 || values[0] == "" {
		return "", present, false
	}
	return values[0], true, true
}

func parseOrigin(raw string, referer bool) (string, bool) {
	for _, c := range raw {
		if c <= 32 || c == 127 || c == '\\' {
			return "", false
		}
	}
	u, err := url.Parse(raw)
	if err != nil || !strings.EqualFold(u.Scheme, "https") || u.Opaque != "" || u.Host == "" || u.User != nil || strings.Contains(raw, "#") {
		return "", false
	}
	if !referer && (u.Path != "" || u.RawQuery != "" || u.ForceQuery) {
		return "", false
	}
	if _, err := url.QueryUnescape(u.RawQuery); err != nil {
		return "", false
	}
	host, port, ok := parseAuthority(u.Host)
	if !ok {
		return "", false
	}
	if port != "443" {
		host = net.JoinHostPort(host, port)
	} else if strings.Contains(host, ":") {
		host = "[" + host + "]"
	}
	return "https://" + host, true
}

func parseAuthority(raw string) (string, string, bool) {
	host, port := raw, "443"
	if strings.HasPrefix(raw, "[") {
		end := strings.IndexByte(raw, ']')
		if end < 0 {
			return "", "", false
		}
		host = raw[1:end]
		tail := raw[end+1:]
		if tail != "" {
			if !strings.HasPrefix(tail, ":") {
				return "", "", false
			}
			port = tail[1:]
		}
		ip, err := netip.ParseAddr(host)
		if err != nil || !ip.Is6() || ip.Zone() != "" {
			return "", "", false
		}
		host = ip.Unmap().String()
	} else {
		if strings.Contains(raw, ":") {
			var err error
			host, port, err = net.SplitHostPort(raw)
			if err != nil {
				return "", "", false
			}
		}
		if ip, err := netip.ParseAddr(host); err == nil {
			host = ip.Unmap().String()
		} else {
			if !validDNSHost(host) {
				return "", "", false
			}
			host = strings.ToLower(host)
		}
	}
	if !validPort(port) {
		return "", "", false
	}
	n, _ := strconv.Atoi(port)
	return host, strconv.Itoa(n), true
}

func validPort(raw string) bool {
	if raw == "" {
		return false
	}
	for _, c := range raw {
		if c < '0' || c > '9' {
			return false
		}
	}
	n, err := strconv.Atoi(raw)
	return err == nil && n > 0 && n <= 65535
}

func validDNSHost(host string) bool {
	// 尾点保留为独立主机名，不擅自与无尾点域名合并；IDN 必须显式使用 A-label。
	name := strings.TrimSuffix(host, ".")
	if len(name) == 0 || len(name) > 253 {
		return false
	}
	for _, label := range strings.Split(name, ".") {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, c := range label {
			if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-') {
				return false
			}
		}
	}
	return true
}
