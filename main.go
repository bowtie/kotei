package main

import (
	"log"
	"os"

	"kotei/internal/config"
	"kotei/internal/scheduler"
	"kotei/internal/sonarr"
	"kotei/internal/util"
)

func main() {
	log.SetFlags(0)

	appConfig, err := config.LoadConfig()
	if err != nil {
		return
	}

	log.Println(util.BlueBold("--- Anime Canon Episode Monitor (Kotei) ---"))
	if appConfig.DryRun {
		log.Println(util.YellowBold(" *** DRY RUN MODE ENABLED (via config) ***"))
	}

	appBaseLogger := log.Default()
	sClient := sonarr.NewClient(appConfig, appBaseLogger)

	errorCount := scheduler.Run(appConfig, sClient, appConfig.DryRun)

	if errorCount > 0 {
		os.Exit(1)
	}
}
