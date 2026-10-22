//go:build windows

// SPDX-License-Identifier: MPL-2.0

package fltkhome

import (
	"math"
	"strings"
	"testing"

	fltk "github.com/pwiecz/go-fltk"
)

func TestToneColorPicksTheColourOfTheResult(t *testing.T) {
	for name, colors := range map[string]palette{"light": lightPalette, "dark": darkPalette} {
		if toneColor(colors, toneSuccess) != colors.success || toneColor(colors, toneFailure) != colors.danger || toneColor(colors, toneNeutral) != colors.muted {
			t.Errorf("%s: toneColor does not map success, failure and neutral to their colours", name)
		}
		if colors.success == colors.danger || colors.success == colors.muted || colors.danger == colors.muted {
			t.Errorf("%s: success, failure and neutral share a colour", name)
		}
	}
}

func luminance(color fltk.Color) float64 {
	linear := func(channel uint8) float64 {
		value := float64(channel) / 255
		if value <= 0.03928 {
			return value / 12.92
		}
		return math.Pow((value+0.055)/1.055, 2.4)
	}
	return 0.2126*linear(uint8(color>>24)) + 0.7152*linear(uint8(color>>16)) + 0.0722*linear(uint8(color>>8))
}

func contrast(first, second fltk.Color) float64 {
	high, low := luminance(first), luminance(second)
	if low > high {
		high, low = low, high
	}
	return (high + 0.05) / (low + 0.05)
}

func TestResultAndDialogColoursMeetWCAGAAOnTheirBackground(t *testing.T) {
	for name, colors := range map[string]palette{"light": lightPalette, "dark": darkPalette} {
		for what, pair := range map[string][2]fltk.Color{
			"success on a row card": {colors.success, colors.surface},
			"failure on a row card": {colors.danger, colors.surface},
			"text on the primary":   {colors.onAccent, colors.accent},
			"dialog text":           {colors.ink, colors.surface},
			"secondary button text": {colors.ink, colors.soft},
		} {
			if ratio := contrast(pair[0], pair[1]); ratio < 4.5 {
				t.Errorf("%s %s: contrast %.2f, want at least 4.5", name, what, ratio)
			}
		}
	}
}

func TestDialogFocusKeys(t *testing.T) {
	state := dialogFocus{count: 3}
	if answer, done := state.key(keyBack); !done || answer != 0 {
		t.Fatalf("Esc = %d, %v; want the safe answer 0", answer, done)
	}
	if answer, done := state.key(keyActivate); !done || answer != 0 {
		t.Fatalf("Enter on the first button = %d, %v", answer, done)
	}
	for _, step := range []struct {
		command keyCommand
		focus   int
	}{{keyNext, 1}, {keyNext, 2}, {keyNext, 0}, {keyPrevious, 2}, {keyPrevious, 1}} {
		if _, done := state.key(step.command); done || state.focus != step.focus {
			t.Fatalf("command %v: focus %d, done %v; want focus %d", step.command, state.focus, done, step.focus)
		}
	}
	if answer, done := state.key(keyActivate); !done || answer != 1 {
		t.Fatalf("Enter on the second button = %d, %v", answer, done)
	}
	if answer, done := state.key(keyBack); !done || answer != 0 {
		t.Fatalf("Esc after moving focus = %d, %v; want 0", answer, done)
	}
	if _, done := state.key(keyNone); done {
		t.Fatal("an unrelated key ended the dialog")
	}
}

// The dialogs below are built but never shown, so nothing opens on screen.
func TestDialogKeysAnswerAndTheFirstAnswerStands(t *testing.T) {
	d := newDialog(nil, "資料位置", "要使用嗎？", false, "否", "使用")
	defer d.window.Destroy()
	if !d.handleKey(keyNext) || d.state.focus != 1 || d.done {
		t.Fatalf("Tab: focus %d, done %v", d.state.focus, d.done)
	}
	if !d.handleKey(keyActivate) || !d.done || d.answer != 1 {
		t.Fatalf("Enter on 使用: answer %d, done %v", d.answer, d.done)
	}
	d.handleKey(keyBack)
	d.finish(0)
	if d.answer != 1 {
		t.Fatalf("a later key changed the answer to %d", d.answer)
	}
	if d.handleKey(keyNone) {
		t.Fatal("an unrelated key was reported as used")
	}
}

func TestDialogEscapeAnswersTheSafeButtonEvenWithFocusElsewhere(t *testing.T) {
	d := newDialog(nil, "資料位置", "訊息", true, "取消", "覆蓋")
	defer d.window.Destroy()
	d.handleKey(keyNext)
	d.handleKey(keyBack)
	if !d.done || d.answer != 0 {
		t.Fatalf("Esc answered %d (done %v), want the first button", d.answer, d.done)
	}
}

func TestDialogOpensOnTheSafeButtonAndFitsItsMessage(t *testing.T) {
	long := strings.Repeat("資料夾位置很長", 20)
	d := newDialog(nil, "標題", long+"\n\n第二段", false, "結束", "使用預設位置")
	defer d.window.Destroy()
	if d.state.focus != 0 || len(d.buttons) != 2 {
		t.Fatalf("focus %d, %d buttons", d.state.focus, len(d.buttons))
	}
	if len(d.lines) < 4 {
		t.Fatalf("a long message wrapped into %d lines", len(d.lines))
	}
	first, last := d.buttons[0], d.buttons[1]
	if first.X()+first.W() > last.X() || last.X()+last.W() > d.width || last.Y()+last.H() > d.height {
		t.Fatalf("buttons overlap or leave the window: %d,%d,%d / %d,%d,%d in %dx%d",
			first.X(), first.W(), first.Y(), last.X(), last.W(), last.Y(), d.width, d.height)
	}
}

func TestMissingFolderQuestionNamesThePathAndExitsOnEscape(t *testing.T) {
	if message := missingFolderMessage(`E:\gone`); !strings.Contains(message, `E:\gone`) {
		t.Fatalf("message %q does not show the path", message)
	}
	if missingFolderLabels[0] != "結束" || missingFolderLabels[1] != "使用預設位置" {
		t.Fatalf("labels = %v; button 0 (Esc, close) must be 結束 and button 1 使用預設位置", missingFolderLabels)
	}
}

func TestDialogTallerThanTheScreenScrollsAndKeepsEveryLine(t *testing.T) {
	_, _, _, workH := fltk.ScreenWorkArea(0)
	message := strings.Repeat("很長的路徑\n", workH)
	d := newDialog(nil, "標題", message, false, "確定")
	defer d.window.Destroy()
	if d.height > workH || d.visible >= len(d.lines) {
		t.Fatalf("height %d of %d, %d of %d lines visible; want a capped dialog that scrolls", d.height, workH, d.visible, len(d.lines))
	}
	if !d.scroll(len(d.lines)) || d.offset != len(d.lines)-d.visible {
		t.Fatalf("scrolling to the end left offset %d of %d lines, %d visible", d.offset, len(d.lines), d.visible)
	}
	short := newDialog(nil, "標題", "短訊息", false, "確定")
	defer short.window.Destroy()
	if short.scroll(1) {
		t.Fatal("a short message claimed the scroll")
	}
}
