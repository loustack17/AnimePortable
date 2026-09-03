// SPDX-License-Identifier: MPL-2.0

package sqlite

import (
	"unicode/utf8"

	"animeportable/core"
	metadatapolicy "animeportable/internal/metadata"
)

const (
	maxIdentityBytes    = 1024
	maxTitleBytes       = 65536
	maxDescriptionBytes = 1048576
	maxCoverURLBytes    = 8192
	maxSeasonBytes      = 128
)

func validText(value string, maximum int) bool {
	return utf8.ValidString(value) && len(value) <= maximum
}

func validIdentity(value string) bool {
	return validText(value, maxIdentityBytes) && value != ""
}

func validAnime(anime core.Anime) bool {
	return validIdentity(string(anime.ID)) &&
		validText(anime.Title, maxTitleBytes) &&
		validText(anime.NativeTitle, maxTitleBytes) &&
		validText(anime.Description, maxDescriptionBytes)
}

func validSourceRef(ref core.SourceRef) bool {
	return validIdentity(ref.Provider) && validIdentity(ref.ID)
}

func validMetadata(metadata core.AnimeMetadata) bool {
	return validIdentity(metadata.Ref.Provider) &&
		validIdentity(metadata.Ref.ID) &&
		metadatapolicy.IsCanonicalPlainText(metadata.Title, metadatapolicy.TitleLimits()) &&
		metadatapolicy.IsCanonicalPlainText(metadata.NativeTitle, metadatapolicy.TitleLimits()) &&
		metadatapolicy.IsCanonicalPlainText(metadata.Description, metadatapolicy.DescriptionLimits()) &&
		metadatapolicy.IsSafeCoverURL(metadata.CoverURL) &&
		metadatapolicy.IsCanonicalPlainText(metadata.Season, metadatapolicy.SeasonLimits()) &&
		metadata.Year >= 0 && metadata.Year <= 9999 &&
		metadatapolicy.IsCanonicalPlainText(metadata.Studio, metadatapolicy.StudioLimits()) &&
		metadata.EpisodeCount >= 0
}
