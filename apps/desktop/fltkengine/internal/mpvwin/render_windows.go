//go:build windows && amd64

// SPDX-License-Identifier: MPL-2.0

package mpvwin

import (
	"runtime"
	"sync"
	"unsafe"
)

const (
	RenderUpdateFrame           uint64 = 1
	renderParamAPIType          int32  = 1
	renderParamOpenGLInitParams int32  = 2
	renderParamOpenGLFBO        int32  = 3
	renderParamFlipY            int32  = 4
)

type RenderContext struct {
	id          uint64
	mpv         *Mpv
	handle      uintptr
	procAddress func(string) uintptr
	mu          sync.Mutex
	update      func()
	closing     bool
	active      int
	drained     chan struct{}
	drainedOnce sync.Once
	freeOnce    sync.Once
}

type renderParam struct {
	Type int32
	Data unsafe.Pointer
}

type glInitParams struct {
	GetProcAddress uintptr
	Context        uintptr
}

type glFBO struct {
	FBO, Width, Height, InternalFormat int32
}

func (context *RenderContext) SetUpdateCallback(callback func()) {
	if context == nil {
		return
	}
	context.mu.Lock()
	if !context.closing {
		context.update = callback
	}
	context.mu.Unlock()
}

func (context *RenderContext) Update() uint64 {
	if context == nil || context.isClosing() {
		return 0
	}
	return context.mpv.library.api.renderContextUpdate(context.handle)
}

func (context *RenderContext) RenderGL(fbo, width, height int, flipY bool) error {
	if context == nil || context.isClosing() || width <= 0 || height <= 0 {
		return ErrPlayerClosed
	}
	framebuffer := glFBO{FBO: int32(fbo), Width: int32(width), Height: int32(height)}
	flip := int32(0)
	if flipY {
		flip = 1
	}
	params := []renderParam{
		{Type: renderParamOpenGLFBO, Data: unsafe.Pointer(&framebuffer)},
		{Type: renderParamFlipY, Data: unsafe.Pointer(&flip)},
		{},
	}
	err := resultError(context.mpv.library.api.renderContextRender(context.handle, unsafe.Pointer(&params[0])))
	runtime.KeepAlive(params)
	runtime.KeepAlive(framebuffer)
	runtime.KeepAlive(flip)
	return err
}

func (context *RenderContext) ReportSwap() {
	if context != nil && !context.isClosing() {
		context.mpv.library.api.renderContextReportSwap(context.handle)
	}
}

func (context *RenderContext) Free() error {
	if context == nil {
		return nil
	}
	context.freeOnce.Do(func() {
		context.mpv.library.api.renderContextSetUpdateCB(context.handle, 0, 0)
		context.mu.Lock()
		context.closing = true
		if context.active == 0 {
			context.drainedOnce.Do(func() { close(context.drained) })
		}
		context.mu.Unlock()
		<-context.drained
		context.mpv.library.api.renderContextFree(context.handle)
		callbackRegistry.Lock()
		delete(callbackRegistry.entries, context.id)
		callbackRegistry.Unlock()
		context.mpv.mu.Lock()
		context.mpv.renders--
		context.mpv.mu.Unlock()
		context.handle = 0
	})
	return nil
}

func (context *RenderContext) isClosing() bool {
	context.mu.Lock()
	defer context.mu.Unlock()
	return context.closing
}
