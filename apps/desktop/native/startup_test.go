// SPDX-License-Identifier: MPL-2.0

package native

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"animeportable/adapters/persistence/sqlite"
	"animeportable/apps/desktop/backend"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
)

type startupTestService struct {
	mu         sync.Mutex
	startCalls int
	startErr   error
	started    chan struct{}
}

func (service *startupTestService) Start(context.Context) error {
	service.mu.Lock()
	service.startCalls++
	service.mu.Unlock()
	if service.started != nil {
		service.started <- struct{}{}
	}
	return service.startErr
}

func (*startupTestService) Library(context.Context) ([]backend.Anime, error) {
	return nil, nil
}

func (*startupTestService) History(context.Context) ([]backend.History, error) {
	return nil, nil
}

func (*startupTestService) Following(context.Context) ([]backend.Following, error) {
	return nil, nil
}

func (*startupTestService) Play(context.Context, backend.PlayRequest) error {
	return nil
}

func (service *startupTestService) calls() int {
	service.mu.Lock()
	defer service.mu.Unlock()
	return service.startCalls
}

type startupUIRequest struct {
	fn   func()
	done chan struct{}
}

type startupTestDriver struct {
	fyne.Driver
	requests chan startupUIRequest
}

func (driver *startupTestDriver) DoFromGoroutine(fn func(), wait bool) {
	request := startupUIRequest{fn: fn, done: make(chan struct{})}
	driver.requests <- request
	if wait {
		<-request.done
	}
}

type startupTestApp struct {
	fyne.App
	driver *startupTestDriver
}

func (application *startupTestApp) Driver() fyne.Driver {
	return application.driver
}

func newStartupTestApp(t *testing.T) *startupTestApp {
	t.Helper()
	base := test.NewApp()
	application := &startupTestApp{
		App:    base,
		driver: &startupTestDriver{Driver: base.Driver(), requests: make(chan startupUIRequest, 32)},
	}
	fyne.SetCurrentApp(application)
	t.Cleanup(application.Quit)
	return application
}

func TestPortableStartupOffersExplicitChoicesWithFocusOnNewAndNoAutomaticImport(t *testing.T) {
	application := newStartupTestApp(t)
	plan, legacyBytes := corruptLegacyPlan(t)
	service := &startupTestService{}
	ctx, cancel := context.WithCancel(context.Background())
	window := NewPortableWindow(application, plan, nil, service, ctx, cancel)
	t.Cleanup(window.Close)
	window.Show()

	newButton := findStartupButton(t, window, "建立新的空白資料")
	findStartupButton(t, window, "複製既有資料")
	if window.Canvas().Focused() != newButton {
		t.Fatalf("initial focus = %T (%v), want the new-profile choice", window.Canvas().Focused(), window.Canvas().Focused())
	}
	if markup := test.RenderToMarkup(window.Canvas()); markup == "" {
		t.Fatal("startup choices did not render")
	}
	time.Sleep(30 * time.Millisecond)
	if service.calls() != 0 {
		t.Fatalf("service started %d times before the owner chose an option", service.calls())
	}
	gotLegacy, err := os.ReadFile(plan.LegacyPath)
	if err != nil || string(gotLegacy) != string(legacyBytes) {
		t.Fatalf("legacy data changed before an explicit choice: bytes=%q err=%v", gotLegacy, err)
	}
	if _, err := os.Stat(plan.DataDir); !os.IsNotExist(err) {
		t.Fatalf("startup imported data before a choice: data directory stat error = %v", err)
	}
}

func TestPortableStartupNewChoiceStartsHomeInSameWindow(t *testing.T) {
	application := newStartupTestApp(t)
	plan, _ := corruptLegacyPlan(t)
	service := &startupTestService{started: make(chan struct{}, 1)}
	ctx, cancel := context.WithCancel(context.Background())
	window := NewPortableWindow(application, plan, nil, service, ctx, cancel)
	t.Cleanup(window.Close)
	window.Show()

	findStartupButton(t, window, "建立新的空白資料").OnTapped()
	select {
	case <-service.started:
	case <-time.After(time.Second):
		t.Fatal("new-profile choice did not start the service")
	}
	waitForHome(t, window, application.driver)
	drainStartupUI(application.driver)
}

func TestPortableStartupCopyFailurePreservesLegacyAndOffersEmptyStart(t *testing.T) {
	application := newStartupTestApp(t)
	plan, legacyBytes := corruptLegacyPlan(t)
	service := &startupTestService{started: make(chan struct{}, 1)}
	ctx, cancel := context.WithCancel(context.Background())
	window := NewPortableWindow(application, plan, nil, service, ctx, cancel)
	t.Cleanup(window.Close)
	window.Show()

	findStartupButton(t, window, "複製既有資料").OnTapped()
	waitForStartupLabel(t, window, application.driver, "無法複製舊資料")
	if service.calls() != 0 {
		t.Fatalf("service started after invalid legacy import: calls=%d", service.calls())
	}
	gotLegacy, err := os.ReadFile(plan.LegacyPath)
	if err != nil || string(gotLegacy) != string(legacyBytes) {
		t.Fatalf("failed import changed legacy data: bytes=%q err=%v", gotLegacy, err)
	}
	if _, err := os.Stat(plan.DatabasePath); !os.IsNotExist(err) {
		t.Fatalf("failed import created the portable database: stat error = %v", err)
	}
	newButton := findStartupButton(t, window, "建立新的空白資料")
	if newButton.Disabled() {
		t.Fatal("failed import did not leave the empty-profile choice available")
	}
	newButton.OnTapped()
	select {
	case <-service.started:
	case <-time.After(time.Second):
		t.Fatal("empty-profile fallback did not start the service")
	}
	waitForHome(t, window, application.driver)
	drainStartupUI(application.driver)
}

func TestPortableStartupDoesNotOfferEmptyProfileAfterSuccessfulCopyAndStartFailure(t *testing.T) {
	application := newStartupTestApp(t)
	plan, legacyBytes := validLegacyPlan(t)
	service := &startupTestService{startErr: errors.New("failed to open private database path"), started: make(chan struct{}, 1)}
	ctx, cancel := context.WithCancel(context.Background())
	window := NewPortableWindow(application, plan, nil, service, ctx, cancel)
	t.Cleanup(window.Close)
	window.Show()

	findStartupButton(t, window, "複製既有資料").OnTapped()
	select {
	case <-service.started:
	case <-time.After(time.Second):
		t.Fatal("successful import did not continue into service startup")
	}
	waitForStartupSettled(t, window, application.driver)

	viewText := startupText(window)
	if strings.Contains(viewText, "無法複製舊資料") {
		t.Errorf("successful copy was reported as a failed import: %q", viewText)
	}
	if hasStartupButton(window, "建立新的空白資料") {
		t.Error("empty-profile action was offered after a successful copy created the portable database")
	}
	if _, err := os.Stat(plan.DatabasePath); err != nil {
		t.Errorf("successful import did not leave its portable database: %v", err)
	}
	gotLegacy, err := os.ReadFile(plan.LegacyPath)
	if err != nil || string(gotLegacy) != string(legacyBytes) {
		t.Errorf("failed service startup changed the legacy profile: bytes=%q err=%v", gotLegacy, err)
	}
}

func TestPortableStartupErrorsAreGenericAndCloseCancels(t *testing.T) {
	t.Run("plan error", func(t *testing.T) {
		application := newStartupTestApp(t)
		privatePath := filepath.Join(t.TempDir(), "private folder", "animeportable.db")
		service := &startupTestService{}
		ctx, cancel := context.WithCancel(context.Background())
		window := NewPortableWindow(application, backend.PortablePlan{}, errors.New("cannot open "+privatePath), service, ctx, cancel)
		window.Show()
		assertStartupTextRedacted(t, window, privatePath)
		if service.calls() != 0 {
			t.Fatalf("service started despite plan failure: calls=%d", service.calls())
		}
		findStartupButton(t, window, "關閉").OnTapped()
		if ctx.Err() == nil {
			t.Fatal("closing plan-error window did not cancel startup context")
		}
	})

	t.Run("service start error", func(t *testing.T) {
		application := newStartupTestApp(t)
		plan, _ := corruptLegacyPlan(t)
		privatePath := filepath.Join(t.TempDir(), "private folder", "animeportable.db")
		service := &startupTestService{startErr: errors.New("failed to open " + privatePath), started: make(chan struct{}, 1)}
		ctx, cancel := context.WithCancel(context.Background())
		window := NewPortableWindow(application, plan, nil, service, ctx, cancel)
		window.Show()
		findStartupButton(t, window, "建立新的空白資料").OnTapped()
		select {
		case <-service.started:
		case <-time.After(time.Second):
			t.Fatal("new-profile choice did not call Start")
		}
		waitForStartupLabel(t, window, application.driver, "無法在此資料夾開啟資料")
		assertStartupTextRedacted(t, window, privatePath)
		findStartupButton(t, window, "關閉").OnTapped()
		if ctx.Err() == nil {
			t.Fatal("closing startup-error window did not cancel startup context")
		}
	})
}

func corruptLegacyPlan(t *testing.T) (backend.PortablePlan, []byte) {
	t.Helper()
	root := filepath.Join(t.TempDir(), "Anime Portable ü")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	dataDir := filepath.Join(root, "data")
	legacyPath := filepath.Join(t.TempDir(), "old config", "AnimePortable", "animeportable.db")
	if err := os.MkdirAll(filepath.Dir(legacyPath), 0o755); err != nil {
		t.Fatal(err)
	}
	legacyBytes := []byte("not a valid SQLite profile")
	if err := os.WriteFile(legacyPath, legacyBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	return backend.PortablePlan{
		Root:         root,
		DataDir:      dataDir,
		DatabasePath: filepath.Join(dataDir, "animeportable.db"),
		LegacyPath:   legacyPath,
		OfferImport:  true,
	}, legacyBytes
}

func validLegacyPlan(t *testing.T) (backend.PortablePlan, []byte) {
	t.Helper()
	root := filepath.Join(t.TempDir(), "Anime Portable ü")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	dataDir := filepath.Join(root, "data")
	legacyPath := filepath.Join(t.TempDir(), "old config", "AnimePortable", "animeportable.db")
	if err := os.MkdirAll(filepath.Dir(legacyPath), 0o755); err != nil {
		t.Fatal(err)
	}
	store, err := sqlite.Open(context.Background(), legacyPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	legacyBytes, err := os.ReadFile(legacyPath)
	if err != nil {
		t.Fatal(err)
	}
	return backend.PortablePlan{
		Root:         root,
		DataDir:      dataDir,
		DatabasePath: filepath.Join(dataDir, "animeportable.db"),
		LegacyPath:   legacyPath,
		OfferImport:  true,
	}, legacyBytes
}

func findStartupButton(t *testing.T, window fyne.Window, text string) *widget.Button {
	t.Helper()
	var found *widget.Button
	walkStartupObjects(window.Content(), func(object fyne.CanvasObject) {
		button, ok := object.(*widget.Button)
		if ok && button.Text == text {
			found = button
		}
	})
	if found == nil {
		t.Fatalf("button %q is not present", text)
	}
	return found
}

func hasStartupButton(window fyne.Window, text string) bool {
	found := false
	walkStartupObjects(window.Content(), func(object fyne.CanvasObject) {
		button, ok := object.(*widget.Button)
		if ok && button.Text == text {
			found = true
		}
	})
	return found
}

func countStartupButtons(window fyne.Window, labels []string) int {
	wanted := make(map[string]struct{}, len(labels))
	for _, label := range labels {
		wanted[label] = struct{}{}
	}
	seen := make(map[string]struct{}, len(labels))
	walkStartupObjects(window.Content(), func(object fyne.CanvasObject) {
		var text string
		switch button := object.(type) {
		case *widget.Button:
			text = button.Text
		case *navigationButton:
			text = button.Text
		}
		if text != "" {
			if _, found := wanted[text]; found {
				seen[text] = struct{}{}
			}
		}
	})
	return len(seen)
}

func waitForStartupLabel(t *testing.T, window fyne.Window, driver *startupTestDriver, contains string) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		dispatchStartupUI(driver)
		if strings.Contains(startupText(window), contains) {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("startup view never displayed %q; text = %q", contains, startupText(window))
}

func waitForStartupSettled(t *testing.T, window fyne.Window, driver *startupTestDriver) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		dispatchStartupUI(driver)
		if !strings.Contains(startupText(window), "正在準備資料") {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("startup view did not leave its pending state")
}

func waitForHome(t *testing.T, window fyne.Window, driver *startupTestDriver) {
	t.Helper()
	wanted := []string{"首頁", "時間表", "追蹤", "歷史紀錄", "搜尋", "設定"}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		dispatchStartupUI(driver)
		if countStartupButtons(window, wanted) == len(wanted) {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("Home navigation buttons = %d, want all six", countStartupButtons(window, wanted))
}

func dispatchStartupUI(driver *startupTestDriver) bool {
	select {
	case request := <-driver.requests:
		request.fn()
		close(request.done)
		return true
	default:
		return false
	}
}

func drainStartupUI(driver *startupTestDriver) {
	quietUntil := time.Now().Add(20 * time.Millisecond)
	for time.Now().Before(quietUntil) {
		if dispatchStartupUI(driver) {
			quietUntil = time.Now().Add(20 * time.Millisecond)
		} else {
			time.Sleep(time.Millisecond)
		}
	}
}

func startupText(window fyne.Window) string {
	var texts []string
	walkStartupObjects(window.Content(), func(object fyne.CanvasObject) {
		if label, ok := object.(*widget.Label); ok {
			texts = append(texts, label.Text)
		}
	})
	return strings.Join(texts, " ")
}

func assertStartupTextRedacted(t *testing.T, window fyne.Window, secret string) {
	t.Helper()
	if text := startupText(window); strings.Contains(text, secret) {
		t.Fatalf("startup view exposed a private path: %q", text)
	}
}

func walkStartupObjects(object fyne.CanvasObject, visit func(fyne.CanvasObject)) {
	visit(object)
	if container, ok := object.(*fyne.Container); ok {
		for _, child := range container.Objects {
			walkStartupObjects(child, visit)
		}
	}
}
