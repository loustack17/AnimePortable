// SPDX-License-Identifier: MPL-2.0

// Package uitext is the one table of UI text, in Traditional Chinese, English and Japanese.
// Remote content (titles, synopses) is never translated. The language comes from the stored
// setting and can change while the app runs.
package uitext

import "sync/atomic"

// LanguageNames are the choices of the language setting, each in its own language, so they
// read the same whatever the UI language is. The order matches LanguageCodes.
var LanguageNames = [...]string{"繁體中文", "English", "日本語"}

// LanguageCodes are the backend's names of the languages of LanguageNames.
var LanguageCodes = [...]string{"zh-TW", "en", "ja"}

var current atomic.Pointer[Strings]

// T is the text of the language chosen with Use; Traditional Chinese until then.
func T() *Strings {
	if strings := current.Load(); strings != nil {
		return strings
	}
	return &traditionalChinese
}

// Use makes code's language the one T returns.
func Use(code string) { current.Store(For(code)) }

// For is the text of a backend language code; any other code is Traditional Chinese.
func For(code string) *Strings {
	switch code {
	case "en":
		return &english
	case "ja":
		return &japanese
	}
	return &traditionalChinese
}

// Strings is every UI text of one language. A text with % verbs is a fmt format; every
// language uses the same verbs in the same order.
type Strings struct {
	// Sections are the pages of the navigation and the player menu, in their fixed order.
	Sections [6]string
	// Weekdays start on Sunday, like time.Weekday.
	Weekdays [7]string

	OK, Cancel, No, Close, Quit              string
	Follow, Unfollow                         string
	PreviousPage, NextPage                   string
	Reload, Refresh, RetryLoad               string
	BackToHome, BackToFollowing              string
	BackToSchedule, BackToResults            string
	UnknownTitle, UnknownNumber              string
	Today, Yesterday, DaysAgo, TodayDate     string
	EpisodeNumber, EpisodeUnknown            string
	EpisodeIndex, EpisodeWatched             string
	ContinueWatching, ContinuePlaying        string
	ContinueTooltip                          string
	Finished, NotStarted, NotWatched         string
	WatchedUpTo, WatchedPosition             string
	LastWatched                              string
	RowFinished, RowPosition                 string
	OpenAnime, OpenAnimeTooltip              string
	SearchSource, SearchSourceTooltip        string
	LoadAnimeFailed, AnimeLoadFailed         string
	FollowFailed                             string
	DataExists, DataCopiedUnopenable         string
	DataCopyFailed, DataFolderUnwritable     string
	StartupFolderUnusable                    string
	PreparingData, SettingsDefaultsUsed      string
	OfflineNotice                            string
	HomeTitle, HomeHint, HeroTitle, HeroHint string
	BrowseAnime, LoadingContinue             string
	NothingToContinue, RecentUpdates         string
	NoFollowing, NoRecentUpdates             string
	TodayOnAir, NoLocalSchedule              string
	SearchHint, FollowingHint, HistoryHint   string
	ScheduleHint, SettingsHint, PageMissing  string
	// Separator sits between the parts of an episode row.
	Separator string

	// Search and the anime page.
	SearchButton, Searching                  string
	PreparingPlayback, LoadingAnime          string
	CancellingForPreview, CancellingPrevious string
	NoPlayableEpisodes, AnimeLoadFailedBack  string
	Studio, EpisodeCount                     string
	LoadingStored, StoredLoadFailed          string
	SearchFailed, SearchIdle                 string
	SearchNoOnline, SearchNoStored           string
	SearchOnlineFound, SearchStoredFound     string
	NoResults, ResultRange                   string
	CoverNone, CoverFailed, CoverLoading     string
	EpisodesLoadFailed, EpisodesLoading      string
	NoEpisodes, NoEpisodesShort              string
	ProgressUnavailable, EpisodeRange        string

	// 追蹤, 歷史紀錄 and 時間表.
	UpdateEpisodes, FollowingLoading          string
	FollowingLoadFailed, FollowingEmpty       string
	FollowingCount, FollowingRefreshing       string
	FollowingRefreshFailed                    string
	FollowingSomeFailed                       string
	HistoryLoading, HistoryLoadFailed         string
	HistoryEmpty, HistoryCount                string
	RemoveHistory, RemoveHistoryFailed        string
	ScheduleLoading, ScheduleStoredFailed     string
	ScheduleCount, ScheduleFetching           string
	ScheduleFetchFailed, ScheduleEmpty        string
	ScheduleRefreshing, ScheduleRefreshFailed string

	// 設定.
	SettingsSaveFailed, SettingsLoadFailed           string
	SettingsNotSavedTitle, SettingsNotSaved          string
	HeadingAppearance, HeadingPlayback               string
	HeadingNetwork, HeadingData, HeadingAbout        string
	Theme, ThemeHint                                 string
	ThemeSystem, ThemeDark, ThemeLight               string
	Language, LanguageHint                           string
	AutoplayNext, AutoplayNextHint                   string
	AutoPlay, AutoPlayHint, AutoPlayDisabled         string
	Resume, ResumeHint                               string
	HardwareDecoding, HardwareDecodingHint           string
	Proxy, ProxyHint                                 string
	ProxyNone, ProxySystem, ProxyManual              string
	ProxyAddress, ProxyAddressHint                   string
	ProxyAddressInvalid, ProxyPlaceholder            string
	TestConnection, ProxyIdle, ProxyTesting          string
	ProxyOK, ProxyFailed                             string
	ProxyUnreachable, ProxyRefused                   string
	ProxyTimeout, ProxyBadAddress                    string
	ProxyNoAddress, ProxySystemUnusable              string
	DataFolder, ChangeLocation, ChangeLocationButton string
	RestoreDefault, RestoreDefaultHint               string
	DataIdle, DataMoving, DataMoved                  string
	DataInUse, DataInvalid, DataFailed               string
	DataCopiedNotice, DataSwitchedNotice             string
	DataLoading, DataUnreadable                      string
	DataDialogTitle, ChooseFolder                    string
	ExistingData, UseExistingData                    string
	OverwriteData, OverwriteButton                   string
	MissingFolderTitle, MissingFolder                string
	UseDefaultFolder                                 string
	CurrentVersion, CheckUpdate                      string
	UpdateIdle, UpdateChecking, UpdateFailed         string
	UpdateAvailable, UpToDate                        string
	DownloadNew, OpenDownloadPage                    string
	DownloadPageHint, UpdateOpenFailed               string
	ProjectPage, OpenProjectPage                     string
	ProjectIdle, ProjectOpenFailed                   string

	// Notices from playback and the app.
	ProgressNotSavedTitle, ProgressNotSaved string
	PlaybackFailedTitle, PlayEpisodeFailed  string
	EpisodeUnavailable, NextEpisodeFailed   string
	SpeedTitle, SpeedSaveFailed             string
	FollowFailedTitle, RestartFailed        string
	// StartFailed is shown in a Windows message box: the app has no console and no window yet.
	StartFailed string

	// The player's own labels.
	PlayerEpisodes, PlayerLoading, PlayerSkipIntro string
}
