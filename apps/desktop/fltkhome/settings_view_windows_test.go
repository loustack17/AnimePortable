//go:build windows

// SPDX-License-Identifier: MPL-2.0

package fltkhome

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"animeportable/apps/desktop/backend"
	"animeportable/apps/desktop/uitext"
)

// settingsPageService keeps the settings in a settingsTestService under the Home test service
// and answers the acting rows from a settingsActionsFake.
type settingsPageService struct {
	*homeRefreshTestService
	store   *settingsTestService
	actions *settingsActionsFake
}

func (service settingsPageService) Settings(ctx context.Context) (backend.Settings, error) {
	return service.store.Settings(ctx)
}

func (service settingsPageService) SaveSettings(ctx context.Context, settings backend.Settings) error {
	return service.store.SaveSettings(ctx, settings)
}

func (service settingsPageService) TestProxy(ctx context.Context, input backend.Settings) (backend.ProxyTestResult, error) {
	return service.actions.TestProxy(ctx, input)
}

func (service settingsPageService) CheckForUpdate(ctx context.Context) (backend.UpdateInfo, error) {
	return service.actions.CheckForUpdate(ctx)
}

func (service settingsPageService) DataLocation(ctx context.Context) (backend.DataLocation, error) {
	return service.actions.DataLocation(ctx)
}

func (service settingsPageService) MoveDataLocation(ctx context.Context, request backend.MoveDataLocationRequest) (backend.MoveDataLocationResult, error) {
	return service.actions.MoveDataLocation(ctx, request)
}

// openSettingsPage starts the UI with stored settings, as startup does, and shows 設定.
func openSettingsPage(t *testing.T, stored backend.Settings) (*view, *settingsTestService) {
	t.Helper()
	ui, store, _ := openSettingsPageWith(t, stored, &settingsActionsFake{})
	return ui, store
}

func openSettingsPageWith(t *testing.T, stored backend.Settings, actions *settingsActionsFake) (*view, *settingsTestService, *settingsActionsFake) {
	t.Helper()
	store := &settingsTestService{settings: stored}
	ui := newHomeRefreshFixture(t, settingsPageService{&homeRefreshTestService{}, store, actions})
	ui.finishStartup(nil, stored, nil)
	ui.showSection(settingsSection)
	// A real notice would block the test; a test that expects one replaces this.
	ui.tasks.notice = func(_, message string) { t.Errorf("unexpected notice %q", message) }
	return ui, store, actions
}

// settingsOf is the stored settings with the four older choices set and every other field at
// its resolved default, as the backend returns them.
func settingsOf(appearance, next, play, resume string) backend.Settings {
	settings := backend.DefaultSettings()
	settings.Appearance, settings.AutoplayNext, settings.AutoPlay, settings.ResumePlayback = appearance, next, play, resume
	return settings
}

// toggleStates lists each toggle row's value and whether the row is enabled.
func toggleStates(ui *view) (on, enabled []bool) {
	for _, toggle := range ui.settingsToggles {
		on = append(on, toggle.on())
		enabled = append(enabled, toggle.button.IsActive())
	}
	return on, enabled
}

func waitForSaves(t *testing.T, store *settingsTestService, count int) []backend.Settings {
	t.Helper()
	pumpSearchEvents(t, func() bool { return len(store.savedSettings()) >= count })
	return store.savedSettings()
}

// segmentStates lists whether each segment of control is the selected one and whether it is enabled.
func segmentStates(control *segmentedControl) (selected, enabled []bool) {
	for index, segment := range control.buttons {
		selected = append(selected, control.selected() == index)
		enabled = append(enabled, segment.IsActive())
	}
	return selected, enabled
}

func segmentLabels(control *segmentedControl) []string {
	var labels []string
	for _, segment := range control.buttons {
		labels = append(labels, segment.Label())
	}
	return labels
}

// focusStops lists, top to bottom, the widgets Tab and the arrow keys can reach now.
func focusStops(ui *view) []focusTarget {
	var stops []focusTarget
	for _, row := range ui.settingsRows {
		if row.target == nil || !row.isShown() {
			continue
		}
		if target := row.target(); target != nil && target.IsActive() {
			stops = append(stops, target)
		}
	}
	return stops
}

func isFocused(target focusTarget) bool {
	return target.(interface{ HasFocus() bool }).HasFocus()
}

func TestNativeSettingsPageReflectsStoredSettings(t *testing.T) {
	fakeAppsUseLightTheme(t, 1, nil)
	stored := settingsOf("dark", "disabled", "enabled", "disabled")
	stored.PlaybackSpeed, stored.HardwareDecoding, stored.ProxyMode = 150, "disabled", "system"
	ui, _ := openSettingsPage(t, stored)
	if len(ui.settingsToggles) != 4 || !ui.settingsPage.Visible() {
		t.Fatalf("settings page shows %d toggles, visible %v", len(ui.settingsToggles), ui.settingsPage.Visible())
	}
	var titles, headings []string
	for _, toggle := range ui.settingsToggles {
		titles = append(titles, toggle.button.Label())
	}
	for _, heading := range ui.settingsHeadings {
		headings = append(headings, heading.Label())
	}
	if want := []string{"自動下一集", "自動播放", "從上次位置繼續播放", "硬體解碼"}; !slices.Equal(titles, want) {
		t.Fatalf("toggle titles = %v, want %v", titles, want)
	}
	if want := []string{"外觀", "播放", "網路", "資料", "關於"}; !slices.Equal(headings, want) {
		t.Fatalf("section headings = %v, want %v", headings, want)
	}
	for control, want := range map[*segmentedControl][]string{
		ui.themeControl: {"跟隨系統", "暗色", "明亮"},
		ui.proxyControl: {"不使用", "系統", "手動"},
	} {
		if got := segmentLabels(control); !slices.Equal(got, want) {
			t.Fatalf("segments = %v, want %v", got, want)
		}
	}
	for control, want := range map[*segmentedControl][]bool{
		ui.themeControl: {false, true, false},
		ui.proxyControl: {false, true, false},
	} {
		selected, enabled := segmentStates(control)
		if !slices.Equal(selected, want) {
			t.Fatalf("selected segments = %v, want %v", selected, want)
		}
		if slices.Contains(enabled, false) {
			t.Fatalf("segments enabled = %v, want all enabled", enabled)
		}
	}
	on, enabled := toggleStates(ui)
	// 自動播放 keeps its stored value while it is disabled.
	if want := []bool{false, true, false, false}; !slices.Equal(on, want) {
		t.Fatalf("toggle values = %v, want %v", on, want)
	}
	if want := []bool{true, false, true, true}; !slices.Equal(enabled, want) {
		t.Fatalf("toggles enabled = %v, want %v", enabled, want)
	}
	if ui.proxyField.card.Visible() || ui.proxyField.input.Visible() {
		t.Fatal("the proxy address shows while the proxy is not manual")
	}
	if !ui.dark {
		t.Fatal("a stored dark appearance did not show dark")
	}
}

func TestNativeSettingsPageSelectsTheStoredSegmentOnOpen(t *testing.T) {
	for index, appearance := range []string{"system", "dark", "light"} {
		fakeAppsUseLightTheme(t, 1, nil)
		ui, _ := openSettingsPage(t, settingsOf(appearance, "enabled", "enabled", "enabled"))
		if got := ui.themeControl.selected(); got != index {
			t.Fatalf("appearance %q selects segment %d, want %d", appearance, got, index)
		}
	}
}

func TestNativeSettingsPageShowsDefaultsDisabledAndSavesNothingWhenStoredSettingsCannotLoad(t *testing.T) {
	fakeAppsUseLightTheme(t, 0, nil)
	stored := settingsOf("light", "disabled", "disabled", "disabled")
	store := &settingsTestService{settings: stored}
	ui := newHomeRefreshFixture(t, settingsPageService{&homeRefreshTestService{}, store, &settingsActionsFake{}})
	ui.finishStartup(nil, backend.Settings{}, errors.New("unreadable"))
	ui.showSection(settingsSection)
	// Nothing is saved, so 硬體解碼 must not claim a change either (a real notice would block).
	ui.tasks.notice = func(_, message string) { t.Errorf("unexpected notice %q", message) }
	if got := ui.message.Label(); got != "無法載入設定，暫時使用預設值。" {
		t.Fatalf("load failure message = %q", got)
	}
	if got := ui.settingsNotice.Label(); got != uitext.T().SettingsLoadFailed || !ui.settingsNotice.Visible() {
		t.Fatalf("settings page notice = %q, visible %v", got, ui.settingsNotice.Visible())
	}
	if !ui.dark {
		t.Fatal("the theme did not render from the defaults (follow system, dark system)")
	}
	on, enabled := toggleStates(ui)
	if want := []bool{true, true, true, true}; !slices.Equal(on, want) {
		t.Fatalf("default toggle values = %v, want %v", on, want)
	}
	if slices.Contains(enabled, true) {
		t.Fatalf("toggles enabled = %v, want all disabled", enabled)
	}
	selected, segmentEnabled := segmentStates(ui.themeControl)
	if want := []bool{true, false, false}; !slices.Equal(selected, want) {
		t.Fatalf("default selected segment = %v, want %v", selected, want)
	}
	for _, control := range []*segmentedControl{ui.themeControl, ui.proxyControl} {
		if _, segmentEnabled = segmentStates(control); slices.Contains(segmentEnabled, true) {
			t.Fatalf("segments enabled = %v, want all disabled", segmentEnabled)
		}
	}
	if ui.proxyTest.button.IsActive() {
		t.Fatal("the connection test stayed enabled with defaults on the page")
	}
	// The rows that act without saving a setting stay usable: the data folder and update check.
	if !ui.dataChange.button.IsActive() || !ui.updateCheck.button.IsActive() {
		t.Fatal("the data folder or update rows were disabled by a settings load failure")
	}

	ui.chooseTheme("light")
	ui.chooseTheme("dark")
	ui.chooseProxyMode("manual")
	ui.toggleAutoplayNext()
	ui.toggleAutoPlay()
	ui.toggleResumePlayback()
	ui.toggleHardwareDecoding()
	if err := ui.settingsWriter.close(); err != nil {
		t.Fatalf("close() = %v", err)
	}
	if saved := store.savedSettings(); len(saved) != 0 || store.settings != stored {
		t.Fatalf("changing after a failed load saved %#v and left %#v, want nothing saved and %#v kept", saved, store.settings, stored)
	}
	if !ui.dark || ui.settings != backend.DefaultSettings() {
		t.Fatal("changing after a failed load changed what the page shows")
	}
}

func TestNativeSettingsKeysToggleAndSaveAtOnce(t *testing.T) {
	fakeAppsUseLightTheme(t, 1, nil)
	ui, store := openSettingsPage(t, settingsOf("light", "enabled", "enabled", "enabled"))
	ui.settingsToggles[0].button.TakeFocus()
	pressKey(t, ui.window, vkReturn)
	saved := waitForSaves(t, store, 1)
	if saved[0] != settingsOf("light", "disabled", "enabled", "enabled") {
		t.Fatalf("Enter on 自動下一集 saved %#v", saved[0])
	}
	if _, enabled := toggleStates(ui); enabled[1] {
		t.Fatal("自動播放 stayed enabled with 自動下一集 off")
	}
	if !ui.settingsToggles[0].button.HasFocus() {
		t.Fatal("toggling moved focus away")
	}

	// Space is the same as Enter; the disabled 自動播放 is skipped by Tab.
	pressKey(t, ui.window, vkTab)
	pumpSearchEvents(t, func() bool { return ui.settingsToggles[2].button.HasFocus() })
	pressKey(t, ui.window, vkSpace)
	saved = waitForSaves(t, store, 2)
	if saved[1] != settingsOf("light", "disabled", "enabled", "disabled") {
		t.Fatalf("Space on 從上次位置繼續播放 saved %#v", saved[1])
	}
	if on, _ := toggleStates(ui); on[2] {
		t.Fatal("the switch did not turn off")
	}
}

func TestNativeSettingsHardwareDecodingToggleSaves(t *testing.T) {
	ui, store := openSettingsPage(t, settingsOf("light", "enabled", "enabled", "enabled"))
	toggle := ui.settingsToggles[3]
	if toggle.title.String() != "硬體解碼" || !toggle.on() {
		t.Fatalf("hardware decoding row = %q on %v, want 硬體解碼 on by default", toggle.title.String(), toggle.on())
	}
	var notices []string
	ui.tasks.notice = func(title, message string) { notices = append(notices, title+": "+message) }
	toggle.button.TakeFocus()
	pressKey(t, ui.window, vkSpace)
	saved := waitForSaves(t, store, 1)
	if saved[0].HardwareDecoding != "disabled" || saved[0].PlaybackSpeed != 100 {
		t.Fatalf("toggling hardware decoding saved %#v", saved[0])
	}
	pressKey(t, ui.window, vkReturn)
	saved = waitForSaves(t, store, 2)
	if saved[1].HardwareDecoding != "enabled" {
		t.Fatalf("toggling again saved %#v", saved[1])
	}
	// Each change says it applies when the player opens next, so nobody restarts the app for it.
	want := "硬體解碼: 已變更硬體解碼，下次開啟播放器時生效，不需要重新開啟程式。"
	if len(notices) != 2 || notices[0] != want || notices[1] != want {
		t.Fatalf("notices = %q, want %q twice", notices, want)
	}
}

func TestNativeSettingsEachSegmentSavesItsValueAndAppliesTheTheme(t *testing.T) {
	fakeAppsUseLightTheme(t, 0, nil) // the Windows theme is dark
	ui, store := openSettingsPage(t, settingsOf("light", "enabled", "enabled", "enabled"))
	for step, want := range []struct {
		segment int
		value   string
		dark    bool
	}{{1, "dark", true}, {2, "light", false}, {0, "system", true}} {
		ui.themeControl.buttons[want.segment].TakeFocus()
		pressKey(t, ui.window, vkReturn)
		saved := waitForSaves(t, store, step+1)
		if got := saved[step].Appearance; got != want.value {
			t.Fatalf("segment %d saved appearance %q, want %q", want.segment, got, want.value)
		}
		if ui.dark != want.dark || ui.themeControl.selected() != want.segment {
			t.Fatalf("segment %d: dark %v, selected %d; want dark %v", want.segment, ui.dark, ui.themeControl.selected(), want.dark)
		}
	}
	if err := ui.settingsWriter.close(); err != nil {
		t.Fatalf("close() = %v", err)
	}
	if store.settings != settingsOf("system", "enabled", "enabled", "enabled") {
		t.Fatalf("stored settings = %#v", store.settings)
	}
}

func TestNativeSettingsSystemSegmentReadsTheWindowsThemeWhenChosen(t *testing.T) {
	fakeAppsUseLightTheme(t, 1, nil)
	ui, _ := openSettingsPage(t, settingsOf("dark", "enabled", "enabled", "enabled"))
	if !ui.dark {
		t.Fatal("a stored dark appearance did not show dark")
	}
	ui.chooseTheme("system")
	if ui.dark {
		t.Fatal("choosing 跟隨系統 did not read the light Windows theme")
	}
	// The Windows theme changes while the app runs: it is read again only when chosen.
	fakeAppsUseLightTheme(t, 0, nil)
	if ui.dark {
		t.Fatal("the theme changed without choosing a segment")
	}
	ui.chooseTheme("system")
	if !ui.dark || ui.settings.Appearance != "system" {
		t.Fatalf("choosing 跟隨系統 again: dark %v, appearance %q; want the dark Windows theme", ui.dark, ui.settings.Appearance)
	}
	// A fixed choice stops following the system.
	ui.chooseTheme("light")
	if ui.dark || ui.settings.Appearance != "light" {
		t.Fatalf("choosing 明亮: dark %v, appearance %q", ui.dark, ui.settings.Appearance)
	}
}

func TestNativeSettingsThemeKeysMoveBetweenSegmentsAndSelect(t *testing.T) {
	fakeAppsUseLightTheme(t, 1, nil)
	ui, store := openSettingsPage(t, settingsOf("system", "enabled", "enabled", "enabled"))
	segments := ui.themeControl.buttons
	ui.focusSettings()
	if !segments[0].HasFocus() {
		t.Fatal("entering 設定 did not focus the selected segment")
	}
	pressKey(t, ui.window, vkRight)
	pumpSearchEvents(t, func() bool { return segments[1].HasFocus() })
	pressKey(t, ui.window, vkReturn)
	saved := waitForSaves(t, store, 1)
	if saved[0].Appearance != "dark" || !ui.dark || !segments[1].HasFocus() {
		t.Fatalf("Enter on 暗色 saved %#v, dark %v, focus kept %v", saved[0], ui.dark, segments[1].HasFocus())
	}
	pressKey(t, ui.window, vkRight)
	pumpSearchEvents(t, func() bool { return segments[2].HasFocus() })
	pressKey(t, ui.window, vkRight)
	pumpFor(50 * time.Millisecond)
	if !segments[2].HasFocus() {
		t.Fatal("Right on the last segment moved focus")
	}
	pressKey(t, ui.window, vkSpace)
	saved = waitForSaves(t, store, 2)
	if saved[1].Appearance != "light" || ui.dark {
		t.Fatalf("Space on 明亮 saved %#v, dark %v", saved[1], ui.dark)
	}
	pressKey(t, ui.window, vkLeft)
	pumpSearchEvents(t, func() bool { return segments[1].HasFocus() })
	pressKey(t, ui.window, vkLeft)
	pumpSearchEvents(t, func() bool { return segments[0].HasFocus() })
	pressKey(t, ui.window, vkLeft)
	pumpSearchEvents(t, func() bool { return ui.navigation[settingsSection].HasFocus() })
}

func TestNativeSettingsSegmentedRowIsOneStopForUpDownAndTab(t *testing.T) {
	fakeAppsUseLightTheme(t, 1, nil)
	ui, _ := openSettingsPage(t, settingsOf("light", "enabled", "enabled", "enabled"))
	segments := ui.themeControl.buttons
	// 語言 follows 主題 and shows 繁體中文 selected.
	language := ui.languageControl.buttons[0]
	// Right from the navigation enters the group on the selected segment; Tab and Down leave it in one step.
	ui.navigation[settingsSection].TakeFocus()
	pressKey(t, ui.window, vkRight)
	pumpSearchEvents(t, func() bool { return segments[2].HasFocus() })
	pressKey(t, ui.window, vkTab)
	pumpSearchEvents(t, func() bool { return language.HasFocus() })
	pressKey(t, ui.window, vkUp)
	pumpSearchEvents(t, func() bool { return segments[2].HasFocus() })
	pressKey(t, ui.window, vkDown)
	pumpSearchEvents(t, func() bool { return language.HasFocus() })
	segments[1].TakeFocus()
	pressKey(t, ui.window, vkUp)
	pumpSearchEvents(t, func() bool { return ui.navigation[settingsSection].HasFocus() })
}

func TestNativeSettingsTabWalksEveryEnabledRowInOrder(t *testing.T) {
	fakeAppsUseLightTheme(t, 1, nil)
	// 自動播放 is disabled and the restore button does not apply, so Tab skips both.
	ui, _ := openSettingsPage(t, settingsOf("system", "disabled", "enabled", "enabled"))
	pumpSearchEvents(t, func() bool { return ui.tasks.dataKnown })
	stops := focusStops(ui)
	if ui.settingsToggles[1].button.IsActive() || ui.dataRestore.button.IsActive() {
		t.Fatal("a row that does not apply is enabled")
	}
	want := []focusTarget{
		ui.themeControl.buttons[0], ui.languageControl.buttons[0], ui.settingsToggles[0].button, ui.settingsToggles[2].button,
		ui.settingsToggles[3].button, ui.proxyControl.buttons[0], ui.proxyTest.button,
		ui.dataChange.button, ui.updateCheck.button, ui.projectOpen.button,
	}
	if len(stops) != len(want) {
		t.Fatalf("%d tab stops, want %d", len(stops), len(want))
	}
	for index := range want {
		if stops[index] != want[index] {
			t.Fatalf("tab stop %d is a different widget than expected", index)
		}
	}
	ui.navigation[settingsSection].TakeFocus()
	pressKey(t, ui.window, vkRight)
	pumpSearchEvents(t, func() bool { return isFocused(stops[0]) })
	for index := 1; index < len(stops); index++ {
		pressKey(t, ui.window, vkTab)
		pumpSearchEvents(t, func() bool { return isFocused(stops[index]) })
	}
	pressKey(t, ui.window, vkTab)
	pumpFor(50 * time.Millisecond)
	if !isFocused(stops[len(stops)-1]) {
		t.Fatal("Tab on the last row moved focus")
	}
	// Shift+Tab and Up are the same command; before the first row focus leaves for the navigation.
	for index := len(stops) - 2; index >= 0; index-- {
		ui.moveFocusFrom(slices.IndexFunc(ui.settingsRows, func(row *settingsRow) bool { return row.target != nil && row.target() == stops[index+1] }), -1)
		if !isFocused(stops[index]) {
			t.Fatalf("moving back from stop %d did not reach stop %d", index+1, index)
		}
	}
	ui.moveSettingsFocus(ui.themeControl.row, -1)
	if !ui.navigation[settingsSection].HasFocus() {
		t.Fatal("moving back from the first row did not reach the navigation")
	}
	ui.settingsToggles[0].button.TakeFocus()
	pressKey(t, ui.window, vkLeft)
	pumpSearchEvents(t, func() bool { return ui.navigation[settingsSection].HasFocus() })
}

func TestNativeSettingsEscapeGoesBackFromEveryKindOfRow(t *testing.T) {
	ui, _ := openSettingsPage(t, settingsOf("light", "enabled", "enabled", "enabled"))
	ui.chooseProxyMode("manual")
	for _, focus := range []focusTarget{ui.settingsToggles[0].button, ui.themeControl.buttons[1], ui.proxyField.input, ui.proxyTest.button} {
		ui.showSection(settingsSection)
		focus.TakeFocus()
		pressKey(t, ui.window, vkEscape)
		pumpSearchEvents(t, func() bool { return ui.selected == 0 && ui.navigation[0].HasFocus() })
		if ui.settingsPage.Visible() || ui.closing {
			t.Fatal("Escape left the settings page showing or closed the app")
		}
	}
}

func TestNativeSettingsQuickSavesFromDifferentTogglesAreBothStored(t *testing.T) {
	fakeAppsUseLightTheme(t, 1, nil)
	ui, store := openSettingsPage(t, settingsOf("light", "enabled", "enabled", "enabled"))
	store.writeStarted = make(chan struct{}, 1)
	store.continueWrite = make(chan struct{})
	ui.chooseTheme("dark")
	<-store.writeStarted
	ui.toggleResumePlayback()
	close(store.continueWrite)
	if err := ui.settingsWriter.close(); err != nil {
		t.Fatalf("close() = %v", err)
	}
	saved := store.savedSettings()
	want := settingsOf("dark", "enabled", "enabled", "disabled")
	if len(saved) == 0 || saved[len(saved)-1] != want {
		t.Fatalf("saved settings = %#v, want the last to hold both changes: %#v", saved, want)
	}
	if store.settings != want {
		t.Fatalf("stored settings = %#v, want %#v", store.settings, want)
	}
}

func TestNativeSettingsFailedSaveShowsTheMessage(t *testing.T) {
	ui, store := openSettingsPage(t, settingsOf("light", "enabled", "enabled", "enabled"))
	store.writeErr = errors.New("disk full")
	ui.toggleAutoPlay()
	pumpSearchEvents(t, func() bool { return ui.message.Label() == uitext.T().SettingsSaveFailed })

	// The next save that works clears it.
	ui.toggleAutoPlay()
	pumpSearchEvents(t, func() bool { return ui.message.Label() == "" })
}

func TestNativeSettingsSavedKeepsAnotherMessage(t *testing.T) {
	ui, store := openSettingsPage(t, settingsOf("light", "enabled", "enabled", "enabled"))
	pumpFor(200 * time.Millisecond)
	ui.message.SetLabel("其他訊息")
	ui.toggleAutoPlay()
	waitForSaves(t, store, 1)
	pumpFor(100 * time.Millisecond)
	if got := ui.message.Label(); got != "其他訊息" {
		t.Fatalf("a successful save changed the message to %q", got)
	}
}

func TestSegmentFocusRingContrastsWithTheSelectedFill(t *testing.T) {
	for name, colors := range map[string]palette{"light": lightPalette, "dark": darkPalette} {
		selected := segmentLookFor(colors, true, true, true)
		if selected.ring == selected.fill || selected.ring == colors.surface || selected.gap < 2 {
			t.Errorf("%s: focus ring on the selected segment (ring %v, fill %v, gap %d) does not stand out", name, selected.ring, selected.fill, selected.gap)
		}
		plain := segmentLookFor(colors, true, false, true)
		if plain.ring != colors.accent || plain.fill != colors.surface {
			t.Errorf("%s: focus ring on an unselected segment changed", name)
		}
		if selected.fill != colors.accent || segmentLookFor(colors, true, false, false).fill == selected.fill {
			t.Errorf("%s: selected and unselected segments are not told apart by fill", name)
		}
	}
}

func TestNativeSettingsProxyModeSavesAndShowsTheAddressRowOnlyForManual(t *testing.T) {
	ui, store := openSettingsPage(t, settingsOf("light", "enabled", "enabled", "enabled"))
	for step, want := range []struct {
		segment int
		mode    string
		address bool
	}{{2, "manual", true}, {1, "system", false}, {0, "none", false}, {2, "manual", true}} {
		ui.proxyControl.buttons[want.segment].TakeFocus()
		pressKey(t, ui.window, vkReturn)
		saved := waitForSaves(t, store, step+1)
		if saved[step].ProxyMode != want.mode {
			t.Fatalf("proxy segment %d saved %q, want %q", want.segment, saved[step].ProxyMode, want.mode)
		}
		if ui.proxyField.input.Visible() != want.address || ui.proxyField.card.Visible() != want.address {
			t.Fatalf("mode %q: address row visible %v, want %v", want.mode, ui.proxyField.input.Visible(), want.address)
		}
	}
	// The address row is a stop only while it shows.
	if !slices.Contains(focusStops(ui), focusTarget(ui.proxyField.input)) {
		t.Fatal("the manual proxy's address field is not reachable by keyboard")
	}
}

func TestNativeSettingsProxyAddressSavesOnEnterAndOnFocusLeave(t *testing.T) {
	stored := settingsOf("light", "enabled", "enabled", "enabled")
	stored.ProxyMode, stored.ProxyAddress = "manual", "127.0.0.1:1080"
	ui, store := openSettingsPage(t, stored)
	input := ui.proxyField.input
	if input.Value() != "127.0.0.1:1080" {
		t.Fatalf("the field shows %q, want the stored address", input.Value())
	}

	input.TakeFocus()
	input.SetValue("  127.0.0.1:7890 ")
	pressKey(t, ui.window, vkReturn)
	saved := waitForSaves(t, store, 1)
	if saved[0].ProxyAddress != "127.0.0.1:7890" || saved[0].ProxyMode != "manual" {
		t.Fatalf("Enter saved %#v", saved[0])
	}
	if !input.HasFocus() {
		t.Fatal("Enter moved focus out of the field")
	}

	input.SetValue("proxy.example:8080")
	ui.settingsToggles[0].button.TakeFocus()
	saved = waitForSaves(t, store, 2)
	if saved[1].ProxyAddress != "proxy.example:8080" {
		t.Fatalf("leaving the field saved %#v", saved[1])
	}

	// Unchanged text saves nothing.
	input.TakeFocus()
	ui.settingsToggles[0].button.TakeFocus()
	pumpFor(100 * time.Millisecond)
	if got := len(store.savedSettings()); got != 2 {
		t.Fatalf("an unchanged address saved again: %d saves", got)
	}
	// An empty address is allowed: the proxy is just not set.
	input.TakeFocus()
	input.SetValue("")
	pressKey(t, ui.window, vkReturn)
	if saved = waitForSaves(t, store, 3); saved[2].ProxyAddress != "" {
		t.Fatalf("clearing the address saved %#v", saved[2])
	}
}

func TestNativeSettingsProxyAddressRefusesABadAddressAndSaysSo(t *testing.T) {
	stored := settingsOf("light", "enabled", "enabled", "enabled")
	stored.ProxyMode, stored.ProxyAddress = "manual", "127.0.0.1:1080"
	ui, store := openSettingsPage(t, stored)
	input := ui.proxyField.input
	if uitext.T().ProxyAddressInvalid != "位址格式不正確，請輸入 主機名稱:連接埠。" {
		t.Fatalf("invalid address message = %q", uitext.T().ProxyAddressInvalid)
	}
	for _, bad := range []string{"http://127.0.0.1:7890", "127.0.0.1", "host:99999", "user:pw@host:80", "bad host:80"} {
		input.TakeFocus()
		input.SetValue(bad)
		pressKey(t, ui.window, vkReturn)
		pumpSearchEvents(t, func() bool { return ui.proxyField.invalid })
		ui.proxyField.invalid = false
	}
	ui.toggleResumePlayback()
	saved := waitForSaves(t, store, 1)
	if len(saved) != 1 || saved[0].ProxyAddress != "127.0.0.1:1080" || ui.settings.ProxyAddress != "127.0.0.1:1080" {
		t.Fatalf("a refused address reached the saved settings: %#v", saved)
	}
	input.TakeFocus()
	input.SetValue("host:80")
	pressKey(t, ui.window, vkReturn)
	waitForSaves(t, store, 2)
	if ui.proxyField.invalid {
		t.Fatal("a valid address left the error showing")
	}
}

func TestNativeSettingsProxyInputKeepsLeftRightAndLeavesWithUpDownAndTab(t *testing.T) {
	stored := settingsOf("light", "enabled", "enabled", "enabled")
	stored.ProxyMode, stored.ProxyAddress = "manual", "127.0.0.1:1080"
	ui, _ := openSettingsPage(t, stored)
	input := ui.proxyField.input
	input.TakeFocus()
	input.SetInsertPosition(3, 3)
	pressKey(t, ui.window, vkLeft)
	pumpFor(50 * time.Millisecond)
	if !input.HasFocus() || input.InsertPosition() != 2 {
		t.Fatalf("Left in the field: focus %v, cursor %d; want it to move the cursor", input.HasFocus(), input.InsertPosition())
	}
	pressKey(t, ui.window, vkDown)
	pumpSearchEvents(t, func() bool { return ui.proxyTest.button.HasFocus() })
	pressKey(t, ui.window, vkUp)
	pumpSearchEvents(t, func() bool { return input.HasFocus() })
	pressKey(t, ui.window, vkUp)
	pumpSearchEvents(t, func() bool { return ui.proxyControl.buttons[2].HasFocus() })
}

func TestNativeSettingsScrollKeepsTheFocusedRowInView(t *testing.T) {
	ui, _ := openSettingsPage(t, settingsOf("light", "enabled", "enabled", "enabled"))
	ui.window.Resize(0, 0, 760, 480)
	pumpFor(50 * time.Millisecond)
	if ui.window.W() != 760 || ui.window.H() != 480 {
		t.Skipf("window is %dx%d; the desktop may be too small for this test", ui.window.W(), ui.window.H())
	}
	scroll := ui.settingsScroll
	pumpSearchEvents(t, func() bool { return ui.tasks.dataKnown })
	last := ui.updateCheck.button
	if last.Y()+last.H() <= scroll.Y()+scroll.H() {
		t.Fatal("the page fits 760x480, so nothing proves scrolling")
	}
	stops := focusStops(ui)
	ui.focusSettings()
	for index, stop := range stops {
		if !isFocused(stop) {
			t.Fatalf("stop %d did not take focus", index)
		}
		if stop.Y() < scroll.Y() || stop.Y()+stop.H() > scroll.Y()+scroll.H() {
			t.Fatalf("stop %d is at %d..%d outside the scroll %d..%d", index, stop.Y(), stop.Y()+stop.H(), scroll.Y(), scroll.Y()+scroll.H())
		}
		if index+1 < len(stops) {
			pressKey(t, ui.window, vkTab)
			pumpSearchEvents(t, func() bool { return isFocused(stops[index+1]) })
		}
	}
	if scroll.YPosition() == 0 {
		t.Fatal("reaching the last row did not scroll")
	}
	// Going back to the first row shows the heading above it too.
	for index := len(stops) - 1; index > 0; index-- {
		ui.moveFocusFrom(slices.IndexFunc(ui.settingsRows, func(row *settingsRow) bool { return row.target != nil && row.target() == stops[index] }), -1)
	}
	if scroll.YPosition() != 0 || ui.settingsHeadings[0].Y() < scroll.Y() {
		t.Fatalf("the first row is focused with the scroll at %d and the first heading at %d", scroll.YPosition(), ui.settingsHeadings[0].Y())
	}
}

func TestNativeSettingsLayoutStaysInsideTheScrollWhenTheProxyRowComesAndGoes(t *testing.T) {
	ui, _ := openSettingsPage(t, settingsOf("light", "enabled", "enabled", "enabled"))
	ui.window.Resize(0, 0, 760, 480)
	pumpFor(50 * time.Millisecond)
	scroll := ui.settingsScroll
	before := ui.updateCheck.button.Y()
	ui.chooseProxyMode("manual")
	if got := ui.updateCheck.button.Y(); got != before+settingsRowHeight+settingsRowGap {
		t.Fatalf("the manual address row moved 檢查更新 by %d, want %d", got-before, settingsRowHeight+settingsRowGap)
	}
	scrollIntoView(scroll, ui.updateCheck.button)
	position := scroll.YPosition()
	ui.chooseProxyMode("none")
	if scroll.YPosition() > position {
		t.Fatalf("the scroll moved from %d to %d when content got shorter", position, scroll.YPosition())
	}
	last := ui.settingsEnd
	if last.Y()+last.H() < scroll.Y()+scroll.H() {
		t.Fatalf("the scroll shows empty space below the content: the end is at %d, the scroll ends at %d", last.Y()+last.H(), scroll.Y()+scroll.H())
	}
}

func TestNativeSettingsDisablingTheFocusedRowHandsFocusToThePreviousOne(t *testing.T) {
	ui, _ := openSettingsPage(t, settingsOf("light", "enabled", "enabled", "enabled"))
	ui.settingsToggles[1].button.TakeFocus()
	ui.toggleAutoplayNext() // 自動播放 now needs 自動下一集
	if ui.settingsToggles[1].button.IsActive() {
		t.Fatal("自動播放 stayed enabled with 自動下一集 off")
	}
	pumpSearchEvents(t, func() bool { return ui.settingsToggles[0].button.HasFocus() })
}

func TestNativeSettingsProxyAddressIsCommittedWhenThePageIsLeftOrTheWindowCloses(t *testing.T) {
	stored := settingsOf("light", "enabled", "enabled", "enabled")
	stored.ProxyMode, stored.ProxyAddress = "manual", "127.0.0.1:1080"

	ui, store := openSettingsPage(t, stored)
	ui.proxyField.input.SetValue("proxy.example:8080") // typed, no Enter, focus still in the field
	ui.showSection(0)
	if saved := waitForSaves(t, store, 1); saved[0].ProxyAddress != "proxy.example:8080" {
		t.Fatalf("leaving the page saved %#v", saved[0])
	}

	ui, store = openSettingsPage(t, stored)
	ui.proxyField.input.SetValue("closing.example:9090")
	ui.cancel()
	if saved := waitForSaves(t, store, 1); saved[0].ProxyAddress != "closing.example:9090" {
		t.Fatalf("closing the window saved %#v", saved[0])
	}

	// An invalid address is not saved on either path.
	ui, store = openSettingsPage(t, stored)
	ui.proxyField.input.SetValue("not valid")
	ui.showSection(0)
	pumpFor(100 * time.Millisecond)
	if got := len(store.savedSettings()); got != 0 {
		t.Fatalf("an invalid address saved %d times", got)
	}
}
