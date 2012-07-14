package musebot

type SongInfo struct {
	Title  string
	Album  string
	Artist string
	Length int
	Id     string

	MusicUrl    string // contains *on-disk* location of the song
	CoverArtUrl string // contains URL (may be HTTP) of the coverart

	Provider   Provider
	ProviderId string
}

type CurrentSongInfo struct {
	SongInfo SongInfo

	Position float64
}

func (csi *CurrentSongInfo) PercentPosition() float64 {
	return float64(csi.Position) / float64(csi.SongInfo.Length)
}
