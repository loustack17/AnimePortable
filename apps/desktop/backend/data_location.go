// SPDX-License-Identifier: MPL-2.0

package backend

import (
	"context"
	"time"
)

// Existing choices for MoveDataLocation when the target folder already holds a database.
const (
	ExistingUse       = "use"
	ExistingOverwrite = "overwrite"
)

// DataLocation is the data folder the app is set to use. Dir changes as soon as a move
// succeeds, although the running app keeps its open database until it restarts.
type DataLocation struct {
	Dir    string `json:"dir"`
	Custom bool   `json:"custom"`
}

// MoveDataLocationRequest asks to move the data to Target, or back to the default folder when
// RestoreDefault is set (Target is then empty). Existing answers a NeedsChoice result: empty
// at first, then ExistingUse or ExistingOverwrite.
type MoveDataLocationRequest struct {
	Target         string `json:"target"`
	RestoreDefault bool   `json:"restoreDefault"`
	Existing       string `json:"existing"`
}

// MoveDataLocationResult says that the folder already has data and the user must choose
// (NeedsChoice), or the folder the data now belongs in.
type MoveDataLocationResult struct {
	NeedsChoice bool   `json:"needsChoice"`
	Dir         string `json:"dir"`
}

// DataLocation reports the configured data folder.
func (service *Service) DataLocation(ctx context.Context) (DataLocation, error) {
	_, done, err := service.begin(ctx)
	if err != nil {
		return DataLocation{}, err
	}
	defer done()
	service.mu.Lock()
	plan := service.plan
	service.mu.Unlock()
	if plan.DataDir == "" {
		return DataLocation{}, ErrUnavailable
	}
	return DataLocation{Dir: plan.DataDir, Custom: plan.Location == LocationCustom}, nil
}

// MoveDataLocation copies the open database to the chosen folder and records it for the next
// start; the old data stays where it was. It only works with the store, so it lives on the
// Service. After a success the app must restart at once: the open store keeps the old
// database, so it refuses every write (ErrRestartRequired) and so does a second move. The
// folder of the open database cannot be the target (ErrDataFolderInUse).
func (service *Service) MoveDataLocation(ctx context.Context, request MoveDataLocationRequest) (MoveDataLocationResult, error) {
	operationCtx, done, err := service.begin(ctx)
	if err != nil {
		return MoveDataLocationResult{}, err
	}
	defer done()
	if request.Existing != "" && request.Existing != ExistingUse && request.Existing != ExistingOverwrite {
		return MoveDataLocationResult{}, ErrInvalidInput
	}
	service.locationMu.Lock()
	defer service.locationMu.Unlock()
	if service.dataMoved.Load() {
		return MoveDataLocationResult{}, ErrRestartRequired
	}
	service.mu.Lock()
	plan, store := service.plan, service.store
	service.mu.Unlock()
	snapshotter, ok := store.(DatabaseSnapshotter)
	if !ok {
		return MoveDataLocationResult{}, ErrUnavailable
	}

	var move LocationMove
	if request.RestoreDefault {
		if request.Target != "" {
			return MoveDataLocationResult{}, ErrInvalidInput
		}
		move, err = plan.PrepareRestoreDefault()
	} else {
		move, err = plan.PrepareLocationChange(request.Target)
	}
	if err != nil {
		return MoveDataLocationResult{}, err
	}
	if move.HasExistingData() && request.Existing == "" {
		return MoveDataLocationResult{NeedsChoice: true, Dir: move.Dir()}, nil
	}
	// Refuse writes before the copy starts: a write that waits for the store while the
	// snapshot runs must not succeed into the old database afterwards. A failed move allows
	// writes again.
	service.dataMoved.Store(true)
	switch {
	case !move.HasExistingData():
		err = move.CopyCurrent(operationCtx, snapshotter)
	case request.Existing == ExistingUse:
		err = move.UseExisting(operationCtx)
	default:
		err = move.OverwriteWithCurrent(operationCtx, snapshotter, time.Now())
	}
	if err != nil {
		service.dataMoved.Store(false)
		return MoveDataLocationResult{}, err
	}

	service.mu.Lock()
	service.plan.DataDir = move.Dir()
	service.plan.CustomDir = move.Dir()
	service.plan.Location = LocationCustom
	if request.RestoreDefault {
		service.plan.CustomDir = ""
		service.plan.Location = LocationDefault
	}
	service.mu.Unlock()
	return MoveDataLocationResult{Dir: move.Dir()}, nil
}
