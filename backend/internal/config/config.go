package config

import (
	"encoding/json"
	"os"
)

type Config struct {
	MaxRetries  int    `json:"max_retries"`
	BackoffBase int    `json:"backoff_base"`
	DBPath      string `json:"db_path"`
}

func Default() *Config {
	return &Config{MaxRetries: 3, BackoffBase: 2, DBPath: "../queue.db"}
}

func Load(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {

		return Default(), nil
	}
	defer f.Close()
	var c Config
	if err := json.NewDecoder(f).Decode(&c); err != nil {
		return nil, err
	}
	return &c, nil
}

func (c *Config) Save(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return json.NewEncoder(f).Encode(c)
}
