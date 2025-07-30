package config

import (
	"encoding/json"
	"os"
)

const configFileName = ".gatorconfig.json"

type Config struct {
	DbUrl           string `json:"db_url"`
	CurrentUserName string `json:"current_user_name"`
}

func (cfg Config) write() error {
	data, err := json.Marshal(cfg)
	if err != nil {
		return err
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		panic(err)
	}
	err = os.WriteFile(homeDir+"/"+configFileName, data, 0644)
	if err != nil {
		return err
	}

	return nil
}

func (cfg Config) SetUser(user string) error {
	cfg.CurrentUserName = user
	err := cfg.write()
	if err != nil {
		return err
	}

	return nil
}

func Read() Config {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		panic(err)
	}

	data, err := os.ReadFile(homeDir + "/" + configFileName)
	if err != nil {
		panic(err)
	}

	config := Config{}
	decoder_err_ := json.Unmarshal(data, &config)
	if decoder_err_ != nil {
		panic(decoder_err_)
	}

	return config
}
