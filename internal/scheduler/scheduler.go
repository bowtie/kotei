package scheduler

import (
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"kotei/internal/config"
	"kotei/internal/processor"
	"kotei/internal/sonarr"
	"kotei/internal/util"

	"github.com/robfig/cron/v3"
)

const scheduleTagText = "[SCHEDULE]"

func runChecks(appConfig config.Config, sClient *sonarr.Client, dryRun bool, isScheduledRun bool) (int, bool) {
	if len(appConfig.Animes) == 0 {
		return 0, isScheduledRun
	}
	wasAllQuietOrNoOp := false
	if !isScheduledRun {
		log.Printf("%s Processing %d anime series...", util.Green("[INFO]"), len(appConfig.Animes))
	}
	runErrorsEncountered, runHardFailuresCount, runSuccessCount, runSkippedNotFoundCount := 0, 0, 0, 0
	anyAnimeHadActionOrErrorInRun := false
	anyAnimeOutputtedLogs := false
	for _, animeCfg := range appConfig.Animes {
		animeActionTaken, processErr, animeDidLog := processor.ProcessAnime(animeCfg, sClient, dryRun, isScheduledRun)
		if animeDidLog {
			anyAnimeOutputtedLogs = true
		}
		if animeActionTaken || processErr != nil {
			anyAnimeHadActionOrErrorInRun = true
		}
		var statusPartString string
		printStatusLineForThisAnime := false
		if processErr != nil {
			runErrorsEncountered++
			printStatusLineForThisAnime = true
			if errors.Is(processErr, sonarr.ErrSeriesNotFound) {
				statusPartString = util.Yellow("[STATUS] SKIPPED (Not Found)")
				runSkippedNotFoundCount++
			} else {
				statusPartString = util.RedBold("[STATUS] ERROR")
				runHardFailuresCount++
			}
		} else {
			runSuccessCount++
			if animeActionTaken {
				statusPartString = util.GreenBold("[STATUS] OK (New actions taken)")
				printStatusLineForThisAnime = true
			} else if !isScheduledRun {
				statusPartString = util.GreenBold("[STATUS] OK (No new actions)")
				printStatusLineForThisAnime = true
			}
		}
		if printStatusLineForThisAnime {
			if !animeDidLog {
				log.Println()
				log.Printf("  %s%s", util.BlueBold("Processing: "), animeCfg.SonarrTitle)
			}
			log.Printf("  %s", statusPartString)
			anyAnimeOutputtedLogs = true
		}
	}
	if isScheduledRun && !anyAnimeHadActionOrErrorInRun && runErrorsEncountered == 0 {
		wasAllQuietOrNoOp = true
	} else {
		wasAllQuietOrNoOp = false
		if anyAnimeOutputtedLogs || !isScheduledRun {
			log.Println()
		}
		var statsParts []string
		statsParts = append(statsParts, "Processed: "+util.BlueBold(strconv.Itoa(len(appConfig.Animes))))
		statsParts = append(statsParts, "OK: "+util.GreenBold(strconv.Itoa(runSuccessCount)))
		if runSkippedNotFoundCount > 0 {
			statsParts = append(statsParts, "Skipped: "+util.YellowBold(strconv.Itoa(runSkippedNotFoundCount)))
		}
		if runHardFailuresCount > 0 {
			statsParts = append(statsParts, "Failed: "+util.RedBold(strconv.Itoa(runHardFailuresCount)))
		}
		otherIssues := runErrorsEncountered - runHardFailuresCount - runSkippedNotFoundCount
		if otherIssues < 0 {
			otherIssues = 0
		}
		if otherIssues > 0 {
			statsParts = append(statsParts, util.Purple("Other Issues: ")+util.Purple(strconv.Itoa(otherIssues)))
		}
		statsString := strings.Join(statsParts, " | ")
		log.Printf("%s %s", util.Cyan("[Run Stats]"), statsString)
		if dryRun {
			if !anyAnimeOutputtedLogs && isScheduledRun {
				log.Println()
			}
			log.Printf("  %s", util.YellowBold("(Dry Run - No changes made)"))
		}
		if runHardFailuresCount > 0 {
			log.Println(util.Red("  Run completed with errors."))
		} else if runErrorsEncountered > 0 {
			log.Println(util.Yellow("  Run completed with some non-critical issues or skips."))
		} else if anyAnimeHadActionOrErrorInRun || !isScheduledRun {
			log.Println(util.Green("  Run completed successfully."))
		}
	}
	return runErrorsEncountered, wasAllQuietOrNoOp
}

func Run(appConfig config.Config, sClient *sonarr.Client, dryRun bool) int {
	cronSpec := appConfig.Schedule.CronSpec
	schedulerTagColored := util.YellowBold(scheduleTagText)

	jobFuncWrapper := func() {
		runStartTime := time.Now()
		errorsInRun, wasAllQuietOrNoOp := runChecks(appConfig, sClient, dryRun, true)

		if wasAllQuietOrNoOp && errorsInRun == 0 {
			dayWithSuffix := strconv.Itoa(runStartTime.Day()) + util.GetOrdinalSuffix(runStartTime.Day())
			dateTimePart := fmt.Sprintf("%s %s %d at %s",
				dayWithSuffix, runStartTime.Month().String(), runStartTime.Year(), runStartTime.Format("15:04"))
			durationPart := fmt.Sprintf("took %s", time.Since(runStartTime).Round(time.Millisecond).String())
			detailsInsideParentheses := util.Gray(fmt.Sprintf("%s, %s", dateTimePart, durationPart))

			message := "All quiet."
			if len(appConfig.Animes) == 0 {
				message = "No anime configured."
			} else if len(appConfig.Animes) == 1 {
				message = fmt.Sprintf("1 series checked, all quiet.")
			} else {
				message = fmt.Sprintf("%d series checked, all quiet.", len(appConfig.Animes))
			}

			log.Printf("%s %s %s%s%s",
				schedulerTagColored,
				message,
				util.Gray("("),
				detailsInsideParentheses,
				util.Gray(")"))
		} else {
			log.Printf("\n%s ----- Scheduled Run Starting (%s) -----",
				schedulerTagColored,
				runStartTime.Format("2006-01-02 15:04:05"))

			log.Printf("%s ----- Scheduled Run Finished (%s, Duration: %s) -----",
				schedulerTagColored,
				time.Now().Format("2006-01-02 15:04:05"),
				time.Since(runStartTime).Round(time.Millisecond))
			if errorsInRun > 0 {
				log.Printf("%s   Note: Scheduled run completed with %s.", schedulerTagColored, util.Yellow("issues"))
			}
			log.Println()
		}
	}

	if cronSpec == "" {
		log.Println()
		log.Println(util.BlueBold("--- Single Run Mode ---"))
		errors, _ := runChecks(appConfig, sClient, dryRun, false)
		return errors
	}

	log.Println(util.BlueBold("\n--- Scheduler Mode ---"))
	log.Printf("%s Cron Spec: %s.", schedulerTagColored, util.Yellow(cronSpec))
	log.Printf("%s Performing initial check (verbose)...", schedulerTagColored)
	_, _ = runChecks(appConfig, sClient, dryRun, false)

	log.Printf("%s Scheduler active. Waiting for next run...", schedulerTagColored)
	c := cron.New(cron.WithChain(cron.SkipIfStillRunning(cron.DefaultLogger)))
	_, err := c.AddFunc(cronSpec, jobFuncWrapper)
	if err != nil {
		log.Fatalf("%s Failed to add cron job: %v", util.RedBold("!!! FATAL"), err)
	}
	c.Start()
	select {}
}
