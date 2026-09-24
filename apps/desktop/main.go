// SPDX-License-Identifier: MPL-2.0

package main

import (
	"context"
	"log"
	"os"

	"animeportable/apps/desktop/backend"
	"animeportable/apps/desktop/native"
	"fyne.io/fyne/v2/app"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	executable, executableErr := os.Executable()
	configDir, _ := os.UserConfigDir()
	plan, planErr := backend.PlanPortable(executable, configDir)
	if executableErr != nil {
		planErr = executableErr
	}
	fyneDataDir := plan.DataDir
	var temporaryDataDir string
	defer func() {
		if temporaryDataDir != "" {
			_ = os.RemoveAll(temporaryDataDir)
		}
	}()
	if planErr != nil {
		temporaryDataDir, _ = os.MkdirTemp("", "animeportable-ui-*")
		fyneDataDir = temporaryDataDir
	}
	originalEnvironment, restoreEnvironment, redirectErr := native.RedirectFyneEnvironment(fyneDataDir)
	if redirectErr != nil && planErr == nil {
		planErr = redirectErr
		temporaryDataDir, _ = os.MkdirTemp("", "animeportable-ui-*")
		originalEnvironment, restoreEnvironment, redirectErr = native.RedirectFyneEnvironment(temporaryDataDir)
	}
	if redirectErr != nil {
		log.Print(redirectErr)
		return
	}
	defer restoreEnvironment()
	service := backend.NewAtWithPlayerEnvironment(plan.DatabasePath, originalEnvironment)
	defer func() {
		cancel()
		if err := service.Close(); err != nil {
			log.Print(err)
		}
	}()
	application := app.New()
	native.NewPortableWindow(application, plan, planErr, service, ctx, cancel).ShowAndRun()
}
