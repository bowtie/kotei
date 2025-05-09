package sonarr

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"kotei/internal/config"
	"kotei/internal/util"

	"github.com/go-resty/resty/v2"
)

var NilLogger = log.New(io.Discard, "", 0)

var ErrSeriesNotFound = errors.New("series not found in Sonarr")

type Series struct {
	ID    int    `json:"id"`
	Title string `json:"title"`
}
type Episode struct {
	ID                    int    `json:"id"`
	AbsoluteEpisodeNumber int    `json:"absoluteEpisodeNumber"`
	Monitored             bool   `json:"monitored"`
	Title                 string `json:"title"`
	SeasonNumber          int    `json:"seasonNumber"`
	EpisodeNumber         int    `json:"episodeNumber"`
}
type EpisodeMonitorRequest struct {
	EpisodeIDs []int `json:"episodeIds"`
	Monitored  bool  `json:"monitored"`
}
type SonarrCommandRequest struct {
	Name       string `json:"name"`
	EpisodeIDs []int  `json:"episodeIds,omitempty"`
}

type Client struct {
	resty  *resty.Client
	logger *log.Logger
}

func NewClient(cfg config.Config, appLogger *log.Logger) *Client {
	if appLogger == nil {
		appLogger = log.Default()
	}
	cleanBaseURL := strings.TrimSuffix(cfg.Sonarr.BaseURL, "/")
	sonarrFullBaseURL := cleanBaseURL + cfg.Sonarr.APIPath
	restyClient := resty.New().
		SetBaseURL(sonarrFullBaseURL).
		SetHeader("X-Api-Key", cfg.Sonarr.APIKey).
		SetTimeout(time.Duration(cfg.Sonarr.TimeoutSeconds) * time.Second).
		SetRetryCount(cfg.Sonarr.RetryCount).
		SetRetryWaitTime(time.Duration(cfg.Sonarr.RetryWaitSeconds) * time.Second).
		OnError(func(req *resty.Request, err error) {
			errMsg := fmt.Sprintf("API Request Error. URL: %s, Method: %s", req.URL, req.Method)
			if err != nil {
				log.Printf("  %s %s | Error: %v", util.RedBold("[SONARR HTTP ERR]"), errMsg, err.Error())
				if v, ok := err.(*resty.ResponseError); ok && v.Response != nil {
					if len(v.Response.Body()) > 0 && len(v.Response.Body()) < 500 {
						log.Printf("  %s Response Body: %s", util.RedBold("[SONARR HTTP ERR]"), string(v.Response.Body()))
					}
				}
			} else {
				log.Printf("  %s %s | Unknown Error (err is nil)", util.RedBold("[SONARR HTTP ERR]"), errMsg)
			}
		})
	return &Client{resty: restyClient, logger: appLogger}
}

func (c *Client) GetLogger() *log.Logger {
	if c.logger == nil {
		return NilLogger
	}
	return c.logger
}

func (c *Client) SetLogger(logger *log.Logger) {
	if logger == nil {
		c.logger = NilLogger
	} else {
		c.logger = logger
	}
}

func (c *Client) GetSeriesID(sonarrSeriesSearchTitle string) (int, error) {
	var seriesList []Series
	resp, err := c.resty.R().SetQueryParam("term", sonarrSeriesSearchTitle).SetResult(&seriesList).Get("/series")

	if err != nil {
		return 0, fmt.Errorf("failed to request series lookup for '%s': %w", sonarrSeriesSearchTitle, err)
	}
	if !resp.IsSuccess() {
		return 0, fmt.Errorf("Sonarr API error searching series '%s'. Status: %s, Body: %s", sonarrSeriesSearchTitle, resp.Status(), resp.String())
	}

	currentLogger := c.GetLogger()

	for _, series := range seriesList {
		if strings.EqualFold(series.Title, sonarrSeriesSearchTitle) {
			currentLogger.Printf("  %s Series %s (ID: %s)",
				util.Cyan("[SONARR]"),
				util.Blue(fmt.Sprintf("'%s'", series.Title)),
				util.Yellow(strconv.Itoa(series.ID)))
			return series.ID, nil
		}
	}
	currentLogger.Printf("  %s Series '%s' %s in %s (%d results, none matched exactly).",
		util.Cyan("[SONARR]"),
		sonarrSeriesSearchTitle,
		util.RedBold("not found"),
		util.Cyan("Sonarr"),
		len(seriesList))
	return 0, fmt.Errorf("%w: exact title '%s'", ErrSeriesNotFound, sonarrSeriesSearchTitle)
}

func (c *Client) GetEpisodeIDsToNewlyMonitor(sonarrSeriesID int, targetAbsoluteNumbers []int) ([]int, error) {
	if len(targetAbsoluteNumbers) == 0 {
		return []int{}, nil
	}
	var allSonarrEpisodes []Episode
	resp, err := c.resty.R().SetQueryParam("seriesId", strconv.Itoa(sonarrSeriesID)).SetResult(&allSonarrEpisodes).Get("/episode")

	if err != nil {
		return nil, fmt.Errorf("failed to request episodes for series ID %d: %w", sonarrSeriesID, err)
	}
	if !resp.IsSuccess() {
		return nil, fmt.Errorf("Sonarr API error fetching episodes for series ID %d. Status: %s, Body: %s", sonarrSeriesID, resp.Status(), resp.String())
	}

	currentLogger := c.GetLogger()
	episodesToNewlyMonitor := []int{}
	sonarrEpsMap := make(map[int]Episode)
	for _, ep := range allSonarrEpisodes {
		if ep.AbsoluteEpisodeNumber > 0 {
			sonarrEpsMap[ep.AbsoluteEpisodeNumber] = ep
		}
	}

	newlyMon, alreadyMon, notFoundCount := 0, 0, 0
	for _, absNum := range targetAbsoluteNumbers {
		sonarrEp, ok := sonarrEpsMap[absNum]
		if ok {
			if !sonarrEp.Monitored {
				episodesToNewlyMonitor = append(episodesToNewlyMonitor, sonarrEp.ID)
				newlyMon++
			} else {
				alreadyMon++
			}
		} else {
			notFoundCount++
		}
	}
	notFoundMsg := ""
	if notFoundCount > 0 {
		notFoundMsg = fmt.Sprintf(", %d not found in Sonarr", notFoundCount)
	}

	if newlyMon > 0 || alreadyMon > 0 || notFoundCount > 0 {
		currentLogger.Printf("  %s Episodes: %s to newly monitor, %s already monitored%s.",
			util.Cyan("[SONARR]"), util.GreenBold(strconv.Itoa(newlyMon)), util.Green(strconv.Itoa(alreadyMon)), notFoundMsg)
	}
	return episodesToNewlyMonitor, nil
}

func (c *Client) MonitorEpisodes(sonarrInternalEpisodeIDs []int, dryRun bool) error {
	if len(sonarrInternalEpisodeIDs) == 0 {
		return nil
	}
	currentLogger := c.GetLogger()
	actionMsg := fmt.Sprintf("  %s Monitoring %s episodes...", util.Cyan("[SONARR]"), util.GreenBold(strconv.Itoa(len(sonarrInternalEpisodeIDs))))
	if dryRun {
		currentLogger.Printf("%s %s", actionMsg, util.YellowBold("(DRY RUN)"))
		return nil
	}

	currentLogger.Printf("%s", actionMsg)
	resp, err := c.resty.R().
		SetBody(EpisodeMonitorRequest{EpisodeIDs: sonarrInternalEpisodeIDs, Monitored: true}).
		Put("/episode/monitor")

	if err != nil {
		return fmt.Errorf("failed to send monitor request for %d episodes: %w", len(sonarrInternalEpisodeIDs), err)
	}
	if resp.StatusCode() != http.StatusAccepted && resp.StatusCode() != http.StatusOK {
		return fmt.Errorf("Sonarr API error setting episodes to monitored. Expected 200/202, Got %s. Body: %s", resp.Status(), resp.String())
	}
	currentLogger.Printf("    └─ %s Status: %s", util.Cyan("[SONARR]"), util.Green("OK"))
	return nil
}

func (c *Client) SearchEpisodes(sonarrInternalEpisodeIDs []int, dryRun bool) error {
	if len(sonarrInternalEpisodeIDs) == 0 {
		return nil
	}
	currentLogger := c.GetLogger()
	actionMsg := fmt.Sprintf("  %s Searching for %s episodes...", util.Cyan("[SONARR]"), util.GreenBold(strconv.Itoa(len(sonarrInternalEpisodeIDs))))
	if dryRun {
		currentLogger.Printf("%s %s", actionMsg, util.YellowBold("(DRY RUN)"))
		return nil
	}

	currentLogger.Printf("%s", actionMsg)
	resp, err := c.resty.R().
		SetBody(SonarrCommandRequest{Name: "EpisodeSearch", EpisodeIDs: sonarrInternalEpisodeIDs}).
		Post("/command")

	if err != nil {
		return fmt.Errorf("failed to send search command for %d episodes: %w", len(sonarrInternalEpisodeIDs), err)
	}
	if resp.StatusCode() != http.StatusCreated {
		return fmt.Errorf("Sonarr API error queuing search. Expected 201, Got %s. Body: %s", resp.Status(), resp.String())
	}
	currentLogger.Printf("    └─ %s Status: %s (%d eps)", util.Cyan("[SONARR]"), util.Green("Queued"), len(sonarrInternalEpisodeIDs))
	return nil
}
