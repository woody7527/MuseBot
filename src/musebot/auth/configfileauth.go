package auth

import "musebot"

type ConfigFileAuth struct {
	availableUsers map[string]string
}

func (cfa *ConfigFileAuth) String() string {
	return "ConfigFileAuth by Luke Granger-Brown"
}

func (cfa *ConfigFileAuth) Setup(cfg map[string]string) {
	cfa.availableUsers = cfg
}

func (cfa *ConfigFileAuth) CanChangePassword() bool {
	return false
}

func (cfa *ConfigFileAuth) ChangePassword(userId string, password string) (bool, error) {
	return false, CantChangePasswordError
}

func (cfa *ConfigFileAuth) CheckLogin(username string, password string) (bool, *musebot.User, error) {
	actualPassword, ok := cfa.availableUsers[username]
	if !ok {
		return false, nil, nil
	}

	return (actualPassword == password), &musebot.User{Id: username, Username: username, Administrator: (username == "lukegb")}, nil
}
