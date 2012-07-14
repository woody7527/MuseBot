package auth

import "errors"

type Authenticator interface {
	Setup(map[string]string)

	CanChangePassword() bool
	ChangePassword(string, string) (bool, error)

	CheckLogin(string, string) (bool, error)
}

func Authenticators() []Authenticator {
	return []Authenticator{&ConfigFileAuth{}}
}

var CantChangePasswordError = errors.New("Can't change password with this backend")
