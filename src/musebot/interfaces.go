package musebot

type Provider interface {
	Setup(map[string]string) error

	Search(string) ([]SongInfo, error)
	UpdateSongInfo(*SongInfo) error
	FetchSong(*SongInfo, chan ProviderMessage)
}

type Backend interface {
	CurrentSong() (CurrentSongInfo, bool, error)
	PlaybackQueue() ([]SongInfo, error)
	Add(SongInfo) error
	Remove(SongInfo) error

	Setup(map[string]string, chan string)
}

type Authenticator interface {
	Setup(map[string]string)

	CanChangePassword() bool
	ChangePassword(string, string) (bool, error)

	CheckLogin(string, string) (bool, error)
}

type ProviderMessage struct {
	Type    string
	Content interface{}
}
