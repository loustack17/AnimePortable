// SPDX-License-Identifier: MPL-2.0

package main

import (
	"context"
	"log"

	"animeportable/apps/desktop/backend"
	"animeportable/apps/desktop/native"
	"fyne.io/fyne/v2/app"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	service := backend.New()
	if err := service.Start(ctx); err != nil {
		cancel()
		log.Fatal(err)
	}
	defer func() {
		cancel()
		if err := service.Close(); err != nil {
			log.Print(err)
		}
	}()
	application := app.New()
	native.NewWindow(application, service).ShowAndRun()
}
