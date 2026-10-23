// SPDX-License-Identifier: MPL-2.0

package backend

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"testing"

	"animeportable/adapters/network/forwardproxy"
	"animeportable/adapters/network/securehttp"
	"animeportable/adapters/update/github"
	"animeportable/core"
	"animeportable/internal/release"
)

type fakeReleases struct {
	release github.Release
	found   bool
	err     error
}

func (f fakeReleases) Newest(context.Context) (github.Release, bool, error) {
	return f.release, f.found, f.err
}

func updateService(source releaseSource) *Service {
	return newWithDependencies(dependencies{store: newFakeStore(), source: &fakeSource{}, updates: source})
}

// raisedVersion is this build's version with the minor (part 1) or patch (part 2) number raised,
// so the newer cases stay newer after every release bump.
func raisedVersion(t *testing.T, part int) string {
	t.Helper()
	numbers, stage, hasStage := strings.Cut(release.Version, "-")
	parts := strings.Split(numbers, ".")
	value, err := strconv.Atoi(parts[part])
	if len(parts) != 3 || err != nil {
		t.Fatalf("version %q", release.Version)
	}
	parts[part] = strconv.Itoa(value + 1)
	if part == 1 {
		parts[2] = "0"
	}
	if hasStage {
		return strings.Join(parts, ".") + "-" + stage
	}
	return strings.Join(parts, ".")
}

func TestCheckForUpdateComparesWithThisBuild(t *testing.T) {
	page := "https://github.com/loustack17/AnimePortable/releases/tag/"
	newerMinor, newerPatch := raisedVersion(t, 1), raisedVersion(t, 2)
	tests := []struct {
		name  string
		fake  fakeReleases
		want  UpdateInfo
		wantE error
	}{
		{"newer minor", fakeReleases{release: github.Release{Tag: "v" + newerMinor, URL: page + "v" + newerMinor}, found: true}, UpdateInfo{Current: release.Version, Latest: newerMinor, Newer: true, URL: page + "v" + newerMinor}, nil},
		{"newer patch", fakeReleases{release: github.Release{Tag: "v" + newerPatch}, found: true}, UpdateInfo{Current: release.Version, Latest: newerPatch, Newer: true}, nil},
		{"same", fakeReleases{release: github.Release{Tag: "v" + release.Version, URL: page + "v" + release.Version}, found: true}, UpdateInfo{Current: release.Version, Latest: release.Version, URL: page + "v" + release.Version}, nil},
		{"older", fakeReleases{release: github.Release{Tag: "v0.1.0-alpha.2"}, found: true}, UpdateInfo{Current: release.Version, Latest: "0.1.0-alpha.2"}, nil},
		{"none published", fakeReleases{}, UpdateInfo{Current: release.Version}, nil},
		{"lookup failed", fakeReleases{err: errors.New("secret detail")}, UpdateInfo{}, ErrUnavailable},
		{"offline", fakeReleases{err: &securehttp.Error{Kind: securehttp.KindNetwork}}, UpdateInfo{}, ErrOffline},
		{"unparseable tag", fakeReleases{release: github.Release{Tag: "latest"}, found: true}, UpdateInfo{}, ErrUnavailable},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := updateService(test.fake).CheckForUpdate(context.Background())
			if !errors.Is(err, test.wantE) || got != test.want {
				t.Fatalf("got %+v, %v; want %+v, %v", got, err, test.want, test.wantE)
			}
		})
	}
	if _, err := updateService(nil).CheckForUpdate(context.Background()); err != ErrUnavailable {
		t.Fatalf("without a source: %v", err)
	}
}

func TestTestProxyReportsReasonsAndUsesGivenSettings(t *testing.T) {
	var gotAddress string
	probeWith := func(err error) proxyProbe {
		return func(_ context.Context, proxy func() string) error {
			gotAddress = proxy()
			return err
		}
	}
	tests := []struct {
		name   string
		input  Settings
		probe  proxyProbe
		want   ProxyTestResult
		wantIn string
	}{
		{"ok", Settings{ProxyMode: "manual", ProxyAddress: " 127.0.0.1:7890 "}, probeWith(nil), ProxyTestResult{OK: true}, "127.0.0.1:7890"},
		{"none is direct", Settings{ProxyMode: "none", ProxyAddress: "127.0.0.1:7890"}, probeWith(nil), ProxyTestResult{OK: true}, ""},
		{"unreachable", Settings{ProxyMode: "manual", ProxyAddress: "127.0.0.1:1"}, probeWith(&securehttp.Error{Kind: securehttp.KindProxy}), ProxyTestResult{Reason: ReasonProxyUnreachable}, "127.0.0.1:1"},
		{"refused", Settings{ProxyMode: "manual", ProxyAddress: "127.0.0.1:1"}, probeWith(&securehttp.Error{Kind: securehttp.KindProxyRefused}), ProxyTestResult{Reason: ReasonProxyRefused}, "127.0.0.1:1"},
		{"timeout", Settings{ProxyMode: "manual", ProxyAddress: "127.0.0.1:1"}, probeWith(&securehttp.Error{Kind: securehttp.KindTimeout}), ProxyTestResult{Reason: ReasonTimeout}, "127.0.0.1:1"},
		{"other failure", Settings{ProxyMode: "manual", ProxyAddress: "127.0.0.1:1"}, probeWith(&securehttp.Error{Kind: securehttp.KindNetwork}), ProxyTestResult{Reason: ReasonFailed}, "127.0.0.1:1"},
		{"bad address", Settings{ProxyMode: "manual", ProxyAddress: "http://127.0.0.1:1"}, probeWith(nil), ProxyTestResult{Reason: ReasonBadAddress}, "unchanged"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service := updateService(nil)
			service.probeProxy = test.probe
			gotAddress = "unchanged"
			got, err := service.TestProxy(context.Background(), test.input)
			if err != nil || got != test.want || gotAddress != test.wantIn {
				t.Fatalf("got %+v, %v, address %q; want %+v, address %q", got, err, gotAddress, test.want, test.wantIn)
			}
		})
	}
	service := updateService(nil)
	service.probeProxy = probeWith(nil)
	if _, err := service.TestProxy(context.Background(), Settings{ProxyMode: "socks"}); err != ErrInvalidInput {
		t.Fatalf("invalid mode: %v", err)
	}
	// Testing never changes the saved proxy.
	service = updateService(nil)
	service.probeProxy = probeWith(nil)
	if _, err := service.TestProxy(context.Background(), Settings{ProxyMode: "manual", ProxyAddress: "127.0.0.1:7890"}); err != nil || service.proxy.Address() != "" {
		t.Fatalf("TestProxy changed the service proxy: %v", err)
	}
}

func TestTestProxyDoesNotClaimSuccessForSettingsThatDoNothing(t *testing.T) {
	var system forwardproxy.SystemProxy
	var systemErr error
	probed := false
	run := func(input Settings) ProxyTestResult {
		t.Helper()
		service := updateService(nil)
		service.proxy = forwardproxy.NewWithReader(func() (forwardproxy.SystemProxy, error) { return system, systemErr })
		service.probeProxy = func(_ context.Context, proxy func() string) error {
			probed = true
			return nil
		}
		probed = false
		result, err := service.TestProxy(context.Background(), input)
		if err != nil {
			t.Fatal(err)
		}
		return result
	}
	if got := run(Settings{ProxyMode: "manual", ProxyAddress: "  "}); got != (ProxyTestResult{Reason: ReasonNoAddress}) || probed {
		t.Fatalf("manual without an address: %+v, probed %v", got, probed)
	}
	// Direct is a real test of the connection to Anime1.
	if got := run(Settings{ProxyMode: "none"}); !got.OK || !probed {
		t.Fatalf("none: %+v, probed %v", got, probed)
	}
	system = forwardproxy.SystemProxy{Enabled: true, Server: "socks=127.0.0.1:1080"}
	if got := run(Settings{ProxyMode: "system"}); got != (ProxyTestResult{Reason: ReasonSystemUnusable}) || probed {
		t.Fatalf("SOCKS-only system proxy: %+v, probed %v", got, probed)
	}
	system = forwardproxy.SystemProxy{AutoConfig: true}
	if got := run(Settings{ProxyMode: "system"}); got.Reason != ReasonSystemUnusable {
		t.Fatalf("PAC system proxy: %+v", got)
	}
	system, systemErr = forwardproxy.SystemProxy{}, errors.New("registry")
	if got := run(Settings{ProxyMode: "system"}); got.Reason != ReasonSystemUnusable {
		t.Fatalf("unreadable system proxy: %+v", got)
	}
	// A usable system proxy is tested through that proxy; one that is off is tested directly.
	system, systemErr = forwardproxy.SystemProxy{Enabled: true, Server: "http://proxy.example:8080"}, nil
	if got := run(Settings{ProxyMode: "system"}); !got.OK || !probed {
		t.Fatalf("usable system proxy: %+v, probed %v", got, probed)
	}
	system = forwardproxy.SystemProxy{}
	if got := run(Settings{ProxyMode: "system"}); !got.OK || !probed {
		t.Fatalf("system proxy off: %+v, probed %v", got, probed)
	}
}

func TestSaveSettingsUpdatesTheServiceProxy(t *testing.T) {
	service := updateService(nil)
	if err := service.SaveSettings(context.Background(), Settings{ProxyMode: "manual", ProxyAddress: "127.0.0.1:7890"}); err != nil {
		t.Fatal(err)
	}
	if got := service.proxy.Address(); got != "127.0.0.1:7890" {
		t.Fatalf("after manual save: %q", got)
	}
	if err := service.SaveSettings(context.Background(), Settings{ProxyMode: "manual", ProxyAddress: "ftp://bad"}); err != ErrInvalidInput {
		t.Fatal(err)
	}
	if got := service.proxy.Address(); got != "127.0.0.1:7890" {
		t.Fatalf("a rejected save changed the proxy to %q", got)
	}
	if err := service.SaveSettings(context.Background(), Settings{ProxyMode: "none"}); err != nil {
		t.Fatal(err)
	}
	if got := service.proxy.Address(); got != "" {
		t.Fatalf("after none: %q", got)
	}
	// A source handed in by the composition root is the one that is kept in step.
	shared := forwardproxy.New()
	withShared := NewAtWithPlayback("", shared, nil, nil)
	withShared.store, withShared.source = newFakeStore(), &fakeSource{}
	withShared.app = core.NewApp(withShared.source, nil, withShared.store)
	if err := withShared.SaveSettings(context.Background(), Settings{ProxyMode: "manual", ProxyAddress: "127.0.0.1:1"}); err != nil || shared.Address() != "127.0.0.1:1" {
		t.Fatalf("shared source = %q, %v", shared.Address(), err)
	}
}
