package fillerlist

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"kotei/internal/util"

	"github.com/PuerkitoBio/goquery"
)

var NilLogger = log.New(io.Discard, "", 0)

func atoiSimple(s string) int { i, _ := strconv.Atoi(strings.TrimSpace(s)); return i }

func parseRange(parts []string) (int, int) {
	if len(parts) != 2 {
		return 0, 0
	}
	start := atoiSimple(parts[0])
	end := atoiSimple(parts[1])
	if start > 0 && end >= start {
		return start, end
	}
	if start > 0 && end == 0 {
		return start, start
	}
	return 0, 0
}

func parseSingleEpisode(text string) int {
	num := atoiSimple(text)
	if num > 0 {
		return num
	}
	return 0
}

func scrapeEpisodesFromSection(doc *goquery.Document, sectionSelector string, logger *log.Logger) ([]int, error) {
	if logger == nil {
		logger = NilLogger
	}
	var episodes []int
	var parseErrors []string

	doc.Find(sectionSelector).Each(func(i int, s *goquery.Selection) {
		text := strings.TrimSpace(s.Text())
		if text == "" {
			return
		}
		if strings.Contains(text, "-") {
			parts := strings.Split(text, "-")
			start, end := parseRange(parts)
			if start > 0 && end >= start {
				for epNum := start; epNum <= end; epNum++ {
					episodes = append(episodes, epNum)
				}
			} else if start == 0 && end == 0 && (parts[0] != "0" || parts[1] != "0") {
				parseErrors = append(parseErrors, fmt.Sprintf("range '%s'", text))
			}
		} else {
			epNum := parseSingleEpisode(text)
			if epNum > 0 {
				episodes = append(episodes, epNum)
			} else if text != "0" {
				parseErrors = append(parseErrors, fmt.Sprintf("single '%s'", text))
			}
		}
	})

	if len(parseErrors) > 0 {
		log.Printf("  %s Parse warnings for selector '%s': %v", util.Yellow("[FILLER]"), sectionSelector, parseErrors)
	}

	uniqueEpisodesMap := make(map[int]struct{})
	uniqueEpisodesList := []int{}
	for _, ep := range episodes {
		if _, exists := uniqueEpisodesMap[ep]; !exists {
			uniqueEpisodesMap[ep] = struct{}{}
			uniqueEpisodesList = append(uniqueEpisodesList, ep)
		}
	}
	return uniqueEpisodesList, nil
}

func GetCategorizedCanonEpisodes(animeTitle string, includeTypesFromConfig []string, logger *log.Logger) (mangaEps, animeEps, mixedEps []int, err error) {
	if logger == nil {
		logger = NilLogger
	}

	baseURL := fmt.Sprintf("https://www.animefillerlist.com/shows/%s/", animeTitle)
	requestedTypesMap := make(map[string]bool)
	if len(includeTypesFromConfig) == 0 {
		requestedTypesMap["manga"] = true
		requestedTypesMap["mixed"] = true
		requestedTypesMap["anime"] = true
	} else {
		for _, t := range includeTypesFromConfig {
			typeName := strings.ToLower(strings.TrimSpace(t))
			if typeName == "manga" || typeName == "mixed" || typeName == "anime" {
				requestedTypesMap[typeName] = true
			}
		}
	}

	if len(requestedTypesMap) == 0 {
		logger.Printf("  %s No valid types specified or found for '%s'. Defaulting to all for processing.", util.Yellow("[FILLER]"), animeTitle)
		requestedTypesMap["manga"] = true
		requestedTypesMap["mixed"] = true
		requestedTypesMap["anime"] = true
	}

	logger.Printf("  %s Fetching %s from %s...",
		util.Purple("[FILLER]"),
		util.Red(fmt.Sprintf("'%s'", animeTitle)),
		util.Yellow("AnimeFillerList"))

	httpClient := &http.Client{Timeout: 15 * time.Second}
	req, httpErr := http.NewRequest("GET", baseURL, nil)
	if httpErr != nil {
		err = fmt.Errorf("failed create request: %w", httpErr)
		return
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")
	res, httpErr := httpClient.Do(req)
	if httpErr != nil {
		err = fmt.Errorf("failed GET URL %s: %w", baseURL, httpErr)
		return
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		err = fmt.Errorf("HTTP request failed with status %s for URL %s", res.Status, baseURL)
		return
	}
	doc, parseErr := goquery.NewDocumentFromReader(res.Body)
	if parseErr != nil {
		err = fmt.Errorf("failed parse HTML: %w", parseErr)
		return
	}

	mangaCanonSelector := "div.manga_canon span.Episodes a"
	mixedCanonSelector := "div.mixed_canon\\/filler span.Episodes a"
	animeCanonSelector := "div.anime_canon span.Episodes a"
	var scrapeErr error

	if requestedTypesMap["manga"] {
		mangaEps, scrapeErr = scrapeEpisodesFromSection(doc, mangaCanonSelector, logger)
		if scrapeErr != nil {
			logger.Printf("  %s Error scraping manga: %v", util.Yellow("[FILLER]"), scrapeErr)
		}
	}
	if requestedTypesMap["mixed"] {
		mixedEps, scrapeErr = scrapeEpisodesFromSection(doc, mixedCanonSelector, logger)
		if scrapeErr != nil {
			logger.Printf("  %s Error scraping mixed: %v", util.Yellow("[FILLER]"), scrapeErr)
		}
	}
	if requestedTypesMap["anime"] {
		animeEps, scrapeErr = scrapeEpisodesFromSection(doc, animeCanonSelector, logger)
		if scrapeErr != nil {
			logger.Printf("  %s Error scraping anime: %v", util.Yellow("[FILLER]"), scrapeErr)
		}
	}

	var countsParts []string
	if requestedTypesMap["manga"] {
		countsParts = append(countsParts, fmt.Sprintf("Manga: %d", len(mangaEps)))
	}
	if requestedTypesMap["mixed"] {
		countsParts = append(countsParts, fmt.Sprintf("Mixed: %d", len(mixedEps)))
	}
	if requestedTypesMap["anime"] {
		countsParts = append(countsParts, fmt.Sprintf("Anime: %d", len(animeEps)))
	}

	countsString := "No types processed or counts available"
	if len(countsParts) > 0 {
		countsString = strings.Join(countsParts, ", ")
	}
	logger.Printf("  %s Counts - %s.", util.Purple("[FILLER]"), countsString)

	return
}
