//go:build windows

// SPDX-License-Identifier: MPL-2.0

package fltkhome

import (
	"errors"
	"fmt"
	"strings"

	"animeportable/apps/desktop/backend"
	"animeportable/apps/desktop/uitext"
	"animeportable/internal/release"
	fltk "github.com/pwiecz/go-fltk"
	"golang.org/x/sys/windows"
)

// projectURL is the project page the 關於 section links to. It is a constant, so unlike an
// update link it needs no check before it is opened.
const projectURL = "https://github.com/loustack17/AnimePortable"

// settingsTasks is the state of the rows that act instead of saving a setting (proxy test,
// update check, data folder), and the system calls they make, which tests replace.
type settingsTasks struct {
	proxyTesting bool
	proxyStatus  localText
	proxyTone    statusTone

	updateChecking bool
	update         *backend.UpdateInfo
	updateStatus   localText

	projectStatus localText

	dataMoving     bool
	dataKnown      bool
	dataFailed     bool
	data           backend.DataLocation
	dataStatus     localText
	dataGeneration int

	openURL      func(url string) error
	chooseFolder func(start string) (string, bool)
	// choice asks with one button per option and returns the index of the one chosen. The first
	// option is the safe answer: Esc and closing the dialog return 0.
	choice func(message string, options ...string) int
	// notice shows a message under title with one OK button.
	notice func(title, message string)
	// restart closes the app through the normal shutdown path and asks main to start it again.
	restart func()
}

func (ui *view) useSettingsTaskDefaults() {
	ui.tasks = settingsTasks{
		proxyStatus: func(text *uitext.Strings) string { return text.ProxyIdle }, updateStatus: func(text *uitext.Strings) string { return text.UpdateIdle },
		dataStatus: func(text *uitext.Strings) string { return text.DataIdle }, projectStatus: func(text *uitext.Strings) string { return text.ProjectIdle },
		openURL: openInBrowser, chooseFolder: chooseFolderDialog,
		choice: func(message string, options ...string) int {
			return showDialog(ui.window, uitext.T().DataDialogTitle, message, ui.dark, options...)
		},
		notice: func(title, message string) {
			showDialog(ui.window, title, message, ui.dark, uitext.T().OK)
		},
		restart: ui.restartApp,
	}
}

// restartApp closes the window like the close button does (settings are saved, playback
// stops, the backend closes) and tells main to start the program again afterwards.
func (ui *view) restartApp() {
	ui.restarting = true
	if ui.onRestart != nil {
		ui.onRestart()
	}
	ui.cancel()
}

// runOffThread runs work away from the UI thread, then done on it unless the window closed.
func (ui *view) runOffThread(work, done func()) {
	go func() {
		work()
		fltk.Awake(func() {
			if ui.ctx.Err() == nil && !ui.closing {
				done()
			}
		})
	}()
}

func (ui *view) versionText() string { return "v" + release.Version }

func (ui *view) dataFolderText() string {
	switch {
	case ui.tasks.dataKnown:
		return ui.tasks.data.Dir
	case ui.tasks.dataFailed:
		return uitext.T().DataUnreadable
	}
	return uitext.T().DataLoading
}

// testProxy asks the backend to reach Anime1 through the proxy as it is on the page now, saved
// or not. The address is read from the field, so an address not yet committed is tested too.
func (ui *view) testProxy() {
	if ui.tasks.proxyTesting || ui.settingsLoadFailed || ui.closing {
		return
	}
	input := ui.settings
	if input.ProxyMode == "manual" {
		input.ProxyAddress = strings.TrimSpace(ui.proxyField.input.Value())
	}
	ui.tasks.proxyTesting, ui.tasks.proxyStatus, ui.tasks.proxyTone = true, func(text *uitext.Strings) string { return text.ProxyTesting }, toneNeutral
	ui.syncSettings()
	var result backend.ProxyTestResult
	var err error
	ui.runOffThread(func() { result, err = ui.service.TestProxy(ui.ctx, input) }, func() {
		ui.tasks.proxyTesting = false
		switch {
		case err == nil && result.OK:
			ui.tasks.proxyStatus, ui.tasks.proxyTone = func(text *uitext.Strings) string { return text.ProxyOK }, toneSuccess
		case err == nil && result.Reason != "":
			ui.tasks.proxyStatus, ui.tasks.proxyTone = proxyReasonText(result.Reason), toneFailure
		default:
			ui.tasks.proxyStatus, ui.tasks.proxyTone = func(text *uitext.Strings) string { return text.ProxyFailed }, toneFailure
		}
		ui.syncSettings()
	})
}

// proxyReasonText words a failed proxy test's reason code; an unknown code is a plain failure.
func proxyReasonText(reason string) localText {
	return func(text *uitext.Strings) string {
		switch reason {
		case backend.ReasonProxyUnreachable:
			return text.ProxyUnreachable
		case backend.ReasonProxyRefused:
			return text.ProxyRefused
		case backend.ReasonTimeout:
			return text.ProxyTimeout
		case backend.ReasonBadAddress:
			return text.ProxyBadAddress
		case backend.ReasonNoAddress:
			return text.ProxyNoAddress
		case backend.ReasonSystemUnusable:
			return text.ProxySystemUnusable
		default: // ReasonFailed and any reason this build does not know
			return text.ProxyFailed
		}
	}
}

// checkForUpdate asks GitHub for the newest release. It runs only when the button is used.
func (ui *view) checkForUpdate() {
	if ui.tasks.updateChecking || ui.closing {
		return
	}
	ui.tasks.updateChecking, ui.tasks.updateStatus, ui.tasks.update = true, func(text *uitext.Strings) string { return text.UpdateChecking }, nil
	ui.syncSettings()
	var info backend.UpdateInfo
	var err error
	ui.runOffThread(func() { info, err = ui.service.CheckForUpdate(ui.ctx) }, func() {
		ui.tasks.updateChecking = false
		switch {
		case err != nil:
			ui.tasks.updateStatus = func(text *uitext.Strings) string { return text.UpdateFailed }
		case info.Newer && info.Latest != "":
			ui.tasks.update = &info
			ui.tasks.updateStatus = func(text *uitext.Strings) string { return fmt.Sprintf(text.UpdateAvailable, info.Latest) }
		default:
			ui.tasks.updateStatus = func(text *uitext.Strings) string { return fmt.Sprintf(text.UpToDate, info.Current) }
		}
		ui.syncSettings()
	})
}

// openUpdatePage opens the release page of the update found. The backend already filtered
// the link; it is checked again, and only the re-serialised result is opened.
func (ui *view) openUpdatePage() {
	info := ui.tasks.update
	if info == nil {
		return
	}
	page, ok := release.PageURL(info.URL)
	if !ok || ui.tasks.openURL(page) != nil {
		ui.tasks.updateStatus = func(text *uitext.Strings) string { return text.UpdateOpenFailed }
		ui.syncSettings()
	}
}

// openProjectPage opens the project page in the default browser, by the same opener as the
// update page.
func (ui *view) openProjectPage() {
	if ui.tasks.openURL(projectURL) != nil {
		ui.tasks.projectStatus = func(text *uitext.Strings) string { return text.ProjectOpenFailed }
	} else {
		ui.tasks.projectStatus = func(text *uitext.Strings) string { return text.ProjectIdle }
	}
	ui.syncSettings()
}

// openInBrowser opens an https URL in the default browser. The caller has checked it.
func openInBrowser(target string) error {
	address, err := windows.UTF16PtrFromString(target)
	if err != nil {
		return err
	}
	return windows.ShellExecute(0, windows.StringToUTF16Ptr("open"), address, nil, nil, windows.SW_SHOWNORMAL)
}

// refreshDataLocation reads the configured data folder for the page. A move in progress owns
// the value, so a read that started before it or ends after it is dropped.
func (ui *view) refreshDataLocation() {
	if ui.tasks.dataMoving || ui.closing {
		return
	}
	generation := ui.tasks.dataGeneration
	var location backend.DataLocation
	var err error
	ui.runOffThread(func() { location, err = ui.service.DataLocation(ui.ctx) }, func() {
		if ui.tasks.dataGeneration != generation {
			return
		}
		ui.tasks.dataKnown, ui.tasks.dataFailed, ui.tasks.data = err == nil, err != nil, location
		ui.syncSettings()
	})
}

// changeDataLocation asks for a folder (unless restoring the default) and moves the data there.
func (ui *view) changeDataLocation(restoreDefault bool) {
	if ui.tasks.dataMoving || ui.closing {
		return
	}
	request := backend.MoveDataLocationRequest{RestoreDefault: restoreDefault}
	if !restoreDefault {
		target, ok := ui.tasks.chooseFolder(ui.tasks.data.Dir)
		if !ok {
			return
		}
		request.Target = target
	}
	ui.moveDataLocation(request)
}

func (ui *view) moveDataLocation(request backend.MoveDataLocationRequest) {
	ui.tasks.dataMoving, ui.tasks.dataStatus = true, func(text *uitext.Strings) string { return text.DataMoving }
	ui.tasks.dataGeneration++
	ui.syncSettings()
	var result backend.MoveDataLocationResult
	var err error
	ui.runOffThread(func() { result, err = ui.service.MoveDataLocation(ui.ctx, request) }, func() {
		ui.tasks.dataMoving = false
		ui.tasks.dataGeneration++
		switch {
		case err != nil && errors.Is(err, backend.ErrDataFolderInUse):
			ui.tasks.dataStatus = func(text *uitext.Strings) string { return text.DataInUse }
		case err != nil && errors.Is(err, backend.ErrInvalidInput):
			ui.tasks.dataStatus = func(text *uitext.Strings) string { return text.DataInvalid }
		case err != nil:
			ui.tasks.dataStatus = func(text *uitext.Strings) string { return text.DataFailed }
		case result.NeedsChoice:
			existing := ui.askExistingData(result.Dir)
			if existing != "" {
				request.Existing = existing
				ui.moveDataLocation(request)
				return
			}
			ui.tasks.dataStatus = func(text *uitext.Strings) string { return text.DataIdle }
		default:
			ui.tasks.data, ui.tasks.dataKnown, ui.tasks.dataFailed = backend.DataLocation{Dir: result.Dir, Custom: !request.RestoreDefault}, true, false
			ui.tasks.dataStatus = func(text *uitext.Strings) string { return text.DataMoved }
			ui.syncSettings()
			// The open database is now the old copy and the backend refuses writes, so the
			// program restarts at once to open the new one.
			notice := uitext.T().DataCopiedNotice
			if request.Existing == backend.ExistingUse {
				notice = uitext.T().DataSwitchedNotice
			}
			ui.tasks.notice(uitext.T().DataDialogTitle, notice)
			ui.tasks.restart()
			return
		}
		ui.syncSettings()
	})
}

// askExistingData asks what to do with a folder that already holds data. It runs inside the
// fltk.Awake callback of runOffThread: the dialog starts a nested event loop there, which is
// safe because the dialog is modal, so the page cannot be used until it returns. The first
// option of each dialog is the harmless answer, which Esc and closing the dialog give: closing
// the first one only moves on to the second, and closing the second one cancels. It returns the
// answer for MoveDataLocation, or "" for no move.
func (ui *view) askExistingData(dir string) string {
	if ui.tasks.choice(fmt.Sprintf(uitext.T().ExistingData, dir), uitext.T().No, uitext.T().UseExistingData) == 1 {
		return backend.ExistingUse
	}
	if ui.tasks.choice(uitext.T().OverwriteData, uitext.T().Cancel, uitext.T().OverwriteButton) == 1 {
		return backend.ExistingOverwrite
	}
	return ""
}

// chooseFolderDialog shows the Windows folder picker, starting in start when it is set.
func chooseFolderDialog(start string) (string, bool) {
	chooser := fltk.NewNativeFileChooser()
	defer chooser.Destroy()
	chooser.SetType(fltk.NativeFileChooser_BROWSE_DIRECTORY)
	chooser.SetTitle(uitext.T().ChooseFolder)
	chooser.SetOptions(fltk.NativeFileChooser_NEW_FOLDER)
	if start != "" {
		chooser.SetDirectory(start)
	}
	if chooser.Show() != 0 {
		return "", false
	}
	names := chooser.Filenames()
	if len(names) != 1 || names[0] == "" {
		return "", false
	}
	return names[0], true
}
