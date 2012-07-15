package backend

import (
	"code.google.com/p/gompd/mpd"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log"
	//"mpd"
	"musebot"
	"os"
	"strconv"
	"strings"
	"time"
)

var lastPlaylistVersion uint32
var lastPlaylistLength uint32
var lastPlaylist []musebot.SongInfo
var lastPlaybackState string

func constructSongInfo(songDetails mpd.Attrs, m *MpdBackend) *musebot.SongInfo {
	length, _ := strconv.ParseInt(songDetails["Time"], 10, 0)

	musicUrl := songDetails["file"]
	if songDetails["file"][0:7] != "file://" {
		musicUrl = "file://" + m.musicDir + musicUrl
	}

	//stickermap, _ := m.client.StickerGet("song", songDetails["file"], "coverarturl")
	stickermap_b, _ := m.client.StickerGet("song", songDetails["file"], "songinfo")

	si := musebot.SongInfo{}
	//log.Println(si, stickermap_b["songinfo"])
	err := json.Unmarshal([]byte(stickermap_b["songinfo"]), &si)
	if err == nil {
		var wasOk bool
		sdp, wasOk := si.ProviderName.(string)
		if wasOk {
			si.Provider, wasOk = musebot.CurrentProviders[sdp]
		}
	} else {
		si.ProviderName = "<<LOCAL>>"
		si.Title = songDetails["Title"]
		si.Album = songDetails["Album"]
		si.Artist = songDetails["Artist"]
	}
	si.MusicUrl = musicUrl
	si.Length = int(length)
	si.Id = songDetails["Id"]
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

	log.Println("   - Connecting to MPD via", m.network, "at address", m.addr)
	m.client, err = mpd.Dial(m.network, m.addr)
	if err != nil {
		return err
	}

	log.Println("   - Connected!")

	m.client.ConsumeMode(true)

	status, err := m.client.Status()
	if err != nil {
		return err
	}

	lastPlaylistVersionA, err := strconv.ParseUint(status["playlist"], 10, 32)
	lastPlaylistVersion = uint32(lastPlaylistVersionA)

	lastPlaybackState = status["state"]
	log.Println("   - MPD playlist version is at: " + status["playlist"])

	lastPlaylist, err = m.PlaybackQueue()
	if err != nil {
		return err
	}

	return nil
}

func (m *MpdBackend) keepAlive() {
	for {
		status, err := m.client.Status()
		if err != nil {
			log.Println("MPD: Keep alive returned error. Reconnecting!")
			m.connect()
		}

		m.client.ConsumeMode(true)

		// check!
		lastPlaylistVersionA, err := strconv.ParseUint(status["playlist"], 10, 32)
		newLastPlaylistVersion := uint32(lastPlaylistVersionA)

		lastPlaylistLengthA, err := strconv.ParseUint(status["playlistlength"], 10, 32)
		newLastPlaylistLength := uint32(lastPlaylistLengthA)

		newPlaybackState := status["state"]
		if newPlaybackState != lastPlaybackState {
			m.commPipe <- "PLAYBACK_STATE_CHANGE " + newPlaybackState
			lastPlaybackState = newPlaybackState
		}

		if newLastPlaylistVersion != lastPlaylistVersion {
			// okay, so it's different
			newPlaylist, err := m.PlaybackQueue()
			if err != nil {
				m.commPipe <- "RELOAD_PLAYLIST"
				continue
			}

			// plchangespos ONLY LISTS NEW SONGS
			maxNum := len(newPlaylist)
			if len(lastPlaylist) > maxNum {
				maxNum = len(lastPlaylist)
			}

			//removedSongs := make([]musebot.SongInfo, maxNum)
			removedSongsId := make([]int, maxNum)
			removedSongsI := 0

			addedSongs := make([]musebot.SongInfo, maxNum)
			addedSongsPos := make([]int, maxNum)
			addedSongsI := 0

			for i := 0; i < len(newPlaylist); i++ {
				songIsNew := true
				currentLookupSong := newPlaylist[i]

				for k := 0; k < len(lastPlaylist); k++ {
					if lastPlaylist[k].Id == currentLookupSong.Id {
						songIsNew = false
						break
					}
				}
				if songIsNew {
					addedSongs[addedSongsI] = currentLookupSong
					addedSongsPos[addedSongsI] = i
					addedSongsI++
				}
			}
			for i := 0; i < len(lastPlaylist); i++ {
				songIsGone := true
				currentLookupSong := lastPlaylist[i]

				for k := 0; k < len(newPlaylist); k++ {
					if newPlaylist[k].Id == currentLookupSong.Id {
						songIsGone = false
						break
					}
				}
				if songIsGone {
					//removedSongs[removedSongsI] = currentLookupSong
					rsi, _ := strconv.ParseInt(currentLookupSong.Id, 10, 0)
					removedSongsId[removedSongsI] = int(rsi)
					removedSongsI++
				}
			}

			for i := 0; i < addedSongsI; i++ {
				b, err := json.Marshal(addedSongs[i])
				if err != nil {
					m.commPipe <- "RELOAD_PLAYLIST"
					break
				}
				m.commPipe <- "PLAYLIST_ADD " + strconv.Itoa(addedSongsPos[i]) + " " + string(b)
			}

			for i := 0; i < removedSongsI; i++ {
				m.commPipe <- "PLAYLIST_REMOVE " + strconv.Itoa(removedSongsId[i])
			}

			lastPlaylist = newPlaylist
		}

		lastPlaylistVersion = newLastPlaylistVersion
		lastPlaylistLength = newLastPlaylistLength

		time.Sleep(20 * time.Millisecond)
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
		hash.Write([]byte(s.MusicUrl))
		// we need the suffix
		endBit := strings.LastIndex(path, ".")
		newPath := m.musicDir + "/zz_" + hex.EncodeToString(hash.Sum([]byte{})) + path[endBit:]
		os.Symlink(path, newPath)

		s.MusicUrl = newPath
	}
	s.MusicUrl = s.MusicUrl[len(m.musicDir):]

	log.Println(s)
	log.Println(s.MusicUrl)

	// defer this
	defer m.forcePlayback()

	s.ProviderName = s.Provider.PackageName()
	s.Provider = nil
	jsonS, err := json.Marshal(s)

	m.client.Update(s.MusicUrl)
	m.client.StickerSet("song", s.MusicUrl, "coverarturl", s.CoverArtUrl)
	if err == nil {
		m.client.StickerSet("song", s.MusicUrl, "songinfo", string(jsonS))
	}

	return m.client.Add(s.MusicUrl)
}

func (m *MpdBackend) forcePlayback() {
	m.client.Play(-1)
}

func (m *MpdBackend) Remove(s musebot.SongInfo) error {
	intId, _ := strconv.ParseInt(s.Id, 10, 0)
	return m.client.DeleteId(int(intId))
}

func (m *MpdBackend) CurrentSong() (musebot.SongInfo, bool, error) {
	currentInfo, err := m.client.Status()
	if err != nil {
		log.Println("Error fetching status from MPD:", err)
		return musebot.SongInfo{}, false, err
	}

	if currentInfo["state"] != "play" && currentInfo["state"] != "pause" {
		return musebot.SongInfo{}, false, nil
	}

	songDetails, err := m.client.CurrentSong()
	if err != nil {
		log.Println("Error fetching current song from MPD:", err)
		return musebot.SongInfo{}, false, err
	}

	pos, _ := strconv.ParseFloat(currentInfo["elapsed"], 32)

	csi := musebot.CurrentSongInfo{
		Position: pos,
		State:    currentInfo["state"],
	}
	si := constructSongInfo(songDetails, m)
	si.PlaybackInfo = &csi

	return *si, true, nil
}

func (m *MpdBackend) PlaybackQueue() ([]musebot.SongInfo, error) {
	currentInfo, err := m.client.Status()
	if err != nil {
		log.Println("Error fetching status from MPD:", err)
		return make([]musebot.SongInfo, 0), err
	}

	currentPos := int64(0)
	if currentInfo["state"] == "play" || currentInfo["state"] == "pause" {
		//return make([]musebot.SongInfo, 0), nil // meh, may as well
		// we also need to check where we are in the playlist

		songPos, err := strconv.ParseInt(currentInfo["song"], 10, 0)
		if err != nil {
			log.Println("Error converting number to integer:", err)
			return nil, err
		}
		currentPos = songPos
	}

	playlistLength, err := strconv.ParseInt(currentInfo["playlistlength"], 10, 0)
	if err != nil {
		log.Println("Error converting number to integer:", err)
		return nil, err
	}

	songInfo, err := m.client.PlaylistInfo(int(currentPos), int(playlistLength))
	if err != nil {
		log.Println("Error fetching playlist from MPD:", err)
		return nil, err
	}

	outputSongInfo := make([]musebot.SongInfo, len(songInfo))

	currentSongInfo, _, err := m.CurrentSong()

	for i := 0; i < len(songInfo); i++ {
		outputSongInfo[i] = *constructSongInfo(songInfo[i], m)
		if err == nil && outputSongInfo[i].Id == currentSongInfo.Id {
			outputSongInfo[i] = currentSongInfo
		}
	}

	return outputSongInfo, nil
}
