// SPDX-License-Identifier: MPL-2.0

package main

import (
	"context"
	"reflect"
	"testing"

	"animeportable/apps/desktop/backend"

	"github.com/wailsapp/wails/v3/pkg/application"
)

var (
	_ application.ServiceStartup  = (*backend.Service)(nil)
	_ application.ServiceShutdown = (*backend.Service)(nil)
)

func TestDesktopBindingSurface(t *testing.T) {
	allowed := map[string]bool{
		"Catalog": false, "Search": false, "Library": false,
		"Detail": false, "Episodes": false, "Following": false,
		"Follow": false, "Unfollow": false, "Schedule": false,
		"History": false, "RemoveHistory": false, "Settings": false,
		"SaveSettings": false, "Play": false, "GetCover": false,
	}
	serviceType := reflect.TypeOf((*backend.Service)(nil))
	contextType := reflect.TypeFor[context.Context]()
	errorType := reflect.TypeFor[error]()
	for i := 0; i < serviceType.NumMethod(); i++ {
		method := serviceType.Method(i)
		if method.Name == "ServiceStartup" || method.Name == "ServiceShutdown" {
			continue
		}
		if _, ok := allowed[method.Name]; !ok {
			t.Errorf("unexpected frontend capability: %s", method.Name)
			continue
		}
		allowed[method.Name] = true
		if method.Type.NumIn() < 2 || method.Type.In(1) != contextType {
			t.Errorf("%s must accept Wails-injected request context first", method.Name)
			continue
		}
		for j := 2; j < method.Type.NumIn(); j++ {
			assertBindingDTO(t, method.Name, method.Type.In(j))
		}
		for j := 0; j < method.Type.NumOut(); j++ {
			result := method.Type.Out(j)
			if result != errorType {
				assertBindingDTO(t, method.Name, result)
			}
		}
	}
	for name, found := range allowed {
		if !found {
			t.Errorf("missing application action: %s", name)
		}
	}
}

func assertBindingDTO(t *testing.T, method string, value reflect.Type) {
	t.Helper()
	if path := value.PkgPath(); path != "" && path != "animeportable/apps/desktop/backend" {
		t.Errorf("%s exposes non-DTO type %s", method, value)
		return
	}
	switch value.Kind() {
	case reflect.String, reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16,
		reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16,
		reflect.Uint32, reflect.Uint64, reflect.Float32, reflect.Float64:
	case reflect.Slice, reflect.Array, reflect.Pointer:
		assertBindingDTO(t, method, value.Elem())
	case reflect.Struct:
		for i := 0; i < value.NumField(); i++ {
			field := value.Field(i)
			if !field.IsExported() || field.Tag.Get("json") == "" {
				t.Errorf("%s DTO field %s.%s needs an explicit exported JSON contract", method, value, field.Name)
			}
			assertBindingDTO(t, method, field.Type)
		}
	default:
		t.Errorf("%s exposes unsupported binding type %s", method, value)
	}
}
