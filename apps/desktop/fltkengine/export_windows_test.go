//go:build windows && amd64 && cgo

package fltkengine

import "context"

// CoreProperty reads an mpv property on the UI thread, for tests that check what mpv
// actually applied.
func (engine *Engine) CoreProperty(ctx context.Context, name string) (string, error) {
	var value string
	err := dispatch(ctx, func() error {
		value = engine.core.GetPropertyString(name)
		return nil
	})
	return value, err
}

// AllowEnvironmentProxy drops the option that makes mpv ignore the http_proxy environment
// variable, for the control run that shows the option is what keeps the stream direct.
func AllowEnvironmentProxy() { environmentProxyAllowed = true }
