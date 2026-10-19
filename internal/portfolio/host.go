package portfolio

import (
	"net"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/Bebii-Digital/bebii-blog/internal/helper/seo"
)

const (
	HostApex     = "apex"
	HostTenant   = "tenant"
	HostReserved = "reserved"
)

var reservedSubdomains = map[string]struct{}{
	"www": {}, "admin": {}, "api": {}, "blog": {}, "app": {}, "mail": {},
	"static": {}, "img": {}, "kelola": {},
}

// ClassifyHost reports whether the request host is the marketing site, a buyer subdomain, or a reserved label.
func ClassifyHost(host, root string) (kind, subdomain string) {
	host = hostname(host)
	root = hostname(root)
	if root == "" {
		root = "localhost"
	}
	if host == "" || host == root || host == "www."+root || host == "127.0.0.1" || host == "::1" {
		return HostApex, ""
	}
	suffix := "." + root
	if !strings.HasSuffix(host, suffix) {
		return HostApex, ""
	}
	label := strings.TrimSuffix(host, suffix)
	if label == "" || strings.Contains(label, ".") {
		return HostApex, ""
	}
	if _, reserved := reservedSubdomains[label]; reserved {
		return HostReserved, label
	}
	return HostTenant, label
}

func hostname(host string) string {
	host = strings.ToLower(strings.TrimSpace(host))
	if host == "" {
		return ""
	}
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	host = strings.TrimPrefix(host, "https://")
	host = strings.TrimPrefix(host, "http://")
	host = strings.Trim(host, ".")
	if i := strings.Index(host, "/"); i >= 0 {
		host = host[:i]
	}
	return host
}

// TenantPublicURL builds https://sub.root[:port]/ from the current request.
func TenantPublicURL(c *gin.Context, root, sub string) string {
	root = hostname(root)
	sub = strings.ToLower(strings.TrimSpace(sub))
	if root == "" {
		root = "localhost"
	}
	origin := seo.RequestOrigin(c)
	u, err := url.Parse(origin)
	if err != nil || u.Scheme == "" {
		return "http://" + sub + "." + root + "/"
	}
	port := u.Port()
	if port != "" && port != "80" && port != "443" {
		u.Host = net.JoinHostPort(sub+"."+root, port)
	} else {
		u.Host = sub + "." + root
	}
	u.Path = "/"
	u.RawQuery = ""
	u.Fragment = ""
	return u.String()
}
