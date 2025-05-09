package processor

import (
	"errors"
	"fmt"
	"io"
	"log"
	"sort"
	"strconv"
	"strings"

	"kotei/internal/config"
	"kotei/internal/fillerlist"
	"kotei/internal/sonarr"
	"kotei/internal/util"
)

var procNilLogger = log.New(io.Discard, "", 0)

func ProcessAnime(cfg config.AnimeConfig, sClient *sonarr.Client, dryRun bool, isQuietableRun bool) (bool, error, bool) {
	actionTaken := false
	var processingError error
	didLogOwnLines := false

	var sonarrOriginalLogger *log.Logger
	if isQuietableRun {
		sonarrOriginalLogger = sClient.GetLogger()
		sClient.SetLogger(sonarr.NilLogger)
		defer sClient.SetLogger(sonarrOriginalLogger)
	}

	printHeaderOnce := func() {
		if !didLogOwnLines {
			log.Println()
			log.Printf("  %s%s", util.BlueBold("Processing: "), cfg.SonarrTitle)
			didLogOwnLines = true
		}
	}

	logOwnLine := func(forcePrint bool, format string, args ...interface{}) {
		if !isQuietableRun || forcePrint {
			printHeaderOnce()
			log.Printf(format, args...)
		}
	}

	if cfg.FillerListTitle == "" || cfg.SonarrTitle == "" {
		errMsg := fmt.Sprintf("Invalid config for %s: missing title or sonarr_title.", cfg.SonarrTitle)
		logOwnLine(true, "  %s %s", util.RedBold("!!! ERROR"), errMsg)
		return false, errors.New("invalid anime configuration entry"), didLogOwnLines
	}

	var flLogger *log.Logger
	if !isQuietableRun {
		flLogger = log.Default()
	} else {
		flLogger = fillerlist.NilLogger
	}

	mangaEps, animeEps, mixedEps, err := fillerlist.GetCategorizedCanonEpisodes(cfg.FillerListTitle, cfg.IncludeCanonTypes, flLogger)
	if err != nil {
		logOwnLine(true, "  %s Error from GetCategorizedCanonEpisodes for %s: %v", util.RedBold("!!! ERROR [FILLER]"), cfg.FillerListTitle, err)
		return false, err, didLogOwnLines
	}

	includeTypes := cfg.IncludeCanonTypes
	if len(includeTypes) == 0 {
		includeTypes = []string{"manga", "anime", "mixed"}
	}
	combinedEpisodesMap := make(map[int]bool)
	includeMap := make(map[string]bool)
	for _, t := range includeTypes {
		includeMap[strings.ToLower(strings.TrimSpace(t))] = true
	}
	if includeMap["manga"] {
		util.AddEpisodesToMap(combinedEpisodesMap, mangaEps)
	}
	if includeMap["anime"] {
		util.AddEpisodesToMap(combinedEpisodesMap, animeEps)
	}
	if includeMap["mixed"] {
		util.AddEpisodesToMap(combinedEpisodesMap, mixedEps)
	}

	episodesToProcess := []int{}
	for ep := range combinedEpisodesMap {
		if ep >= cfg.CutoffEpisode {
			episodesToProcess = append(episodesToProcess, ep)
		}
	}

	if len(episodesToProcess) == 0 {
		var consideredTypes []string
		if includeMap["manga"] {
			consideredTypes = append(consideredTypes, "manga")
		}
		if includeMap["mixed"] {
			consideredTypes = append(consideredTypes, "mixed")
		}
		if includeMap["anime"] {
			consideredTypes = append(consideredTypes, "anime")
		}
		if len(consideredTypes) == 0 {
			consideredTypes = append(consideredTypes, "configured")
		}
		logOwnLine(false, "  %s No relevant episodes from %v types (cutoff >= %d).", util.Green("[Processor]"), consideredTypes, cfg.CutoffEpisode)
	} else {
		sort.Ints(episodesToProcess)
		logOwnLine(false, "  %s Canon episodes (%s >= %s): %s found.",
			util.Purple("[FILLER]"), util.Cyan("cutoff"), util.Cyan(strconv.Itoa(cfg.CutoffEpisode)),
			util.GreenBold(fmt.Sprintf("%d", len(episodesToProcess))))
	}

	sonarrSeriesID, err := sClient.GetSeriesID(cfg.SonarrTitle)
	if err != nil {
		logOwnLine(true, "  %s Processor: Error obtaining Sonarr Series ID for '%s': %v", util.RedBold("!!! ERROR"), cfg.SonarrTitle, err)
		return false, err, didLogOwnLines
	}

	sonarrIDsToNewlyMonitor, err := sClient.GetEpisodeIDsToNewlyMonitor(sonarrSeriesID, episodesToProcess)
	if err != nil {
		logOwnLine(true, "  %s Processor: Error identifying episodes to monitor for '%s': %v", util.RedBold("!!! ERROR"), cfg.SonarrTitle, err)
		return false, err, didLogOwnLines
	}

	if len(sonarrIDsToNewlyMonitor) > 0 {
		actionTaken = true
		logOwnLine(true, "  %s Identified %d new episode(s) to monitor.", util.Cyan("[SONARR]"), len(sonarrIDsToNewlyMonitor))
		err = sClient.MonitorEpisodes(sonarrIDsToNewlyMonitor, dryRun)
		if err != nil {
			logOwnLine(true, "  %s Processor: Error during Sonarr MonitorEpisodes call for '%s': %v", util.RedBold("!!! ERROR"), cfg.SonarrTitle, err)
			processingError = err
		}
	} else {
		if len(episodesToProcess) > 0 {
			logOwnLine(false, "  %s Monitoring: No update needed (all relevant canon episodes already monitored).", util.Cyan("[SONARR]"))
		} else if !isQuietableRun {
			if len(combinedEpisodesMap) > 0 {
				logOwnLine(false, "  %s Monitoring: No update needed (no episodes to process from canon list).", util.Cyan("[SONARR]"))
			}
		}
	}

	if cfg.SearchEnabled {
		if len(sonarrIDsToNewlyMonitor) > 0 {
			actionTaken = true
			logOwnLine(true, "  %s Queuing search for %d newly monitored episode(s).", util.CyanBold("[SONARR]"), len(sonarrIDsToNewlyMonitor))
			err = sClient.SearchEpisodes(sonarrIDsToNewlyMonitor, dryRun)
			if err != nil {
				logOwnLine(true, "  %s Processor: Error during Sonarr SearchEpisodes call for '%s': %v", util.RedBold("!!! ERROR"), cfg.SonarrTitle, err)
				if processingError == nil {
					processingError = err
				}
			}
		} else {
			logOwnLine(false, "  %s Search: Skipped (no new episodes were monitored to trigger search).", util.Cyan("[SONARR]"))
		}
	} else {
		logOwnLine(false, "  %s Search: Disabled.", util.Cyan("[SONARR]"))
	}

	return actionTaken, processingError, didLogOwnLines
}
