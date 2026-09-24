// SPDX-License-Identifier: MPL-2.0

package native

import (
	"context"

	"animeportable/apps/desktop/backend"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

type StartupService interface {
	HomeService
	Start(context.Context) error
}

func NewPortableWindow(application fyne.App, plan backend.PortablePlan, planErr error, service StartupService, ctx context.Context, cancel context.CancelFunc) fyne.Window {
	window := application.NewWindow("AnimePortable")
	window.Resize(fyne.NewSize(760, 480))
	var closeHome func()
	window.SetOnClosed(func() {
		cancel()
		if closeHome != nil {
			closeHome()
		}
	})
	message := widget.NewLabel("")
	message.Wrapping = fyne.TextWrapWord
	closeButton := widget.NewButton("關閉", window.Close)
	newButton := widget.NewButton("建立新的空白資料", nil)
	copyButton := widget.NewButton("複製既有資料", nil)
	setContent := func(buttons ...fyne.CanvasObject) {
		window.SetContent(container.NewPadded(container.NewVBox(
			widget.NewLabelWithStyle("AnimePortable", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
			message,
			container.NewHBox(buttons...),
		)))
	}
	if planErr != nil {
		message.SetText("無法使用這個資料夾，請將程式解壓到可寫入的位置後重新開啟。")
		setContent(closeButton)
		window.Canvas().Focus(closeButton)
		return window
	}
	var pending bool
	var start func(bool)
	start = func(copyExisting bool) {
		if pending || ctx.Err() != nil {
			return
		}
		pending = true
		newButton.Disable()
		copyButton.Disable()
		message.SetText("正在準備資料…")
		go func() {
			var err error
			var imported bool
			if copyExisting {
				err = plan.Import(ctx)
				imported = err == nil
			} else if !plan.TargetExists {
				err = plan.CreateFresh()
			}
			if err == nil {
				err = service.Start(ctx)
			}
			fyne.Do(func() {
				if ctx.Err() != nil {
					return
				}
				pending = false
				if err == nil {
					closeHome = ConfigureWindow(window, service)
					return
				}
				if imported {
					message.SetText("舊資料已複製到這個資料夾，但目前無法開啟。請關閉後重試；舊資料仍保留。")
					setContent(closeButton)
					window.Canvas().Focus(closeButton)
					return
				}
				if copyExisting && !backend.IsPortableImportConflict(err) {
					message.SetText("無法複製舊資料；舊資料沒有變更。你可以建立新的空白資料，或關閉程式。")
					newButton.Enable()
					setContent(newButton, closeButton)
					window.Canvas().Focus(newButton)
					return
				}
				if backend.IsPortableImportConflict(err) {
					message.SetText("資料夾內已有資料，不會覆蓋。請關閉後重新開啟。")
					setContent(closeButton)
					window.Canvas().Focus(closeButton)
					return
				}
				message.SetText("無法在此資料夾開啟資料，請確認解壓資料夾可寫入後重新開啟。")
				setContent(closeButton)
				window.Canvas().Focus(closeButton)
			})
		}()
	}
	newButton.OnTapped = func() { start(false) }
	copyButton.OnTapped = func() { start(true) }
	if plan.OfferImport {
		message.SetText("找到舊版資料。你可以複製一份到這個資料夾，或建立新的空白資料；舊資料都會保留。")
		setContent(newButton, copyButton, closeButton)
		window.Canvas().Focus(newButton)
	} else {
		message.SetText("正在開啟資料…")
		setContent(closeButton)
		start(false)
	}
	return window
}
