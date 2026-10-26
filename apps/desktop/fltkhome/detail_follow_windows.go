//go:build windows

// SPDX-License-Identifier: MPL-2.0

package fltkhome

import (
	"math"

	"animeportable/apps/desktop/uitext"
	fltk "github.com/pwiecz/go-fltk"
)

func (ui *view) toggleDetailFollow() {
	detail := ui.searchModel.preview
	if detail == nil {
		return
	}
	animeID, follow := detail.Anime.ID, !detail.Following
	started := ui.setFollowing(animeID, follow, func(err error) {
		if preview := ui.searchModel.preview; err == nil && preview != nil && preview.Anime.ID == animeID {
			preview.Following = follow
		}
		ui.renderPreview()
	})
	if started {
		// The button shows inactive while the change runs.
		ui.renderPreview()
	}
}

func (ui *view) renderDetailFollow() {
	detail := ui.searchModel.preview
	if detail == nil || ui.searchModel.requestActive {
		ui.previewFollow.Hide()
		return
	}
	if detail.Following {
		ui.previewFollow.SetLabel(uitext.T().Unfollow)
	} else {
		ui.previewFollow.SetLabel(uitext.T().Follow)
	}
	ui.previewFollow.SetTooltip(ui.previewFollow.Label())
	if ui.followPending {
		ui.previewFollow.Deactivate()
	} else {
		ui.previewFollow.Activate()
	}
	ui.previewFollow.Show()
}

// followIconSize is the width of the detail screen's star follow button.
const followIconSize = 40

// placeFollowIcon puts the detail screen's follow icon at the right end of the button
// row, its right edge flush with the episode rows below it.
func (ui *view) placeFollowIcon() {
	x, width := ui.detailEpisodeRowSpan()
	ui.previewFollow.Resize(x+width-followIconSize, 118, followIconSize, 38)
}

// styleFollowIcon draws a follow button as a star: an outline when the anime is not
// followed, filled in the accent color when it is. The label is not drawn; it names the
// action for the tooltip.
func (ui *view) styleFollowIcon(button *fltk.Button, followed func() bool) {
	button.SetBox(fltk.NO_BOX)
	button.SetDrawHandler(func(func()) {
		colors := ui.colors()
		x, y, width, height := button.X(), button.Y(), button.W(), button.H()
		drawCard(colors, x, y, width, height, 13, button.HasFocus())
		centerX, centerY := x+width/2, y+height/2+1
		points := starPoints(centerX, centerY, 11)
		if followed() {
			fltk.SetDrawColor(colors.accent)
			for index := range points {
				next := points[(index+1)%len(points)]
				fltk.DrawPolygon(centerX, centerY, points[index][0], points[index][1], next[0], next[1])
			}
			return
		}
		fltk.SetDrawColor(colors.muted)
		fltk.SetLineStyle(fltk.SOLID, 2)
		for index := range points {
			next := points[(index+1)%len(points)]
			fltk.DrawLine(points[index][0], points[index][1], next[0], next[1])
		}
		fltk.SetLineStyle(fltk.SOLID, 0)
	})
}

// starPoints returns the ten corners of a five-pointed star, alternating outer and
// inner points, starting at the top.
func starPoints(centerX, centerY, radius int) [10][2]int {
	var points [10][2]int
	for index := range points {
		length := float64(radius)
		if index%2 == 1 {
			length *= 0.45
		}
		angle := -math.Pi/2 + float64(index)*math.Pi/5
		points[index] = [2]int{centerX + int(math.Round(length*math.Cos(angle))), centerY + int(math.Round(length*math.Sin(angle)))}
	}
	return points
}
