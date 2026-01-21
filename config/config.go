package config

import (
	"github.com/go-playground/validator/v10"
	"github.com/spf13/viper"
)

func (c *Config) Validate() error {
	validate := validator.New(validator.WithRequiredStructEnabled())
	validate.RegisterValidation("abspath", ValidateAbsPath)
	validate.RegisterValidation("localpath", ValidateLocalpath)
	validate.RegisterValidation("identifier", ValidateIdentifier)
	validate.RegisterValidation("pathpattern", ValidatePathPattern)

	if err := validate.Struct(c); err != nil {
		return err
	}

	return nil
}

func LoadConfig(file string) (*Config, error) {
	v := viper.New()
	v.SetConfigFile(file)
	v.SetConfigType("yaml")

	if err := v.ReadInConfig(); err != nil {
		return nil, err
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}
