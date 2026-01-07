package config

import (
	"fmt"
	"log"
	"net/url"

	"github.com/spf13/viper"
)

var loaded = false

func LoadAndValidateConfiguration(file string) {
	if loaded {
		return
	}

	loaded = true

	viper.SetConfigFile(file)
	viper.SetConfigType("yaml")
	if err := viper.ReadInConfig(); err != nil {
		log.Fatal(fmt.Errorf("fatal error reading config: %w", err))
	}

	validateConfiguration()
}

func validateConfiguration() {
	validateServerSection()
	validateMicropubSection()
	validatePersistenceSection()
}

func validateServerSection() {
	address := viper.GetString("server.address")
	if address == "" || address[len(address)-1] != ':' {
		log.Fatal("server.address: invalid value, should not be empty and should end with colon (\":\")")
	}

	port := viper.GetUint("server.port")
	if port == 0 {
		log.Fatal("server.port: invalid value, should be a positive whole number (a valid network port)")
	}
}

func validateMicropubSection() {
	meUrl := viper.GetString("micropub.me_url")
	if meUrl == "" {
		log.Fatal("micropub.me_url: should be defined")
	} else if _, err := url.ParseRequestURI(meUrl); err != nil {
		log.Fatal("micropub.me_url: should be a valid URL")
	}

	tokenEndpoint := viper.GetString("micropub.token_endpoint")
	if tokenEndpoint == "" {
		log.Fatal("micropub.token_endpoint: should be defined")
	} else if _, err := url.ParseRequestURI(tokenEndpoint); err != nil {
		log.Fatal("micropub.token_endpoint: should be a valid URL")
	}
}

func validatePersistenceSection() {
	strategy := viper.GetString("persistence.strategy")
	if strategy != "static" {
		log.Fatal("persistence.strategy: invalid value, should be \"static\"")
	}

	validatePersistenceStaticSection()
}

func validatePersistenceStaticSection() {
	method := viper.GetString("persistence.static.method")
	if method != "git" {
		log.Fatal("persistence.static.method: invalid value, should be \"git\"")
	}

	validatePersistenceStaticGitSection()

	// TODO: add other static method checking -- sftp, http
}

func validatePersistenceStaticGitSection() {
	prefix := "persistence.static.git."
	repository := viper.GetString(prefix + "repository")
	if repository == "" {
		log.Fatal("persistence.static.git.repository: should be defined")
	} else if _, err := url.ParseRequestURI(repository); err != nil {
		log.Fatal("persistence.static.git.repository: should be a valid URL")
	}

	username := viper.GetString(prefix + "username")
	if username == "" {
		log.Fatal("persistence.static.git.username: should be defined")
	}

	password := viper.GetString(prefix + "password")
	if password == "" {
		log.Fatal("persistence.static.git.password: should be defined")
	}

	directory := viper.GetString(prefix + "directory")
	if directory == "" {
		log.Fatal("persistence.static.git.directory: should be defined")
	}
}

func BindAddress() string {
	return fmt.Sprintf("%v%v", viper.GetString("server.address"), viper.GetUint("server.port"))
}

func TokenEndpoint() string {
	return viper.GetString("micropub.token_endpoint")
}

func MeUrl() string {
	return viper.GetString("micropub.me_url")
}

func Debug() bool {
	return viper.GetBool("debug")
}
