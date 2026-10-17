// [INPUT]: The current MyGo locale, target URL from native settings, HTTP permission, and navigation URLs.
// [OUTPUT]: Normalized URLs, allowed navigation schemes, and exact origins; reject common tokens in persisted URLs only.
// [POS]: Separate temporary login redirects, the saved home page, and device permission boundaries.
// [PROTOCOL]: Update this header and internal/site/AGENTS.md; update the root AGENTS.md when directory boundaries change.
package site

import (
	"errors"
	"net"
	"net/url"
	"strconv"
	"strings"
	"unicode"

	l10n "github.com/ohxxx/PageDock/internal/locale"
)

const maxURLLength = 8192

func ParseURL(raw string) (*url.URL, error) {
	if len(raw) > maxURLLength || strings.ContainsAny(raw, "\\\r\n\t") {
		return nil, errors.New(l10n.Text("urlCharacters"))
	}
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u == nil || u.Opaque != "" || u.Host == "" {
		return nil, errors.New(l10n.Text("urlComplete"))
	}
	u.Scheme = strings.ToLower(u.Scheme)
	if u.Scheme != "https" && u.Scheme != "http" {
		return nil, errors.New(l10n.Text("urlScheme"))
	}
	if u.User != nil {
		return nil, errors.New(l10n.Text("urlCredentials"))
	}
	host := strings.ToLower(u.Hostname())
	if host == "" || strings.ContainsFunc(host, func(r rune) bool { return r > unicode.MaxASCII || unicode.IsSpace(r) }) {
		return nil, errors.New(l10n.Text("urlDomain"))
	}
	if strings.Contains(host, ":") {
		if net.ParseIP(host) == nil {
			return nil, errors.New(l10n.Text("urlIPv6"))
		}
		host = "[" + host + "]"
	} else {
		for _, label := range strings.Split(strings.TrimSuffix(host, "."), ".") {
			if label == "" || strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-") {
				return nil, errors.New(l10n.Text("urlDomainFormat"))
			}
			for _, c := range label {
				if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-') {
					return nil, errors.New(l10n.Text("urlDomainFormat"))
				}
			}
		}
	}
	port := u.Port()
	if port != "" {
		n, err := strconv.Atoi(port)
		if err != nil || n < 1 || n > 65535 {
			return nil, errors.New(l10n.Text("urlPort"))
		}
		port = strconv.Itoa(n)
		if u.Scheme == "https" && port == "443" || u.Scheme == "http" && port == "80" {
			port = ""
		}
	}
	u.Host = host
	if port != "" {
		u.Host += ":" + port
	}
	return u, nil
}

func NavigationURL(raw string, allowHTTP bool) (*url.URL, error) {
	u, err := ParseURL(raw)
	if err != nil {
		return nil, err
	}
	if u.Scheme == "http" && !allowHTTP {
		return nil, errors.New(l10n.Text("urlHTTPS"))
	}
	if strings.EqualFold(u.Hostname(), "mygo.localhost") {
		return nil, errors.New(l10n.Text("urlReserved"))
	}
	return u, nil
}

func TargetURL(raw string, allowHTTP bool) (*url.URL, error) {
	u, err := NavigationURL(raw, allowHTTP)
	if err != nil {
		return nil, err
	}
	// Reject common credentials in saved login links; silently removing them could change the link's meaning.
	queries := []string{u.RawQuery, u.Fragment}
	if i := strings.IndexByte(u.Fragment, '?'); i >= 0 {
		queries = append(queries, u.Fragment[i+1:])
	}
	for _, query := range queries {
		values, err := url.ParseQuery(query)
		if err != nil {
			return nil, errors.New(l10n.Text("urlParameters"))
		}
		for key := range values {
			switch strings.ToLower(strings.ReplaceAll(key, "-", "_")) {
			case "token", "access_token", "refresh_token", "id_token", "api_key", "apikey", "password", "secret", "code":
				return nil, errors.New(l10n.Text("urlTokens"))
			}
		}
	}
	if u.Path == "" {
		u.Path = "/"
	}
	return u, nil
}

func Origin(u *url.URL) string { return u.Scheme + "://" + u.Host }

func SameOrigin(raw, expected string) bool {
	u, err := ParseURL(raw)
	return err == nil && Origin(u) == expected
}
