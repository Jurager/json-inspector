package httpx

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// Config is what the engine may do on the wire. Its zero value is the app's default behaviour:
// the proxy the environment names, certificates verified, redirects followed, no cookie jar.
type Config struct {
	Timeout      time.Duration
	MaxBodyBytes int64
	// ProxyURL is an explicit proxy; empty means whatever the environment sets (HTTP_PROXY and
	// friends), which is what a desktop app should follow by default.
	ProxyURL string
	// SkipTLSVerify is for the self-signed certificate a development API usually has.
	SkipTLSVerify bool
	// NoRedirects stops at the first response, however it answers.
	NoRedirects  bool
	MaxRedirects int
	// UseCookieJar keeps cookies between requests. Off unless asked for: it changes what a request
	// does, and the default until now has been "each request on its own".
	UseCookieJar bool
	UserAgent    string
	// KeepConnections reuses an open connection for the next request to the same host.
	//
	// Off, and deliberately: a reused connection spends nothing on DNS, TCP or TLS, so the same
	// request sent twice reports a different breakdown — and the second report says nothing about
	// what a cold request costs. This app is a probe, not a browsing session; what a browser does
	// with its connections is not the question, what the endpoint does is.
	//
	// On, every request after the first is cheaper and the timings tab says so, because a phase that
	// did not happen is absent rather than zero and the tab draws what it is given. The settings
	// screen is where this is meant to end up; until then the zero value is the app's answer.
	KeepConnections bool
}

const (
	defaultTimeout      = 60 * time.Second
	defaultMaxBodyBytes = 8 << 20
	defaultMaxRedirects = 10
)

func (c Config) withDefaults() Config {
	if c.Timeout <= 0 {
		c.Timeout = defaultTimeout
	}
	if c.MaxBodyBytes <= 0 {
		c.MaxBodyBytes = defaultMaxBodyBytes
	}
	if c.MaxRedirects <= 0 {
		c.MaxRedirects = defaultMaxRedirects
	}
	return c
}

// newTransport builds the one transport every request shares, which is what makes the proxy and TLS
// settings apply at all.
func newTransport(cfg Config) *http.Transport {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	// Off means every request dials: Go sends `Connection: close`, and the server lets go of the
	// socket too. See Config.KeepConnections for why that is the default.
	transport.DisableKeepAlives = !cfg.KeepConnections

	if cfg.ProxyURL != "" {
		if parsed, err := url.Parse(cfg.ProxyURL); err == nil {
			transport.Proxy = http.ProxyURL(parsed)
		}
		// A malformed proxy URL falls back to the environment rather than failing the request:
		// the setting is validated where it is entered.
	}
	if cfg.SkipTLSVerify {
		// #nosec G402 — the user asked for this on a development endpoint.
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}
	return transport
}

// redirectPolicy is the client's CheckRedirect. Without it, Go follows up to ten redirects and
// returns the last response, which is the behaviour this keeps.
func redirectPolicy(cfg Config) func(*http.Request, []*http.Request) error {
	if cfg.NoRedirects {
		return func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		}
	}
	return func(_ *http.Request, via []*http.Request) error {
		if len(via) >= cfg.MaxRedirects {
			return fmt.Errorf("stopped after %d redirects", cfg.MaxRedirects)
		}
		return nil
	}
}
