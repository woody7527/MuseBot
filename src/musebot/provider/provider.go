package provider

import (
	"bytes"
	"io/ioutil"
	"musebot"
	"net/http"
	"os"
	"strconv"
)

type Provider musebot.Provider

func Providers() []Provider {
	return []Provider{new(GroovesharkProvider)}
}

func downloadFileAndReportProgress(finalUrl string, location string, comms chan musebot.ProviderMessage) (string, error) {
	resp, err := http.Get(finalUrl)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	fullLength, err := strconv.ParseUint(resp.Header.Get("Content-Length"), 10, 64) // this REQUIRES a content-length header
	if err != nil {
		return "", err
	}
	comms <- musebot.ProviderMessage{"length", fullLength}
	finalOutput := new(bytes.Buffer)
	bytesDownloaded := 0

	for {
		buffer := make([]byte, 4096)
		n, err := resp.Body.Read(buffer)
		if err != nil && err.Error() == "EOF" {
			break
		} else if err != nil {
			return "", err
		}
		bytesDownloaded += n
		comms <- musebot.ProviderMessage{"downloaded", bytesDownloaded}
		finalOutput.Write(buffer[:n])
	}
	// write to disk!
	ioutil.WriteFile(location, finalOutput.Bytes(), os.FileMode(0666))

	return "", nil
}

func doesFileExist(path string) (bool, error) {
	_, err := os.Stat(path)
	if err != nil {
		if e, ok := err.(*os.PathError); ok && os.IsNotExist(e) {
			return false, nil
		}

		return false, err
	}
	return true, nil
}
