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

type GroovesharkProvider struct {
	info   groovesharkInfo
	client *http.Client
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
	params["secretKey"] = hexMd5(p.info.sessionID + "qqq")

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

func (p *GroovesharkProvider) Setup(cfg map[string]string) error {
	if cfg == nil {
		return errors.New("Grooveshark Provider requires configuration!")
	}

	p.info = groovesharkInfo{}
	p.info.headers = groovesharkHeaders{}

	clientName, nok := cfg["clientName"]
	clientRevision, rok := cfg["clientRevision"]
	clientRevToken, rtok := cfg["clientRevToken"]

	if !nok || !rok || !rtok {
		return errors.New("Grooveshark Provider: clientName/clientRevision/clientRevToken were missing from configuration.")
	}

	p.info.headers["client"] = clientName
	p.info.headers["clientRevision"] = clientRevision
	p.info.revToken = clientRevToken

	// generate an HTTP client
	p.client = &http.Client{}

	log.Println("Fetching country from Grooveshark...")
	resp, err := p.fetchWebPage("http://html5.grooveshark.com")
	if err != nil {
		return err
	}

	// now we need to pull out the actual config JSON
	// looking for "window.GS.config = " and ending with ;
	gsConfigStart := strings.Index(resp, "window.GS.config = ") + len("window.GS.config = ")
	gsConfigEnd := strings.Index(resp[gsConfigStart:], "};") + 1 + gsConfigStart

	gsFromPage := new(groovesharkConfigHtml5)
	err = json.Unmarshal([]byte(resp[gsConfigStart:gsConfigEnd]), gsFromPage)
	if err != nil {
		return err
	}
	p.info.headers["country"] = gsFromPage.Country
	p.info.runMode = gsFromPage.RunMode
	p.info.headers["session"] = gsFromPage.SessionID
	p.info.sessionID = gsFromPage.SessionID
	p.info.headers["uuid"] = "F245429D-5952-439C-B174-B97357124C46" // TODO: generate this at runtime
	p.info.headers["privacy"] = "0"
	p.info.endpoint = "more.php"

	// fetching comms token
	log.Println("Fetching communications token from Grooveshark...")
	err = p.updateCommsToken()
	if err != nil {
		return err
	}

	// done

	return nil
}

func (p *GroovesharkProvider) Search(string) ([]musebot.SongInfo, error) {
	return make([]musebot.SongInfo, 0), errors.New("Unimplemented")
}

func (p *GroovesharkProvider) UpdateSongInfo(*musebot.SongInfo) error {
	return errors.New("Unimplemented")
}

func (p *GroovesharkProvider) FetchSong(*musebot.SongInfo) (chan string, error) {
	return make(chan string), errors.New("Unimplemented")
}
