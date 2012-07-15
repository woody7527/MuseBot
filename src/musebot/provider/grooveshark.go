package provider

import (
	"bytes"
	"crypto/md5"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"hash"
	"io/ioutil"
	"log"
	"math/rand"
	"musebot"
	"net/http"
	"strings"
)

type groovesharkHeaders map[string]interface{}
type groovesharkCountry map[string]int

type groovesharkInfo struct {
	headers        groovesharkHeaders
	revToken       string
	endpoint       string
	uuID           string
	sessionID      string
	currentToken   string
	runMode        string
	lastRandomizer string
}

type groovesharkConfigHtml5 struct {
	Country   groovesharkCountry
	RunMode   string
	SessionID string
}

type GroovesharkClientConfig struct {
	Name     string
	RevToken string
	Revision string
}

type GroovesharkProvider struct {
	info     groovesharkInfo
	cacheDir string
	client   *http.Client
	normal   GroovesharkClientConfig
	playback GroovesharkClientConfig
}

/* 
	Setup(map[string]string) error

	Search(string) ([]SongInfo, error)
	UpdateSongInfo(*SongInfo) error
	FetchSong(*SongInfo) (chan string, error)
*/

func hexHash(inhash hash.Hash, inp string) string {
	inhash.Write([]byte(inp))
	return hex.EncodeToString(inhash.Sum([]byte{}))
}

func hexMd5(inp string) string {
	return hexHash(md5.New(), inp)
}

func hexSha1(inp string) string {
	return hexHash(sha1.New(), inp)
}

func magicHeaders(req *http.Request) {
	req.Header.Add("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/536.11 (KHTML, like Gecko) Chrome/20.0.1132.57 Safari/536.11")
	req.Header.Add("Accept-Language", "en-GB,en-US;q=0.8,en;q=0.6")
	req.Header.Add("Accept-Charset", "ISO-8859-1,utf-8;q=0.7,*;q=0.3")
}

func generateGroovesharkUuid() string {
	// "xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx"
	// for x, random hex digit
	// for y, random (number & 3 | 8) to hex
	startStr := "xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx"
	resultStr := ""
	for i := 0; i < len(startStr); i++ {
		char := startStr[i]
		appendStr := ""
		if char != 'x' && char != 'y' {
			appendStr = string([]byte{char})
		} else if char == 'x' {
			appendStr = (hex.EncodeToString([]byte{byte(rand.Intn(15))}))[1:]
		} else if char == 'y' {
			appendStr = (hex.EncodeToString([]byte{byte(rand.Intn(3) + 8)}))[1:]
		}
		resultStr += appendStr
	}

	return resultStr
}

func generateNewRandomizer(lastRandomizer string) string {
	outputStr := ""
	for i := 0; i < 6; i++ {
		outputStr += (hex.EncodeToString([]byte{byte(rand.Intn(15))}))[1:]
	}
	if outputStr == lastRandomizer {
		return generateNewRandomizer(lastRandomizer)
	}
	return outputStr
}

func (p *GroovesharkProvider) fetchWebPage(url string) (string, error) {
	req, err := http.NewRequest("GET", url, nil)
	magicHeaders(req)
	req.Header.Add("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")

	resp, err := p.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	return string(body), err
}

func (p *GroovesharkProvider) apiCall(method string, parameters interface{}) (interface{}, error) {
	endpoint := "https://" + "html5.grooveshark.com" + "/" + p.info.endpoint + "?" + method

	readySetGo := map[string]interface{}{}
	//readySetGo["header"] = new(map[string]interface{})
	readySetGo["method"] = method
	readySetGo["parameters"] = parameters

	// okay, now...
	readySetGo["header"] = p.info.headers

	// set up the client correctly:
	var clientObject GroovesharkClientConfig
	if method == "getStreamKeyFromSongIDEx" {
		clientObject = p.playback
	} else {
		clientObject = p.normal
	}
	(readySetGo["header"].(groovesharkHeaders))["client"] = clientObject.Name
	(readySetGo["header"].(groovesharkHeaders))["clientRevision"] = clientObject.Revision

	// now we need to do stuff with tokens...
	if p.info.currentToken != "" {
		p.info.lastRandomizer = generateNewRandomizer(p.info.lastRandomizer)
		(readySetGo["header"].(groovesharkHeaders))["token"] = p.info.lastRandomizer + hexSha1(method+":"+p.info.currentToken+":"+clientObject.RevToken+":"+p.info.lastRandomizer)
	}

	/*	log.Println("CALLING GS API", endpoint)
		log.Println("METHOD:", method)
		log.Println(readySetGo)*/

	// serialize to JSON!
	json_enc, err := json.Marshal(readySetGo)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", endpoint, bytes.NewReader(json_enc))
	req.ContentLength = int64(len(json_enc))
	magicHeaders(req)
	//req.Header.Add("Content-Length", string(len(json_enc)))
	req.Header.Add("Content-Type", "text/plain")
	req.Header.Add("Accept", "application/json")
	req.Header.Add("Referer", "http://html5.grooveshark.com/")
	req.Header.Add("Origin", "http://html5.grooveshark.com")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	returnObj := new(interface{})
	json.Unmarshal(body, returnObj)

	returnObjM := (*returnObj).(map[string]interface{})
	_, failed := returnObjM["fault"]
	if failed {
		// fail!
		faultObj := returnObjM["fault"].(map[string]interface{})
		return nil, errors.New("Error in API call " + method + ": " + faultObj["message"].(string))
	}

	return *returnObj, nil
}

func (p *GroovesharkProvider) updateCommsToken() error {
	params := map[string]string{}
	params["secretKey"] = hexMd5(p.info.sessionID)

	resp, err := p.apiCall("getCommunicationToken", params)
	if err != nil {
		return err
	}

	respmarsh := resp.(map[string]interface{})
	p.info.currentToken = respmarsh["result"].(string)

	// YAYAY
	return nil
}

func (p *GroovesharkProvider) String() string {
	return "Grooveshark Provider by Luke Granger-Brown"
}

func (p *GroovesharkProvider) Name() string {
	return "Grooveshark"
}

func (p *GroovesharkProvider) PackageName() string {
	return "provider.GroovesharkProvider"
}

func (p *GroovesharkProvider) Setup(cfg map[string]string) error {
	if cfg == nil {
		return errors.New("Grooveshark Provider requires configuration!")
	}

	p.info = groovesharkInfo{}
	p.info.headers = groovesharkHeaders{}
	cacheDir, cok := cfg["cacheDir"]

	if !cok {
		return errors.New("Grooveshark Provider: cacheDir (the directory where I store files) must be provided!")
	}
	p.cacheDir = cacheDir

	clientName, nok := cfg["clientName"]
	clientRevision, rok := cfg["clientRevision"]
	clientRevToken, rtok := cfg["clientRevToken"]

	if !nok || !rok || !rtok {
		return errors.New("Grooveshark Provider: clientName/clientRevision/clientRevToken were missing from configuration.")
	}

	p.normal = GroovesharkClientConfig{
		Name:     clientName,
		Revision: clientRevision,
		RevToken: clientRevToken,
	}

	clientName, nok = cfg["playbackClientName"]
	clientRevision, rok = cfg["playbackClientRevision"]
	clientRevToken, rtok = cfg["playbackClientRevToken"]

	if !nok || !rok || !rtok {
		return errors.New("Grooveshark Provider: playbackClientName/playbackClientRevision/playbackClientRevToken were missing from configuration.")
	}

	p.playback = GroovesharkClientConfig{
		Name:     clientName,
		Revision: clientRevision,
		RevToken: clientRevToken,
	}

	p.info.currentToken = ""

	// generate an HTTP client
	p.client = &http.Client{}

	gsFromPage := new(groovesharkConfigHtml5)
	forceCfg, fcok := cfg["forceConfig"]
	if !fcok {
		log.Println("     - Fetching configuration from Grooveshark...")
		resp, err := p.fetchWebPage("http://html5.grooveshark.com")
		if err != nil {
			return err
		}

		// now we need to pull out the actual config JSON
		// looking for "window.GS.config = " and ending with ;
		gsConfigStart := strings.Index(resp, "window.GS.config = ") + len("window.GS.config = ")
		gsConfigEnd := strings.Index(resp[gsConfigStart:], "};") + 1 + gsConfigStart

		err = json.Unmarshal([]byte(resp[gsConfigStart:gsConfigEnd]), gsFromPage)
		if err != nil {
			return err
		}
	} else {
		log.Println("     - Using configured Grooveshark configuration...")
		err := json.Unmarshal([]byte(forceCfg), gsFromPage)
		if err != nil {
			return err
		}
	}
	p.info.headers["country"] = gsFromPage.Country
	p.info.runMode = gsFromPage.RunMode
	p.info.headers["session"] = gsFromPage.SessionID
	p.info.sessionID = gsFromPage.SessionID
	p.info.headers["uuid"] = generateGroovesharkUuid()
	p.info.headers["privacy"] = "0"
	p.info.endpoint = "more.php"

	// fetching comms token
	log.Println("     - Fetching communications token from Grooveshark...")
	err := p.updateCommsToken()
	if err != nil {
		return err
	}

	// done

	return nil
}

func (p *GroovesharkProvider) groovesharkSongToMuseBotSong(r map[string]interface{}, song *musebot.SongInfo) {
	coverArtFn := r["CoverArtFilename"].(string)
	if coverArtFn == "" {
		coverArtFn = "http://images.grooveshark.com/static/albums/500_default.png"
	} else {
		coverArtFn = "http://images.grooveshark.com/static/albums/500_" + coverArtFn
	}
	Title, ok := r["SongName"].(string)
	if !ok {
		Title = r["Name"].(string)
	}
	song.Title = Title
	song.Artist = r["ArtistName"].(string)
	song.Album = r["AlbumName"].(string)
	song.CoverArtUrl = coverArtFn
	song.Provider = p
	song.ProviderName = p.PackageName()
	song.ProviderId = r["SongID"].(string)
}

func (p *GroovesharkProvider) Search(query string) ([]musebot.SongInfo, error) {
	// api call is "getResultsFromSearch"
	parameters := map[string]string{"query": query, "type": "Songs", "guts": "1"}
	result, err := p.apiCall("getResultsFromSearch", parameters)
	if err != nil {
		return make([]musebot.SongInfo, 0), err
	}

	resultArrNotYet := ((((result.(map[string]interface{}))["result"]).(map[string]interface{}))["result"])
	resultArr := resultArrNotYet.([]interface{})
	finalResultArr := make([]musebot.SongInfo, len(resultArr))

	for i := 0; i < len(resultArr); i++ {
		r := resultArr[i].(map[string]interface{})
		song := new(musebot.SongInfo)
		p.groovesharkSongToMuseBotSong(r, song)
		finalResultArr[i] = *song
	}

	return finalResultArr, nil
}

func (p *GroovesharkProvider) UpdateSongInfo(song *musebot.SongInfo) error {
	if song.ProviderName != p.PackageName() {
		// buh?
		return errors.New("Song was not from this provider!")
	}

	params := map[string]interface{}{}
	params["songIDs"] = []string{song.ProviderId}

	res, err := p.apiCall("getQueueSongListFromSongIDs", params)
	if err != nil {
		return err
	}

	// here goes.
	result := res.(map[string]interface{})
	resultArray := (result["result"]).([]interface{})

	if len(resultArray) == 0 {
		return errors.New("That song no longer exists!")
	}

	resultItem := (resultArray[0]).(map[string]interface{})
	// yay?

	p.groovesharkSongToMuseBotSong(resultItem, song)

	return nil
}

func (p *GroovesharkProvider) FetchSong(song *musebot.SongInfo, comms chan musebot.ProviderMessage) {
	// this should be run inside a goroutine :P
	// here goes
	if song.ProviderName != p.PackageName() {
		// what. the. hell.
		comms <- musebot.ProviderMessage{"error", errors.New("Song was not from this provider!")}
	}

	downloadLocation := p.cacheDir + "/" + (song.ProviderId) + ".mp3"

	if c, _ := doesFileExist(downloadLocation); !c {
		// GO GO GO
		params := map[string]interface{}{}
		params["mobile"] = false
		params["prefetch"] = false
		params["songID"] = song.ProviderId
		params["country"] = p.info.headers["country"]

		comms <- musebot.ProviderMessage{"stages", 1}
		comms <- musebot.ProviderMessage{"current_stage", 1}
		comms <- musebot.ProviderMessage{"current_stage_description", "Downloading file..."}

		res, err := p.apiCall("getStreamKeyFromSongIDEx", params)
		if err != nil {
			comms <- musebot.ProviderMessage{"error", err}
			return
		}

		// now we need two pieces of iformation:
		// the streamKey, and the ip
		_, didFail := ((res.(map[string]interface{}))["result"]).([]interface{})
		if didFail {
			comms <- musebot.ProviderMessage{"error", errors.New("That song appears to no longer exist!")}
			return
		}

		resMap := ((res.(map[string]interface{}))["result"]).(map[string]interface{})
		resStreamKey := (resMap["streamKey"]).(string)
		resIp := (resMap["ip"]).(string)

		finalUrl := "http://" + resIp + "/stream.php?streamKey=" + resStreamKey // phew!
		downloadFileAndReportProgress(finalUrl, downloadLocation, comms)
	} else {
		comms <- musebot.ProviderMessage{"stages", 0}
		// awesome
	}

	song.MusicUrl = downloadLocation

	// that's actually us done! :)
	comms <- musebot.ProviderMessage{"done", nil}
}
