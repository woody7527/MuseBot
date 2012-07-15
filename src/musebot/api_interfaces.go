package musebot

type ApiResponse interface{}

type CurrentSongApiResponse struct {
	Playing bool

	CurrentSong *SongInfo
}

type PlaybackQueueApiResponse struct {
	Queue []SongInfo
}

type QueuedApiResponse struct {
	Song SongInfo
}

type ErrorApiResponse struct {
	Error string
}

type AvailableProvidersApiResponse struct {
	Providers map[string]string
}

type SearchResultsApiResponse struct {
	Results []SongInfo
}

type LoggedInApiResponse struct {
	Username string
}

type LoggedOutApiResponse struct {
	LoggedOut bool
}

type JobQueuedApiResponse struct {
	JobId string
}

type JobWebSocketApiResponse struct {
	JobId string
	Data  interface{}
}
