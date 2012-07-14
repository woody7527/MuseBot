package musebot

import (
	"encoding/json"
	"io/ioutil"
	"log"
)

type JsonCfg struct {
	Backend       string
	BackendConfig map[string]map[string]string

	AuthBackend       string
	AuthBackendConfig map[string]map[string]string

	ProviderBackendConfig map[string]map[string]string
}

func (cfg *JsonCfg) LoadConfiguration() (err error) {
	b, err := ioutil.ReadFile("/home/lukegb/Projects/musebot3/config.json")
	if err != nil {
		return err
	}
	err = json.Unmarshal(b, &cfg)
	if err != nil {
		log.Fatalln("An error occurred whilst parsing config.json:", err)
	}
	return nil
}
