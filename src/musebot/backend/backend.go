package backend

import "musebot"

type Backend musebot.Backend

func Backends() []Backend {
	return []Backend{new(MpdBackend)}
}
