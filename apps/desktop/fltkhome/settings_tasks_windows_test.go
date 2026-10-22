//go:build windows

// SPDX-License-Identifier: MPL-2.0

package fltkhome

import (
	"context"
	"errors"
	"slices"
	"sync"
	"testing"
	"time"

	"animeportable/apps/desktop/backend"
	"animeportable/internal/release"
	fltk "github.com/pwiecz/go-fltk"
)

type moveReply struct {
	result backend.MoveDataLocationResult
	err    error
}

// settingsActionsFake answers the rows that act. While a channel from holdUntil is open,
// every call waits for it, so a test can look at the page while the call runs.
type settingsActionsFake struct {
	mu          sync.Mutex
	gate        chan struct{}
	proxyInputs []backend.Settings
	proxyResult backend.ProxyTestResult
	proxyErr    error
	update      backend.UpdateInfo
	updateErr   error
	updateCalls int
	location    backend.DataLocation
	locationErr error
	moves       []backend.MoveDataLocationRequest
	replies     []moveReply
}

func (fake *settingsActionsFake) hold(ctx context.Context) error {
	fake.mu.Lock()
	gate := fake.gate
	fake.mu.Unlock()
	if gate == nil {
		return nil
	}
	select {
	case <-gate:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// holdUntil makes later calls wait until the returned channel is closed.
func (fake *settingsActionsFake) holdUntil() chan struct{} {
	fake.mu.Lock()
	defer fake.mu.Unlock()
	fake.gate = make(chan struct{})
	return fake.gate
}

func (fake *settingsActionsFake) release() {
	fake.mu.Lock()
	defer fake.mu.Unlock()
	if fake.gate != nil {
		close(fake.gate)
		fake.gate = nil
	}
}

func (fake *settingsActionsFake) TestProxy(ctx context.Context, input backend.Settings) (backend.ProxyTestResult, error) {
	if err := fake.hold(ctx); err != nil {
		return backend.ProxyTestResult{}, err
	}
	fake.mu.Lock()
	defer fake.mu.Unlock()
	fake.proxyInputs = append(fake.proxyInputs, input)
	return fake.proxyResult, fake.proxyErr
}

func (fake *settingsActionsFake) CheckForUpdate(ctx context.Context) (backend.UpdateInfo, error) {
	if err := fake.hold(ctx); err != nil {
		return backend.UpdateInfo{}, err
	}
	fake.mu.Lock()
	defer fake.mu.Unlock()
	fake.updateCalls++
	return fake.update, fake.updateErr
}

func (fake *settingsActionsFake) DataLocation(context.Context) (backend.DataLocation, error) {
	fake.mu.Lock()
	defer fake.mu.Unlock()
	if fake.location == (backend.DataLocation{}) && fake.locationErr == nil {
		return backend.DataLocation{Dir: defaultTestDataDir}, nil
	}
	return fake.location, fake.locationErr
}

func (fake *settingsActionsFake) MoveDataLocation(ctx context.Context, request backend.MoveDataLocationRequest) (backend.MoveDataLocationResult, error) {
	if err := fake.hold(ctx); err != nil {
		return backend.MoveDataLocationResult{}, err
	}
	fake.mu.Lock()
	defer fake.mu.Unlock()
	fake.moves = append(fake.moves, request)
	if len(fake.replies) == 0 {
		return backend.MoveDataLocationResult{Dir: request.Target}, nil
	}
	reply := fake.replies[0]
	fake.replies = fake.replies[1:]
	return reply.result, reply.err
}

func (fake *settingsActionsFake) recordedMoves() []backend.MoveDataLocationRequest {
	fake.mu.Lock()
	defer fake.mu.Unlock()
	return slices.Clone(fake.moves)
}

func (fake *settingsActionsFake) recordedProxyInputs() []backend.Settings {
	fake.mu.Lock()
	defer fake.mu.Unlock()
	return slices.Clone(fake.proxyInputs)
}

const (
	releasePagePrefix  = "https://github.com/loustack17/AnimePortable/releases/"
	defaultTestDataDir = `D:\Apps\AnimePortable\data`
	chosenTestDataDir  = `F:\New data`
	moveDoneStatus     = "重新開啟中…"
)

func TestNativeSettingsProxyTestShowsProgressThenTheResultAndUsesUnsavedValues(t *testing.T) {
	stored := settingsOf("light", "enabled", "enabled", "enabled")
	stored.ProxyMode, stored.ProxyAddress = "manual", "127.0.0.1:1080"
	actions := &settingsActionsFake{proxyResult: backend.ProxyTestResult{OK: true}}
	ui, store, _ := openSettingsPageWith(t, stored, actions)
	actions.holdUntil()

	// The address is typed but not committed: the test uses it anyway, and saves nothing.
	ui.proxyField.input.SetValue(" 10.0.0.2:8080 ")
	ui.testProxy()
	if got := ui.tasks.proxyStatus; got != "連線中…" || !ui.tasks.proxyTesting || !ui.proxyTest.isBusy() || ui.tasks.proxyTone != toneNeutral {
		t.Fatalf("while testing: status %q, testing %v, busy %v, tone %v", got, ui.tasks.proxyTesting, ui.proxyTest.isBusy(), ui.tasks.proxyTone)
	}
	ui.testProxy() // a second press while it runs does nothing
	actions.release()
	pumpSearchEvents(t, func() bool { return !ui.tasks.proxyTesting })
	if got := ui.tasks.proxyStatus; got != "連線成功" || ui.proxyTest.isBusy() || ui.tasks.proxyTone != toneSuccess {
		t.Fatalf("after success: status %q, busy %v, tone %v", got, ui.proxyTest.isBusy(), ui.tasks.proxyTone)
	}
	inputs := actions.recordedProxyInputs()
	if len(inputs) != 1 || inputs[0].ProxyMode != "manual" || inputs[0].ProxyAddress != "10.0.0.2:8080" {
		t.Fatalf("TestProxy got %#v, want one call with the typed address", inputs)
	}
	if saved := store.savedSettings(); len(saved) != 0 {
		t.Fatalf("testing saved settings: %#v", saved)
	}
	if ui.settings.ProxyAddress != "127.0.0.1:1080" {
		t.Fatalf("testing changed the held address to %q", ui.settings.ProxyAddress)
	}
}

func TestNativeSettingsProxyTestUsesTheModeOnThePageWhenNotManual(t *testing.T) {
	stored := settingsOf("light", "enabled", "enabled", "enabled")
	stored.ProxyMode, stored.ProxyAddress = "system", "ignored:1"
	actions := &settingsActionsFake{proxyResult: backend.ProxyTestResult{OK: true}}
	ui, _, _ := openSettingsPageWith(t, stored, actions)
	ui.proxyField.input.SetValue("typed-but-hidden:2")
	ui.testProxy()
	pumpSearchEvents(t, func() bool { return !ui.tasks.proxyTesting })
	if inputs := actions.recordedProxyInputs(); len(inputs) != 1 || inputs[0].ProxyMode != "system" || inputs[0].ProxyAddress != "ignored:1" {
		t.Fatalf("TestProxy got %#v", inputs)
	}
}

func TestNativeSettingsProxyTestShowsTheReasonOrAGenericFailure(t *testing.T) {
	for name, want := range map[string]struct {
		result backend.ProxyTestResult
		err    error
		status string
	}{
		"reason":  {backend.ProxyTestResult{Reason: "無法連線到代理伺服器"}, nil, "無法連線到代理伺服器"},
		"timeout": {backend.ProxyTestResult{Reason: "逾時"}, nil, "逾時"},
		"error":   {backend.ProxyTestResult{}, backend.ErrUnavailable, "連線失敗"},
		"empty":   {backend.ProxyTestResult{}, nil, "連線失敗"},
	} {
		actions := &settingsActionsFake{proxyResult: want.result, proxyErr: want.err}
		ui, _, _ := openSettingsPageWith(t, settingsOf("light", "enabled", "enabled", "enabled"), actions)
		ui.proxyTest.button.TakeFocus()
		pressKey(t, ui.window, vkReturn)
		pumpSearchEvents(t, func() bool { return !ui.tasks.proxyTesting && ui.tasks.proxyStatus != proxyIdleStatus })
		if ui.tasks.proxyStatus != want.status || ui.tasks.proxyTone != toneFailure {
			t.Fatalf("%s: status = %q tone %v, want %q as a failure", name, ui.tasks.proxyStatus, ui.tasks.proxyTone, want.status)
		}
		if !ui.proxyTest.button.HasFocus() {
			t.Fatalf("%s: focus left the button", name)
		}
	}
}

func TestNativeSettingsUpdateCheckShowsTheResultsAndKeepsTheButtonBusyWhileRunning(t *testing.T) {
	actions := &settingsActionsFake{update: backend.UpdateInfo{Current: release.Version, Latest: release.Version}}
	ui, _, _ := openSettingsPageWith(t, settingsOf("light", "enabled", "enabled", "enabled"), actions)
	if got := ui.versionText(); got != "v"+release.Version {
		t.Fatalf("version row = %q", got)
	}
	if ui.updateOpen.button.Visible() {
		t.Fatal("the download button shows before any check")
	}
	actions.holdUntil()
	ui.updateCheck.button.TakeFocus()
	pressKey(t, ui.window, vkReturn)
	pumpSearchEvents(t, func() bool { return ui.tasks.updateChecking })
	if ui.tasks.updateStatus != "檢查中…" || !ui.updateCheck.isBusy() || !ui.updateCheck.button.HasFocus() {
		t.Fatalf("while checking: status %q, busy %v, focus %v", ui.tasks.updateStatus, ui.updateCheck.isBusy(), ui.updateCheck.button.HasFocus())
	}
	ui.checkForUpdate() // a second press while it runs does nothing
	actions.release()
	pumpSearchEvents(t, func() bool { return !ui.tasks.updateChecking })
	if want := "已是最新版本（v" + release.Version + "）"; ui.tasks.updateStatus != want {
		t.Fatalf("status = %q, want %q", ui.tasks.updateStatus, want)
	}
	if ui.updateOpen.button.Visible() {
		t.Fatal("the download button shows although this is the newest version")
	}
	if actions.updateCalls != 1 {
		t.Fatalf("%d update calls, want 1", actions.updateCalls)
	}

	actions.mu.Lock()
	actions.update = backend.UpdateInfo{Current: release.Version, Latest: "9.9.9", Newer: true, URL: releasePagePrefix + "tag/v9.9.9"}
	actions.mu.Unlock()
	ui.checkForUpdate()
	pumpSearchEvents(t, func() bool { return !ui.tasks.updateChecking && ui.tasks.update != nil })
	if ui.tasks.updateStatus != "有新版本 v9.9.9" || !ui.updateOpen.button.Visible() {
		t.Fatalf("newer: status %q, download button visible %v", ui.tasks.updateStatus, ui.updateOpen.button.Visible())
	}
	if !slices.Contains(focusStops(ui), focusTarget(ui.updateOpen.button)) {
		t.Fatal("the download button is not reachable by keyboard")
	}

	actions.mu.Lock()
	actions.updateErr = backend.ErrUnavailable
	actions.mu.Unlock()
	ui.checkForUpdate()
	pumpSearchEvents(t, func() bool { return !ui.tasks.updateChecking })
	if ui.tasks.updateStatus != "無法檢查更新，請稍後再試。" || ui.updateOpen.button.Visible() {
		t.Fatalf("failure: status %q, download button visible %v", ui.tasks.updateStatus, ui.updateOpen.button.Visible())
	}
}

func TestNativeSettingsDownloadPageOpensOnlyReleasePagesOfTheProject(t *testing.T) {
	goodURL := releasePagePrefix + "tag/v9.9.9"
	for name, want := range map[string]struct {
		url    string
		opened bool
	}{
		"release page":      {goodURL, true},
		"other repository":  {"https://github.com/someone/else/releases/tag/v1", false},
		"other host":        {"https://evil.example/loustack17/AnimePortable/releases/", false},
		"look-alike host":   {"https://github.com.evil.example/loustack17/AnimePortable/releases/", false},
		"http":              {"http://github.com/loustack17/AnimePortable/releases/tag/v1", false},
		"file":              {"file:///C:/Windows/System32/calc.exe", false},
		"prefix with space": {releasePagePrefix + "tag/v1 --x", false},
		"credentials":       {"https://user@github.com/loustack17/AnimePortable/releases/", false},
		"backslash":         {releasePagePrefix + `..\..\x`, false},
		"not a release":     {"https://github.com/loustack17/AnimePortable/issues/1", false},
		"double quote":      {releasePagePrefix + `tag/v1"`, false},
		"escaped quote":     {releasePagePrefix + "tag/v1%22", false},
		"dot dot":           {releasePagePrefix + "tag/../../x", false},
		"query":             {releasePagePrefix + "tag/v1?x=1", false},
	} {
		actions := &settingsActionsFake{update: backend.UpdateInfo{Current: release.Version, Latest: "9.9.9", Newer: true, URL: want.url}}
		ui, _, _ := openSettingsPageWith(t, settingsOf("light", "enabled", "enabled", "enabled"), actions)
		var opened []string
		ui.tasks.openURL = func(url string) error {
			opened = append(opened, url)
			return nil
		}
		ui.checkForUpdate()
		pumpSearchEvents(t, func() bool { return !ui.tasks.updateChecking && ui.tasks.update != nil })
		ui.updateOpen.button.TakeFocus()
		pressKey(t, ui.window, vkReturn)
		if want.opened {
			pumpSearchEvents(t, func() bool { return len(opened) == 1 })
			if opened[0] != goodURL {
				t.Fatalf("%s: opened %q", name, opened[0])
			}
			continue
		}
		pumpSearchEvents(t, func() bool { return ui.tasks.updateStatus == "無法開啟下載頁。" })
		if len(opened) != 0 {
			t.Fatalf("%s: opened %q although it is outside the project's release pages", name, opened)
		}
	}

	// The opener failing shows the same message.
	actions := &settingsActionsFake{update: backend.UpdateInfo{Current: release.Version, Latest: "9.9.9", Newer: true, URL: goodURL}}
	ui, _, _ := openSettingsPageWith(t, settingsOf("light", "enabled", "enabled", "enabled"), actions)
	ui.tasks.openURL = func(string) error { return errors.New("no browser") }
	ui.checkForUpdate()
	pumpSearchEvents(t, func() bool { return !ui.tasks.updateChecking && ui.tasks.update != nil })
	ui.openUpdatePage()
	if ui.tasks.updateStatus != "無法開啟下載頁。" {
		t.Fatalf("status after a failed open = %q", ui.tasks.updateStatus)
	}
}

func TestNativeSettingsDataFolderShowsTheCurrentFolderAndOnlyRestoresACustomOne(t *testing.T) {
	actions := &settingsActionsFake{location: backend.DataLocation{Dir: `E:\Anime data`, Custom: true}}
	ui, _, _ := openSettingsPageWith(t, settingsOf("light", "enabled", "enabled", "enabled"), actions)
	if got := ui.dataFolderText(); got != "讀取中…" && got != `E:\Anime data` {
		t.Fatalf("folder text before the read = %q", got)
	}
	pumpSearchEvents(t, func() bool { return ui.tasks.dataKnown })
	if got := ui.dataFolderText(); got != `E:\Anime data` || !ui.dataRestore.button.IsActive() {
		t.Fatalf("custom folder: text %q, restore enabled %v", got, ui.dataRestore.button.IsActive())
	}

	ui, _, _ = openSettingsPageWith(t, settingsOf("light", "enabled", "enabled", "enabled"), &settingsActionsFake{})
	pumpSearchEvents(t, func() bool { return ui.tasks.dataKnown })
	if ui.dataRestore.button.IsActive() {
		t.Fatal("restore is enabled for the default folder")
	}

	ui, _, _ = openSettingsPageWith(t, settingsOf("light", "enabled", "enabled", "enabled"), &settingsActionsFake{locationErr: backend.ErrUnavailable})
	pumpSearchEvents(t, func() bool { return ui.tasks.dataFailed })
	if ui.dataFolderText() != "無法讀取資料位置。" || ui.dataRestore.button.IsActive() {
		t.Fatalf("unreadable location: text %q, restore enabled %v", ui.dataFolderText(), ui.dataRestore.button.IsActive())
	}
}

// dataPageLog records what the data rows asked the system to do.
type dataPageLog struct {
	picked   []string // folder picker start folders
	notices  []string
	restarts int
}

// openDataPage opens 設定 over a default folder with a scripted folder picker, and a dialog
// that fails the test unless the test replaces it.
func openDataPage(t *testing.T, actions *settingsActionsFake) (*view, *dataPageLog) {
	t.Helper()
	ui, _, _ := openSettingsPageWith(t, settingsOf("light", "enabled", "enabled", "enabled"), actions)
	pumpSearchEvents(t, func() bool { return ui.tasks.dataKnown })
	log := &dataPageLog{}
	ui.tasks.chooseFolder = func(start string) (string, bool) {
		log.picked = append(log.picked, start)
		return chosenTestDataDir, true
	}
	ui.tasks.choice = func(message string, options ...string) int {
		t.Errorf("unexpected dialog %q", message)
		return 0
	}
	// A real notice would block the test, and a real restart would close the window.
	ui.tasks.notice = func(message string) { log.notices = append(log.notices, message) }
	ui.tasks.restart = func() { log.restarts++ }
	return ui, log
}

func TestNativeSettingsChangeDataLocationToAnEmptyFolder(t *testing.T) {
	actions := &settingsActionsFake{}
	ui, log := openDataPage(t, actions)
	actions.holdUntil()
	ui.dataChange.button.TakeFocus()
	pressKey(t, ui.window, vkReturn)
	pumpSearchEvents(t, func() bool { return ui.tasks.dataMoving })
	if !ui.dataChange.isBusy() || !ui.dataRestore.isBusy() || ui.tasks.dataStatus != "處理中…" {
		t.Fatalf("while moving: busy %v/%v, status %q", ui.dataChange.isBusy(), ui.dataRestore.isBusy(), ui.tasks.dataStatus)
	}
	ui.changeDataLocation(false) // pressing again while it runs does nothing
	actions.release()
	pumpSearchEvents(t, func() bool { return !ui.tasks.dataMoving })
	if ui.tasks.dataStatus != moveDoneStatus {
		t.Fatalf("status after the move = %q", ui.tasks.dataStatus)
	}
	if want := []string{defaultTestDataDir}; !slices.Equal(log.picked, want) {
		t.Fatalf("folder picker started in %v, want %v", log.picked, want)
	}
	if moves := actions.recordedMoves(); len(moves) != 1 || moves[0] != (backend.MoveDataLocationRequest{Target: chosenTestDataDir}) {
		t.Fatalf("moves = %#v", moves)
	}
	if got := ui.dataFolderText(); got != chosenTestDataDir || !ui.dataRestore.button.IsActive() {
		t.Fatalf("after the move: folder %q, restore enabled %v", got, ui.dataRestore.button.IsActive())
	}
	if !ui.dataChange.button.HasFocus() {
		t.Fatal("focus left the button")
	}
	// The open database is the old copy now, so the app tells the user and restarts, once.
	if want := []string{"資料已複製到新位置，程式將重新開啟。"}; !slices.Equal(log.notices, want) || log.restarts != 1 {
		t.Fatalf("notices %q, restarts %d; want one notice %q and one restart", log.notices, log.restarts, want)
	}
}

func TestNativeSettingsChangeDataLocationDoesNothingWhenThePickerIsCancelled(t *testing.T) {
	actions := &settingsActionsFake{}
	ui, _ := openDataPage(t, actions)
	ui.tasks.chooseFolder = func(string) (string, bool) { return "", false }
	status := ui.tasks.dataStatus
	ui.changeDataLocation(false)
	pumpFor(100 * time.Millisecond)
	if len(actions.recordedMoves()) != 0 || ui.tasks.dataMoving || ui.tasks.dataStatus != status {
		t.Fatalf("cancelling the picker moved %#v, moving %v, status %q", actions.recordedMoves(), ui.tasks.dataMoving, ui.tasks.dataStatus)
	}
}

func TestNativeSettingsChangeDataLocationAsksWhatToDoWithExistingData(t *testing.T) {
	needsChoice := moveReply{result: backend.MoveDataLocationResult{NeedsChoice: true, Dir: chosenTestDataDir}}
	for name, want := range map[string]struct {
		answers []int // what the dialogs return, in order; 0 is also what closing or Esc returns
		moves   []backend.MoveDataLocationRequest
		status  string
		folder  string
		notice  string // the one notice before the restart; empty when there is no restart
	}{
		"use the folder's data": {[]int{1}, []backend.MoveDataLocationRequest{
			{Target: chosenTestDataDir}, {Target: chosenTestDataDir, Existing: backend.ExistingUse}}, moveDoneStatus, chosenTestDataDir, "將改用該資料夾的資料，程式將重新開啟。"},
		"overwrite with the current data": {[]int{0, 1}, []backend.MoveDataLocationRequest{
			{Target: chosenTestDataDir}, {Target: chosenTestDataDir, Existing: backend.ExistingOverwrite}}, moveDoneStatus, chosenTestDataDir, "資料已複製到新位置，程式將重新開啟。"},
		"close both dialogs": {[]int{0, 0}, []backend.MoveDataLocationRequest{{Target: chosenTestDataDir}}, dataIdleStatus, defaultTestDataDir, ""},
	} {
		actions := &settingsActionsFake{replies: []moveReply{needsChoice}}
		ui, log := openDataPage(t, actions)
		var messages []string
		answers := slices.Clone(want.answers)
		ui.tasks.choice = func(message string, options ...string) int {
			if len(options) != 2 {
				t.Errorf("%s: a dialog with %d options would panic in go-fltk", name, len(options))
			}
			messages = append(messages, message)
			answer := answers[0]
			answers = answers[1:]
			return answer
		}
		ui.changeDataLocation(false)
		pumpSearchEvents(t, func() bool {
			return len(messages) == len(want.answers) && !ui.tasks.dataMoving && len(actions.recordedMoves()) == len(want.moves)
		})
		if got := actions.recordedMoves(); !slices.Equal(got, want.moves) {
			t.Fatalf("%s: moves = %#v, want %#v", name, got, want.moves)
		}
		if ui.tasks.dataStatus != want.status || ui.dataFolderText() != want.folder {
			t.Fatalf("%s: status %q, folder %q; want %q, %q", name, ui.tasks.dataStatus, ui.dataFolderText(), want.status, want.folder)
		}
		wantNotices, wantRestarts := []string(nil), 0
		if want.notice != "" {
			wantNotices, wantRestarts = []string{want.notice}, 1
		}
		if !slices.Equal(log.notices, wantNotices) || log.restarts != wantRestarts {
			t.Fatalf("%s: notices %q, restarts %d; want %q, %d", name, log.notices, log.restarts, wantNotices, wantRestarts)
		}
	}
}

func TestNativeSettingsRestoreDefaultLocation(t *testing.T) {
	actions := &settingsActionsFake{location: backend.DataLocation{Dir: `E:\Anime data`, Custom: true}}
	ui, log := openDataPage(t, actions)
	actions.mu.Lock()
	actions.replies = []moveReply{{result: backend.MoveDataLocationResult{Dir: defaultTestDataDir}}}
	actions.mu.Unlock()
	ui.dataRestore.button.TakeFocus()
	pressKey(t, ui.window, vkSpace)
	pumpSearchEvents(t, func() bool { return !ui.tasks.dataMoving && ui.tasks.dataStatus == moveDoneStatus })
	if len(log.picked) != 0 {
		t.Fatalf("restoring opened the folder picker: %v", log.picked)
	}
	if len(log.notices) != 1 || log.restarts != 1 {
		t.Fatalf("restoring: notices %q, restarts %d; want one notice and one restart", log.notices, log.restarts)
	}
	if moves := actions.recordedMoves(); len(moves) != 1 || moves[0] != (backend.MoveDataLocationRequest{RestoreDefault: true}) {
		t.Fatalf("moves = %#v", moves)
	}
	if got := ui.dataFolderText(); got != defaultTestDataDir || ui.dataRestore.button.IsActive() {
		t.Fatalf("after restoring: folder %q, restore enabled %v", got, ui.dataRestore.button.IsActive())
	}
	// The button that just became disabled had focus: focus moves to the row before it.
	pumpSearchEvents(t, func() bool { return ui.dataChange.button.HasFocus() })
}

func TestNativeSettingsDataMoveFailuresShowAMessageAndChangeNothing(t *testing.T) {
	for name, want := range map[string]struct {
		err    error
		status string
	}{
		"bad folder":  {backend.ErrInvalidInput, "無法使用這個資料夾，請改選其他資料夾。"},
		"live folder": {backend.ErrDataFolderInUse, "這是目前使用中的資料夾，請改選其他資料夾。"},
		"other":       {backend.ErrUnavailable, "無法變更資料位置，請重試。"},
	} {
		actions := &settingsActionsFake{replies: []moveReply{{err: want.err}}}
		ui, log := openDataPage(t, actions)
		ui.changeDataLocation(false)
		pumpSearchEvents(t, func() bool { return !ui.tasks.dataMoving && ui.tasks.dataStatus != dataMovingStatus })
		if ui.tasks.dataStatus != want.status || ui.dataFolderText() != defaultTestDataDir {
			t.Fatalf("%s: status %q, folder %q", name, ui.tasks.dataStatus, ui.dataFolderText())
		}
		if len(log.notices) != 0 || log.restarts != 0 {
			t.Fatalf("%s: a failed move restarted the app (notices %q, restarts %d)", name, log.notices, log.restarts)
		}
	}
}

func TestNativeSettingsRestartClosesTheWindowAndAsksMainToRestartOnce(t *testing.T) {
	ui, _, _ := openSettingsPageWith(t, settingsOf("light", "enabled", "enabled", "enabled"), &settingsActionsFake{})
	restarts, closes := 0, 0
	ui.onRestart = func() { restarts++ }
	ui.onClose = func() { closes++ }
	ui.restartApp()
	if restarts != 1 || !ui.closing {
		t.Fatalf("restart requests %d, closing %v; want one request and the normal close started", restarts, ui.closing)
	}
	pumpSearchEvents(t, func() bool { return closes == 1 })
	ui.restartApp() // a second call must not close or restart again
	pumpFor(100 * time.Millisecond)
	if closes != 1 {
		t.Fatalf("the close path ran %d times", closes)
	}
}

func TestNativeSettingsChangesAreIgnoredWhileTheDataMoves(t *testing.T) {
	actions := &settingsActionsFake{}
	ui, store, _ := openSettingsPageWith(t, settingsOf("light", "enabled", "enabled", "enabled"), actions)
	pumpSearchEvents(t, func() bool { return ui.tasks.dataKnown })
	ui.tasks.chooseFolder = func(string) (string, bool) { return chosenTestDataDir, true }
	ui.tasks.notice = func(string) {}
	ui.tasks.restart = func() {}
	actions.holdUntil()
	ui.changeDataLocation(false)
	pumpSearchEvents(t, func() bool { return ui.tasks.dataMoving })

	// The backend refuses writes from the moment the move starts, so the page does not queue any.
	ui.toggleAutoplayNext()
	pumpFor(100 * time.Millisecond)
	if saved := store.savedSettings(); len(saved) != 0 || ui.settings.AutoplayNext != "enabled" {
		t.Fatalf("a change during the move queued %d saves, autoplay next %q", len(saved), ui.settings.AutoplayNext)
	}
	actions.release()
	pumpSearchEvents(t, func() bool { return !ui.tasks.dataMoving })
}

func TestNativeSettingsRestartDoesNotSaveTheTypedProxyAddress(t *testing.T) {
	stored := settingsOf("light", "enabled", "enabled", "enabled")
	stored.ProxyMode, stored.ProxyAddress = "manual", "127.0.0.1:1080"
	ui, store, _ := openSettingsPageWith(t, stored, &settingsActionsFake{})
	closes := 0
	ui.onClose = func() { closes++ }
	ui.proxyField.input.SetValue("typed.example:8080")

	ui.restartApp() // the open database refuses writes after a data move
	pumpSearchEvents(t, func() bool { return closes == 1 })
	if saved := store.savedSettings(); len(saved) != 0 {
		t.Fatalf("restarting saved settings %#v", saved)
	}
}

func TestNativeSettingsProjectButtonOpensExactlyTheProjectPage(t *testing.T) {
	ui, _, _ := openSettingsPageWith(t, settingsOf("light", "enabled", "enabled", "enabled"), &settingsActionsFake{})
	if !ui.projectOpen.button.Visible() || ui.projectOpen.title != "GitHub 專案頁" {
		t.Fatalf("project row visible %v, title %q", ui.projectOpen.button.Visible(), ui.projectOpen.title)
	}
	var opened []string
	ui.tasks.openURL = func(url string) error {
		opened = append(opened, url)
		return nil
	}
	ui.projectOpen.button.TakeFocus()
	pressKey(t, ui.window, vkReturn)
	pumpSearchEvents(t, func() bool { return len(opened) == 1 })
	if want := "https://github.com/loustack17/AnimePortable"; opened[0] != want || projectURL != want {
		t.Fatalf("opened %q (constant %q), want %q", opened[0], projectURL, want)
	}

	ui.tasks.openURL = func(string) error { return errors.New("no browser") }
	ui.openProjectPage()
	if ui.tasks.projectStatus != projectOpenFailedStatus {
		t.Fatalf("status after a failed open = %q", ui.tasks.projectStatus)
	}
}

// dialogProbe answers the page's real dialogs from inside their own event loop and checks that
// each one is a top-level window on a screen. It builds dialogs the way showDialog does.
type dialogProbe struct {
	t       *testing.T
	ui      *view
	answers []int
	shown   int
}

func (probe *dialogProbe) show(message string, labels ...string) int {
	d := newDialog(probe.ui.window, dataDialogTitle, message, probe.ui.dark, labels...)
	answer := probe.answers[0]
	probe.answers = probe.answers[1:]
	// The page opens its dialogs from an Awake callback, so the answer comes from one too.
	go func() {
		time.Sleep(50 * time.Millisecond)
		fltk.Awake(func() {
			probe.check(d)
			d.finish(answer)
		})
	}()
	return d.run()
}

func (probe *dialogProbe) check(d *dialog) {
	probe.t.Helper()
	probe.shown++
	window := d.window
	if !window.IsShown() {
		probe.t.Errorf("dialog %d is not shown", probe.shown)
	}
	if window.XRoot() != window.X() || window.YRoot() != window.Y() {
		probe.t.Errorf("dialog %d is a child of another window: at %d,%d on screen but %d,%d in its parent",
			probe.shown, window.XRoot(), window.YRoot(), window.X(), window.Y())
	}
	workX, workY, workW, workH := fltk.ScreenWorkArea(fltk.ScreenNum(window.XRoot()+window.W()/2, window.YRoot()+window.H()/2))
	if window.XRoot() < workX || window.YRoot() < workY || window.XRoot()+window.W() > workX+workW || window.YRoot()+window.H() > workY+workH {
		probe.t.Errorf("dialog %d at %d,%d size %dx%d is outside the screen work area %d,%d %dx%d",
			probe.shown, window.XRoot(), window.YRoot(), window.W(), window.H(), workX, workY, workW, workH)
	}
}

// The page opens its dialogs from the fltk.Awake callback that ends the background move. They must
// still be real windows of their own, or the nested wait never ends and the app stops responding.
func TestNativeSettingsDataDialogsOpenAsTopLevelWindowsFromTheMoveCallback(t *testing.T) {
	needsChoice := moveReply{result: backend.MoveDataLocationResult{NeedsChoice: true, Dir: chosenTestDataDir}}
	for name, want := range map[string]struct {
		replies []moveReply
		answers []int
		dialogs int
		moves   int
	}{
		// Both questions, then the notice that precedes the restart.
		"folder with data": {[]moveReply{needsChoice}, []int{0, 1, 0}, 3, 2},
		"empty folder":     {nil, []int{0}, 1, 1},
	} {
		actions := &settingsActionsFake{replies: want.replies}
		ui, log := openDataPage(t, actions)
		probe := &dialogProbe{t: t, ui: ui, answers: want.answers}
		ui.tasks.choice = func(message string, options ...string) int { return probe.show(message, options...) }
		ui.tasks.notice = func(message string) {
			log.notices = append(log.notices, message)
			probe.show(message, dialogOKLabel)
		}
		ui.changeDataLocation(false)
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) && log.restarts == 0 {
			fltk.Wait(0.01)
		}
		if log.restarts != 1 || probe.shown != want.dialogs || len(actions.recordedMoves()) != want.moves {
			t.Fatalf("%s: restarts %d, dialogs %d, moves %d; want 1, %d, %d", name, log.restarts, probe.shown, len(actions.recordedMoves()), want.dialogs, want.moves)
		}
	}
}
