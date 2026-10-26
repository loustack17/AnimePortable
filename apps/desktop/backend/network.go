// SPDX-License-Identifier: MPL-2.0

package backend

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"animeportable/adapters/network/securehttp"
	"animeportable/adapters/source/anime1"
	"animeportable/adapters/update/github"
	"animeportable/core"
	"animeportable/internal/release"
)

const (
	proxyTestURL     = "https://anime1.me/"
	proxyTestTimeout = 8 * time.Second
	updateTimeout    = 15 * time.Second

	// The reasons of a failed proxy test; the UI words them in its language.
	ReasonProxyUnreachable = "proxy_unreachable"
	ReasonProxyRefused     = "proxy_refused"
	ReasonTimeout          = "timeout"
	ReasonBadAddress       = "bad_address"
	ReasonNoAddress        = "no_address"
	ReasonSystemUnusable   = "system_unusable"
	ReasonFailed           = "failed"
)

// releaseSource finds the newest published release; the GitHub client is the production one.
type releaseSource interface {
	Newest(ctx context.Context) (github.Release, bool, error)
}

// proxyProbe makes one request to Anime1 through the proxy that proxy() names.
type proxyProbe func(ctx context.Context, proxy func() string) error

// TestProxy makes one request to Anime1 through the given, not yet saved, proxy settings. A
// failure is a result with a short reason, not an error, so the page can show it.
//
// A manual proxy without an address, and a system proxy the app cannot use (SOCKS only, PAC
// script, unreadable: the app would connect directly), are reported without a request,
// because "connected" would be wrong about what the settings do. With 不使用, the request is
// made directly, which tells the user whether Anime1 can be reached at all.
func (service *Service) TestProxy(ctx context.Context, input Settings) (ProxyTestResult, error) {
	operationCtx, done, err := service.begin(ctx)
	if err != nil {
		return ProxyTestResult{}, err
	}
	defer done()
	mode, err := proxyModeNames.parse(input.ProxyMode)
	if err != nil {
		return ProxyTestResult{}, err
	}
	address := strings.TrimSpace(input.ProxyAddress)
	if mode == core.ProxyModeManual {
		if address == "" {
			return ProxyTestResult{Reason: ReasonNoAddress}, nil
		}
		if !core.ValidProxyAddress(address) {
			return ProxyTestResult{Reason: ReasonBadAddress}, nil
		}
	}
	resolution := service.proxy.Preview(mode, address).Resolve()
	if resolution.SystemUnusable {
		return ProxyTestResult{Reason: ReasonSystemUnusable}, nil
	}
	probe := service.probeProxy
	if probe == nil {
		probe = requestAnime1
	}
	testCtx, cancel := context.WithTimeout(operationCtx, proxyTestTimeout)
	defer cancel()
	if err := probe(testCtx, func() string { return resolution.Address }); err != nil {
		if contextErr := operationCtx.Err(); contextErr != nil {
			return ProxyTestResult{}, safeError(contextErr)
		}
		return ProxyTestResult{Reason: proxyFailureReason(err)}, nil
	}
	return ProxyTestResult{OK: true}, nil
}

// requestAnime1 succeeds once response headers arrive: any HTTP status proves the route works.
func requestAnime1(ctx context.Context, proxy func() string) error {
	client, err := securehttp.New(securehttp.Config{AllowedOrigins: anime1.AllowedOrigins(), Proxy: proxy})
	if err != nil {
		return err
	}
	defer client.CloseIdleConnections()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, proxyTestURL, nil)
	if err != nil {
		return err
	}
	response, err := client.Open(request)
	if err != nil {
		return err
	}
	return response.Body.Close()
}

func proxyFailureReason(err error) string {
	var secure *securehttp.Error
	if errors.As(err, &secure) {
		switch secure.Kind {
		case securehttp.KindProxy:
			return ReasonProxyUnreachable
		case securehttp.KindProxyRefused:
			return ReasonProxyRefused
		case securehttp.KindTimeout:
			return ReasonTimeout
		}
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return ReasonTimeout
	}
	return ReasonFailed
}

// CheckForUpdate compares this build with the newest GitHub release, pre-releases included.
// It runs only when asked and never downloads anything.
func (service *Service) CheckForUpdate(ctx context.Context) (UpdateInfo, error) {
	operationCtx, done, err := service.begin(ctx)
	if err != nil {
		return UpdateInfo{}, err
	}
	defer done()
	service.mu.Lock()
	source := service.updates
	service.mu.Unlock()
	if source == nil {
		return UpdateInfo{}, ErrUnavailable
	}
	current, ok := release.Parse(release.Version)
	if !ok {
		return UpdateInfo{}, ErrUnavailable
	}
	lookupCtx, cancel := context.WithTimeout(operationCtx, updateTimeout)
	defer cancel()
	newest, found, err := source.Newest(lookupCtx)
	if err != nil {
		return UpdateInfo{}, safeError(err)
	}
	info := UpdateInfo{Current: release.Version}
	if !found {
		return info, nil
	}
	latest, ok := release.Parse(newest.Tag)
	if !ok {
		return UpdateInfo{}, ErrUnavailable
	}
	info.Latest = latest.String()
	info.Newer = latest.Compare(current) > 0
	info.URL = newest.URL
	return info, nil
}
