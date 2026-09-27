//go:build windows && amd64

// SPDX-License-Identifier: MPL-2.0

package mpvwin

import (
	"unsafe"
)

type Format int32

const (
	FormatNone Format = iota
	FormatString
	FormatOSDString
	FormatFlag
	FormatInt64
	FormatDouble
)

type EventID uint32

const (
	EventNone       EventID = 0
	EventStart      EventID = 6
	EventEnd        EventID = 7
	EventFileLoaded EventID = 8
)

type EndFileReason int32

const (
	EndFileEOF      EndFileReason = 0
	EndFileStop     EndFileReason = 2
	EndFileQuit     EndFileReason = 3
	EndFileError    EndFileReason = 4
	EndFileRedirect EndFileReason = 5
)

type Event struct {
	EventID       EventID
	Error         error
	ReplyUserdata uint64
	Data          unsafe.Pointer
}

type rawEvent struct {
	EventID       uint32
	Error         int32
	ReplyUserdata uint64
	Data          unsafe.Pointer
}

type rawEndFile struct {
	Reason           int32
	Error            int32
	EntryID          int64
	InsertID         int64
	InsertNumEntries int32
	Padding          int32
}

type EndFileEvent struct {
	Reason EndFileReason
	Error  error
}

func (event *Event) EndFile() EndFileEvent {
	if event == nil || event.Data == nil {
		return EndFileEvent{Error: ErrPlayerFailed}
	}
	data := (*rawEndFile)(event.Data)
	result := EndFileEvent{Reason: EndFileReason(data.Reason)}
	if data.Error < 0 {
		result.Error = ErrPlayerFailed
	}
	return result
}

func resultError(code int32) error {
	if code < 0 {
		return ErrPlayerFailed
	}
	return nil
}
