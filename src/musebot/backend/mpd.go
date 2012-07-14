package backend

import (
	"code.google.com/p/gompd/mpd"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"log"
	"musebot"
	"os"
	"strconv"
	"strings"
	"time"
)

func constructSongInfo(songDetails mpd.Attrs, m *MpdBackend) *musebot.SongInfo {
	length, _ := strconv.ParseInt(songDetails["Time"], 10, 0)

	musicUrl := songDetails["file"]
	if songDetails["file"][0:7] != "file://" {
		musicUrl = "file://" + m.musicDir + musicUrl
	}

	stickermap, _ := m.client.StickerGet("song", songDetails["file"], "coverarturl")

	si := musebot.SongInfo{
		Title:       songDetails["Title"],
		Album:       songDetails["Album"],
		Artist:      songDetails["Artist"],
		MusicUrl:    musicUrl,
		CoverArtUrl: stickermap["coverarturl"],
		Length:      int(length),
		Id:          songDetails["Id"],
	}
	return &si
}

type MpdBackend struct {
	client   *mpd.Client
	commPipe chan string

	addr     string
	network  string
	musicDir string
}

func (m *MpdBackend) String() string {
	return "MPD Backend by Luke Granger-Brown"
}

func (m *MpdBackend) Setup(cfg map[string]string, commPipe chan string) {
	m.commPipe = commPipe

	addr, ok := cfg["addr"]
	if !ok {
		addr = "127.0.0.1:6600"
	}
	m.addr = addr

	network, ok := cfg["network"]
	if !ok {
		network = "tcp"
	}
	m.network = network

	m.musicDir, ok = cfg["musicDir"]
	if !ok {
		log.Fatalln("musicDir must be specified for the MPD Backend.")
	}

	err := m.connect()
	if err != nil {
		log.Fatalln("Error connecting to MPD", err)
	}

	go m.keepAlive()
}

func (m *MpdBackend) connect() error {
	var err error

	log.Println("Connecting to MPD via", m.network, "at address", m.addr)
	m.client, err = mpd.Dial(m.network, m.addr)
	if err != nil {
		return err
	}

	log.Println("Connected!")
	return nil
}

func (m *MpdBackend) keepAlive() {
	for {
		err := m.client.Ping()
		if err != nil {
			log.Println("MPD: Keep alive returned error. Reconnecting!")
			m.connect()
		}

		time.Sleep(1 * time.Second)
	}
}

func (m *MpdBackend) ifNotPlayingEmptyQueue() {
	// grab current info
	currentInfo, err := m.client.Status()
	if err == nil {
		if currentInfo["state"] == "stop" {
			m.client.Clear() // boom. Bye.
		}
	}
}

func (m *MpdBackend) Add(s musebot.SongInfo) error {
	m.ifNotPlayingEmptyQueue()

	// here goes
	path := s.MusicUrl

	// this should be a local filesystem path by now...
	if len(path) == 0 || path[0] != '/' {
		err := errors.New("MpdBackend: path invalid for song with path " + path)
		log.Println("Error adding song to queue: non-absolute path!", err)
		return err
	}

	if !strings.HasPrefix(path, m.musicDir) {
		hash := sha256.New()
		hash.Write([]byte(m.musicDir))
		newPath := m.musicDir + "/zz_" + hex.EncodeToString(hash.Sum([]byte{}))
		os.Symlink(path, newPath)

		s.MusicUrl = newPath
	} else {
		s.MusicUrl = s.MusicUrl[len(m.musicDir):]
	}

	// defer this
	defer m.forcePlayback()

	m.client.StickerSet("song", s.MusicUrl, "coverarturl", s.CoverArtUrl)

	return m.client.Add(s.MusicUrl)

}

func (m *MpdBackend) forcePlayback() {
	m.client.Play(-1)
}

func (m *MpdBackend) Remove(s musebot.SongInfo) error {
	intId, _ := strconv.ParseInt(s.Id, 10, 0)
	return m.client.DeleteId(int(intId))
}

func (m *MpdBackend) CurrentSong() (musebot.CurrentSongInfo, bool, error) {
	currentInfo, err := m.client.Status()
	if err != nil {
		log.Println("Error fetching status from MPD:", err)
		return musebot.CurrentSongInfo{}, false, err
	}

	if currentInfo["state"] != "play" {
		return musebot.CurrentSongInfo{}, false, nil
	}

	songDetails, err := m.client.CurrentSong()
	if err != nil {
		log.Println("Error fetching current song from MPD:", err)
		return musebot.CurrentSongInfo{}, false, err
	}

	pos, _ := strconv.ParseFloat(currentInfo["elapsed"], 32)

	csi := musebot.CurrentSongInfo{
		Position: pos,
	}
	csi.SongInfo = *constructSongInfo(songDetails, m)

	return csi, true, nil
}

func (m *MpdBackend) PlaybackQueue() ([]musebot.SongInfo, error) {
	songInfo, err := m.client.PlaylistInfo(-1, -1)
	if err != nil {
		log.Println("Error fetching playlist from MPD:", err)
		return nil, err
	}

	outputSongInfo := make([]musebot.SongInfo, len(songInfo))

	for i := 0; i < len(songInfo); i++ {
		outputSongInfo[i] = *constructSongInfo(songInfo[i], m)
	}

	return outputSongInfo, nil
}
