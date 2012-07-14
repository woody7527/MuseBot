package musebot

type SongInfo struct {
	Title    string
	Album    string
	Artist   string
	MusicUrl string
	Length   int
	Id       string
}

type CurrentSongInfo struct {
	SongInfo SongInfo

	Position float64
}

func (csi *CurrentSongInfo) PercentPosition() float64 {
	return float64(csi.Position) / float64(csi.SongInfo.Length)
}
