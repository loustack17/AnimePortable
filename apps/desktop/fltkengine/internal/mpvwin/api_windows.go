//go:build windows && amd64

// SPDX-License-Identifier: MPL-2.0

package mpvwin

import (
	"math"
	"runtime"
	"sync"
	"sync/atomic"
	"unsafe"

	"github.com/ebitengine/purego"
)

type api struct {
	create                   func() uintptr
	setOptionString          func(uintptr, string, string) int32
	initialize               func(uintptr) int32
	terminateDestroy         func(uintptr)
	command                  func(uintptr, **byte) int32
	setProperty              func(uintptr, string, int32, unsafe.Pointer) int32
	getPropertyString        func(uintptr, string) unsafe.Pointer
	waitEvent                func(uintptr, float64) unsafe.Pointer
	wakeup                   func(uintptr)
	free                     func(unsafe.Pointer)
	renderContextCreate      func(*uintptr, uintptr, unsafe.Pointer) int32
	renderContextRender      func(uintptr, unsafe.Pointer) int32
	renderContextUpdate      func(uintptr) uint64
	renderContextReportSwap  func(uintptr)
	renderContextSetUpdateCB func(uintptr, uintptr, uintptr)
	renderContextFree        func(uintptr)
}

var requiredSymbols = []string{
	"mpv_create",
	"mpv_set_option_string",
	"mpv_initialize",
	"mpv_terminate_destroy",
	"mpv_command",
	"mpv_set_property",
	"mpv_get_property_string",
	"mpv_wait_event",
	"mpv_wakeup",
	"mpv_free",
	"mpv_render_context_create",
	"mpv_render_context_render",
	"mpv_render_context_update",
	"mpv_render_context_report_swap",
	"mpv_render_context_set_update_callback",
	"mpv_render_context_free",
}

func bindAPI(handle uintptr, resolve func(uintptr, string) (uintptr, error)) (api, error) {
	addresses := make(map[string]uintptr, len(requiredSymbols))
	for _, name := range requiredSymbols {
		address, err := resolve(handle, name)
		if err != nil || address == 0 {
			return api{}, ErrMissingSymbol
		}
		addresses[name] = address
	}
	var bound api
	registrations := []struct {
		name string
		ptr  any
	}{
		{"mpv_create", &bound.create},
		{"mpv_set_option_string", &bound.setOptionString},
		{"mpv_initialize", &bound.initialize},
		{"mpv_terminate_destroy", &bound.terminateDestroy},
		{"mpv_command", &bound.command},
		{"mpv_set_property", &bound.setProperty},
		{"mpv_get_property_string", &bound.getPropertyString},
		{"mpv_wait_event", &bound.waitEvent},
		{"mpv_wakeup", &bound.wakeup},
		{"mpv_free", &bound.free},
		{"mpv_render_context_create", &bound.renderContextCreate},
		{"mpv_render_context_render", &bound.renderContextRender},
		{"mpv_render_context_update", &bound.renderContextUpdate},
		{"mpv_render_context_report_swap", &bound.renderContextReportSwap},
		{"mpv_render_context_set_update_callback", &bound.renderContextSetUpdateCB},
		{"mpv_render_context_free", &bound.renderContextFree},
	}
	for _, registration := range registrations {
		purego.RegisterFunc(registration.ptr, addresses[registration.name])
	}
	return bound, nil
}

type Library struct {
	handle uintptr
	api    api
	loader moduleLoader
	mu     sync.Mutex
	closed bool
	cores  int
}

func Open(absolutePath, expectedSHA256 string) (*Library, error) {
	return openLibrary(absolutePath, expectedSHA256, windowsLoader{})
}

func (library *Library) New() (*Mpv, error) {
	if library == nil {
		return nil, ErrLibraryClosed
	}
	library.mu.Lock()
	defer library.mu.Unlock()
	if library.closed {
		return nil, ErrLibraryClosed
	}
	handle := library.api.create()
	if handle == 0 {
		return nil, ErrPlayerFailed
	}
	library.cores++
	return &Mpv{library: library, handle: handle}, nil
}

func (library *Library) Close() error {
	if library == nil {
		return nil
	}
	library.mu.Lock()
	defer library.mu.Unlock()
	if library.closed {
		return nil
	}
	if library.cores != 0 {
		return ErrLibraryInUse
	}
	if err := library.loader.Free(library.handle); err != nil {
		return ErrLibraryLoad
	}
	library.closed = true
	library.handle = 0
	return nil
}

type Mpv struct {
	library *Library
	handle  uintptr
	closed  bool
	mu      sync.Mutex
	renders int
}

func (mpv *Mpv) SetOptionString(name, value string) error {
	if mpv == nil || mpv.isClosed() {
		return ErrPlayerClosed
	}
	if !validCString(name) || !validCString(value) {
		return ErrPlayerFailed
	}
	return resultError(mpv.library.api.setOptionString(mpv.handle, name, value))
}

func (mpv *Mpv) Initialize() error {
	if mpv == nil || mpv.isClosed() {
		return ErrPlayerClosed
	}
	return resultError(mpv.library.api.initialize(mpv.handle))
}

func (mpv *Mpv) Command(command []string) error {
	if mpv == nil || mpv.isClosed() || len(command) == 0 {
		return ErrPlayerFailed
	}
	strings := make([][]byte, len(command)+1)
	argv := make([]*byte, len(command)+1)
	for index, value := range command {
		if !validCString(value) {
			return ErrPlayerFailed
		}
		strings[index] = append([]byte(value), 0)
		argv[index] = &strings[index][0]
	}
	err := mpv.library.api.command(mpv.handle, &argv[0])
	runtime.KeepAlive(strings)
	runtime.KeepAlive(argv)
	return resultError(err)
}

func (mpv *Mpv) SetProperty(name string, format Format, value any) error {
	if mpv == nil || mpv.isClosed() || !validCString(name) {
		return ErrPlayerClosed
	}
	switch format {
	case FormatString:
		text, ok := value.(string)
		if !ok || !validCString(text) {
			return ErrPlayerFailed
		}
		buffer := append([]byte(text), 0)
		pointer := unsafe.Pointer(&buffer[0])
		err := mpv.library.api.setProperty(mpv.handle, name, int32(format), unsafe.Pointer(&pointer))
		runtime.KeepAlive(buffer)
		return resultError(err)
	case FormatFlag:
		flag, ok := value.(bool)
		if !ok {
			return ErrPlayerFailed
		}
		var raw int32
		if flag {
			raw = 1
		}
		return resultError(mpv.library.api.setProperty(mpv.handle, name, int32(format), unsafe.Pointer(&raw)))
	case FormatDouble:
		number, ok := value.(float64)
		if !ok {
			return ErrPlayerFailed
		}
		return resultError(mpv.library.api.setProperty(mpv.handle, name, int32(format), unsafe.Pointer(&number)))
	default:
		return ErrPlayerFailed
	}
}

func (mpv *Mpv) GetPropertyString(name string) string {
	if mpv == nil || mpv.isClosed() || !validCString(name) {
		return ""
	}
	value := mpv.library.api.getPropertyString(mpv.handle, name)
	if value == nil {
		return ""
	}
	defer mpv.library.api.free(value)
	return copyCString(value, 4096)
}

func (mpv *Mpv) WaitEvent(timeout float64) *Event {
	if mpv == nil || mpv.isClosed() || math.IsNaN(timeout) || math.IsInf(timeout, 0) || timeout < 0 {
		return nil
	}
	pointer := mpv.library.api.waitEvent(mpv.handle, timeout)
	if pointer == nil {
		return nil
	}
	raw := (*rawEvent)(pointer)
	event := &Event{EventID: EventID(raw.EventID), ReplyUserdata: raw.ReplyUserdata, Data: raw.Data}
	if raw.Error < 0 {
		event.Error = ErrPlayerFailed
	}
	return event
}

func (mpv *Mpv) Wakeup() {
	if mpv != nil && !mpv.isClosed() {
		mpv.library.api.wakeup(mpv.handle)
	}
}

func (mpv *Mpv) TerminateDestroy() error {
	if mpv == nil {
		return nil
	}
	mpv.mu.Lock()
	if mpv.closed {
		mpv.mu.Unlock()
		return nil
	}
	if mpv.renders != 0 {
		mpv.mu.Unlock()
		return ErrLibraryInUse
	}
	mpv.closed = true
	handle := mpv.handle
	mpv.handle = 0
	mpv.mu.Unlock()
	mpv.library.api.terminateDestroy(handle)
	mpv.library.mu.Lock()
	mpv.library.cores--
	mpv.library.mu.Unlock()
	return nil
}

func (mpv *Mpv) NewRenderContextGL(getProcAddress func(string) uintptr) (*RenderContext, error) {
	if mpv == nil || mpv.isClosed() || getProcAddress == nil {
		return nil, ErrPlayerClosed
	}
	if ensureCallbacks() != nil {
		return nil, ErrPlayerFailed
	}
	context := &RenderContext{mpv: mpv, procAddress: getProcAddress, drained: make(chan struct{})}
	context.id = callbackSequence.Add(1)
	if context.id == 0 {
		context.id = callbackSequence.Add(1)
	}
	callbackRegistry.Lock()
	callbackRegistry.entries[context.id] = context
	callbackRegistry.Unlock()
	apiType := append([]byte("opengl"), 0)
	glParams := glInitParams{GetProcAddress: callbacks.proc, Context: uintptr(context.id)}
	params := []renderParam{
		{Type: renderParamAPIType, Data: unsafe.Pointer(&apiType[0])},
		{Type: renderParamOpenGLInitParams, Data: unsafe.Pointer(&glParams)},
		{},
	}
	var handle uintptr
	code := mpv.library.api.renderContextCreate(&handle, mpv.handle, unsafe.Pointer(&params[0]))
	runtime.KeepAlive(params)
	runtime.KeepAlive(glParams)
	runtime.KeepAlive(apiType)
	if code < 0 || handle == 0 {
		callbackRegistry.Lock()
		delete(callbackRegistry.entries, context.id)
		callbackRegistry.Unlock()
		return nil, ErrPlayerFailed
	}
	context.handle = handle
	context.mpv.mu.Lock()
	context.mpv.renders++
	context.mpv.mu.Unlock()
	mpv.library.api.renderContextSetUpdateCB(context.handle, callbacks.update, uintptr(context.id))
	return context, nil
}

func (mpv *Mpv) isClosed() bool {
	if mpv == nil {
		return true
	}
	mpv.mu.Lock()
	defer mpv.mu.Unlock()
	return mpv.closed
}

func (library *Library) String() string { return "mpvwin.Library{redacted}" }

func (library *Library) GoString() string { return library.String() }

var callbacks struct {
	sync.Once
	proc   uintptr
	update uintptr
	err    error
}

var callbackRegistry = struct {
	sync.Mutex
	entries map[uint64]*RenderContext
}{entries: make(map[uint64]*RenderContext)}

var callbackSequence atomic.Uint64

func ensureCallbacks() error {
	callbacks.Do(func() {
		defer func() {
			if recover() != nil {
				callbacks.err = ErrPlayerFailed
			}
		}()
		callbacks.proc = purego.NewCallback(func(_ purego.CDecl, context uintptr, name unsafe.Pointer) uintptr {
			entry := acquireCallback(context)
			if entry == nil {
				return 0
			}
			defer releaseCallback(entry)
			return callbackResult(func() uintptr { return entry.procAddress(copyCString(name, 256)) })
		})
		callbacks.update = purego.NewCallback(func(_ purego.CDecl, context uintptr) uintptr {
			entry := acquireCallback(context)
			if entry == nil {
				return 0
			}
			defer releaseCallback(entry)
			entry.mu.Lock()
			callback := entry.update
			entry.mu.Unlock()
			callbackResult(func() uintptr {
				if callback != nil {
					callback()
				}
				return 0
			})
			return 0
		})
	})
	return callbacks.err
}

func callbackResult(callback func() uintptr) (result uintptr) {
	defer func() {
		if recover() != nil {
			result = 0
		}
	}()
	return callback()
}

func acquireCallback(id uintptr) *RenderContext {
	callbackRegistry.Lock()
	entry := callbackRegistry.entries[uint64(id)]
	if entry != nil {
		entry.mu.Lock()
		if entry.closing {
			entry.mu.Unlock()
			entry = nil
		} else {
			entry.active++
			entry.mu.Unlock()
		}
	}
	callbackRegistry.Unlock()
	return entry
}

func releaseCallback(entry *RenderContext) {
	entry.mu.Lock()
	entry.active--
	if entry.closing && entry.active == 0 {
		entry.drainedOnce.Do(func() { close(entry.drained) })
	}
	entry.mu.Unlock()
}

func validCString(value string) bool {
	for index := range value {
		if value[index] == 0 {
			return false
		}
	}
	return true
}

func copyCString(pointer unsafe.Pointer, maximum int) string {
	if pointer == nil {
		return ""
	}
	bytes := unsafe.Slice((*byte)(pointer), maximum)
	for index, value := range bytes {
		if value == 0 {
			return string(bytes[:index])
		}
	}
	return ""
}
