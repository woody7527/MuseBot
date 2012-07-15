package musebot

type SongInfo struct {
	Title  string
	Album  string
	Artist string
	Length int
	Id     string

	MusicUrl    string // contains *on-disk* location of the song
	CoverArtUrl string // contains URL (may be HTTP) of the coverart

	Provider     Provider
	ProviderName interface{}
	ProviderId   string

	QueueInfo    *QueuedSongInfo
	PlaybackInfo *CurrentSongInfo
}

type QueuedSongInfo struct {
	Culprit      string   // identifier of user who added song to queue
	VotedAgainst []string // victims :P
}

type CurrentSongInfo struct {
	Position float64
	State    string
}

type FullSongInfo struct {
	Song         *SongInfo
	QueueInfo    *QueuedSongInfo
	PlaybackInfo *CurrentSongInfo
}

func (si *SongInfo) PercentPosition() float64 {
	return float64((*(si.PlaybackInfo)).Position) / float64(si.Length)
}
