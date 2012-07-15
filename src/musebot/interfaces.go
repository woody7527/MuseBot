package musebot

type Provider interface {
	Setup(map[string]string) error

	Name() string
	PackageName() string

	Search(string) ([]SongInfo, error)
	UpdateSongInfo(*SongInfo) error
	FetchSong(*SongInfo, chan ProviderMessage)
}

type Backend interface {
	CurrentSong() (SongInfo, bool, error)
	PlaybackQueue() ([]SongInfo, error)
	Add(SongInfo) error
	Remove(SongInfo) error

	Setup(map[string]string, chan string)
}

type Authenticator interface {
	Setup(map[string]string)

	CanChangePassword() bool
	ChangePassword(string, string) (bool, error)

	CheckLogin(string, string) (bool, *User, error)
}

type SystemMessage struct {
	Type    string
	Content interface{}
}

type User struct {
	Id            string
	Username      string
	Administrator bool
}

type ProviderMessage SystemMessage

type BackendMessage SystemMessage

type Providers map[string]Provider

var CurrentAuthenticator Authenticator
var CurrentBackend Backend
var CurrentProviders Providers
