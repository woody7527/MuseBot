package auth

import "errors"
import "musebot"

type Authenticator musebot.Authenticator

func Authenticators() []Authenticator {
	return []Authenticator{&ConfigFileAuth{}}
}

var CantChangePasswordError = errors.New("Can't change password with this backend")
