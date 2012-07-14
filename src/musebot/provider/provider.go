package provider

import "musebot"

type Provider musebot.Provider

func Providers() []Provider {
	return []Provider{new(GroovesharkProvider)}
}
