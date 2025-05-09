package config

import (
	"log"

	"github.com/spf13/viper"
)

type AnimeConfig struct {
	FillerListTitle   string   `mapstructure:"title"`
	SonarrTitle       string   `mapstructure:"sonarr_title"`
	IncludeCanonTypes []string `mapstructure:"include_canon_types"`
	CutoffEpisode     int      `mapstructure:"cutoff_episode"`
	SearchEnabled     bool     `mapstructure:"search_enabled"`
}
type Config struct {
	DryRun bool `mapstructure:"dry_run"`
	Sonarr struct {
		BaseURL          string `mapstructure:"baseurl"`
		APIKey           string `mapstructure:"apikey"`
		APIPath          string `mapstructure:"api_path"`
		TimeoutSeconds   int    `mapstructure:"timeout_seconds"`
		RetryCount       int    `mapstructure:"retry_count"`
		RetryWaitSeconds int    `mapstructure:"retry_wait_seconds"`
	} `mapstructure:"sonarr"`
	Animes   []AnimeConfig `mapstructure:"animes"`
	Schedule struct {
		CronSpec string `mapstructure:"cron_spec"`
	} `mapstructure:"schedule"`
}

func LoadConfig() (Config, error) {
	var cfg Config

	viper.SetConfigFile("./config.yaml")

	viper.SetDefault("sonarr.api_path", "/api/v3")
	viper.SetDefault("sonarr.timeout_seconds", 15)
	viper.SetDefault("sonarr.retry_count", 3)
	viper.SetDefault("sonarr.retry_wait_seconds", 5)

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			log.Fatalf("FATAL: Config file (config.yaml) not found.")
		} else {
			log.Fatalf("FATAL: Error reading config file: %v", err)
		}
	}

	if err := viper.Unmarshal(&cfg); err != nil {
		log.Fatalf("FATAL: Unable to decode config: %v", err)
	}

	if cfg.Sonarr.BaseURL == "" {
		log.Fatal("FATAL: Critical config: sonarr.baseurl is not set.")
	}
	if cfg.Sonarr.APIKey == "" {
		log.Fatal("FATAL: Critical config: sonarr.apikey is not set.")
	}
	if len(cfg.Animes) == 0 {
		log.Fatal("FATAL: Critical config: no entries found in 'animes' list.")
	}

	return cfg, nil
}
