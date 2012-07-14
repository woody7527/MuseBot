package backend

import "musebot"

type Backend interface {
	CurrentSong() (musebot.CurrentSongInfo, bool)
	PlaybackQueue() []musebot.SongInfo
	Add(musebot.SongInfo) error
	Remove(musebot.SongInfo) error

	Setup(map[string]string, chan string)
}

func Backends() []Backend {
	return []Backend{new(MpdBackend)}
}
