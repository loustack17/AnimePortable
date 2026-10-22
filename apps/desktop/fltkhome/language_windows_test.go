//go:build windows

// SPDX-License-Identifier: MPL-2.0

package fltkhome

import (
	"testing"

	"animeportable/apps/desktop/backend"
	"animeportable/apps/desktop/uitext"
	fltk "github.com/pwiecz/go-fltk"
)

// useChineseAfter puts the process back in Traditional Chinese after a test that switches,
// because the language is process-wide and other tests read Chinese text.
func useChineseAfter(t *testing.T) {
	t.Cleanup(func() { uitext.Use("zh-TW") })
}

func TestNativeLanguageSwitchesEveryShownTextAtOnceAndIsSaved(t *testing.T) {
	ui, store := openSettingsPage(t, settingsOf("light", "enabled", "enabled", "enabled"))
	useChineseAfter(t)
	for _, step := range []struct {
		segment                   int
		code, home, settings, row string
	}{
		{1, "en", "Home", "Settings", "Next episode"},
		{2, "ja", "ホーム", "設定", "次のエピソード"},
		{0, "zh-TW", "首頁", "設定", "自動下一集"},
	} {
		ui.languageControl.buttons[step.segment].TakeFocus()
		pressKey(t, ui.window, vkReturn)
		pumpSearchEvents(t, func() bool {
			saved := store.savedSettings()
			return uitext.T() == uitext.For(step.code) && len(saved) > 0 && saved[len(saved)-1].Language == step.code
		})
		text := uitext.T()
		if got := ui.navigation[0].Label(); got != step.home {
			t.Errorf("%s: navigation shows %q, want %q", step.code, got, step.home)
		}
		if ui.sectionTitle.Label() != step.settings || ui.contentHint.Label() != text.SettingsHint {
			t.Errorf("%s: heading %q / %q", step.code, ui.sectionTitle.Label(), ui.contentHint.Label())
		}
		toggle := ui.settingsToggles[0]
		if toggle.title.String() != step.row || toggle.button.Label() != step.row {
			t.Errorf("%s: first toggle reads %q, labelled %q", step.code, toggle.title.String(), toggle.button.Label())
		}
		if got := ui.themeControl.buttons[0].Label(); got != text.ThemeSystem {
			t.Errorf("%s: theme segment %q, want %q", step.code, got, text.ThemeSystem)
		}
		if got := ui.proxyTest.status(); got != text.ProxyIdle {
			t.Errorf("%s: proxy status %q, want %q", step.code, got, text.ProxyIdle)
		}
		if got := ui.languageControl.selected(); got != step.segment {
			t.Errorf("%s: selected segment %d", step.code, got)
		}
		if !ui.languageControl.buttons[step.segment].HasFocus() {
			t.Errorf("%s: focus left the language control", step.code)
		}
	}
}

func TestNativeStoredLanguageAppliesAtStartup(t *testing.T) {
	stored := backend.DefaultSettings()
	stored.Language = "ja"
	ui, _ := openSettingsPage(t, stored)
	useChineseAfter(t)
	if uitext.T() != uitext.For("ja") || ui.navigation[0].Label() != "ホーム" || ui.languageControl.selected() != 2 {
		t.Fatalf("after startup: navigation %q, selected %d", ui.navigation[0].Label(), ui.languageControl.selected())
	}
}

// TestNativeFixedButtonsFitTheirLabelsInEveryLanguage checks the buttons whose width does not
// follow their label.
func TestNativeFixedButtonsFitTheirLabelsInEveryLanguage(t *testing.T) {
	ui, _ := openSettingsPage(t, settingsOf("light", "enabled", "enabled", "enabled"))
	useChineseAfter(t)
	for _, code := range uitext.LanguageCodes {
		ui.useLanguage(code)
		text := uitext.T()
		labelled := map[string]*fltk.Button{
			"search": ui.searchButton, "search back": ui.searchBack, "results previous": ui.searchPrevious,
			"results next": ui.searchNext, "detail continue": ui.previewContinue, "detail retry": ui.previewRetry,
			"episodes previous": ui.previewPrevious, "episodes next": ui.previewNext, "browse": ui.browseButton,
			"home retry": ui.retryButton, "following update": ui.followingRefresh, "history reload": ui.historyRetry,
			"schedule refresh": ui.scheduleRefresh,
		}
		for index, button := range ui.navigation {
			labelled[text.Sections[index]] = button
		}
		for name, button := range labelled {
			requireFits(t, code, name, button, button.Label())
		}
		for _, back := range []string{text.BackToHome, text.BackToFollowing, text.BackToSchedule, text.BackToResults} {
			requireFits(t, code, "detail back", ui.previewBack, back)
		}
		// Home's 繼續播放 buttons are as wide and styled as its retry button.
		requireFits(t, code, "home resume", ui.retryButton, text.ContinuePlaying)
	}
}

// requireFits measures label as styleButton draws it: bold 15, inset 18 left and 8 right.
func requireFits(t *testing.T, code, name string, button *fltk.Button, label string) {
	t.Helper()
	if width := boldTextWidth(15, label); width > button.W()-26 {
		t.Errorf("%s: %s label %q is %d px wide, button %d px", code, name, label, width, button.W())
	}
}
