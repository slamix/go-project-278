package helper

import (
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"link-shortener/internal/db"
)

const defaultShortURLBase = "http://localhost:8080/r"

type LinkResponse struct {
	ID          int64  `json:"id"`
	OriginalURL string `json:"original_url"`
	ShortName   string `json:"short_name"`
	ShortURL    string `json:"short_url"`
}

type LinkVisitResponse struct {
	ID        int64     `json:"id"`
	LinkID    int64     `json:"link_id"`
	CreatedAt time.Time `json:"created_at"`
	IP        string    `json:"ip"`
	UserAgent string    `json:"user_agent"`
	Referer   string    `json:"referer"`
	Status    int32     `json:"status"`
}

func ToLinkResponses(links []db.Link, shortURLBase string) []LinkResponse {
	response := make([]LinkResponse, 0, len(links))
	for _, link := range links {
		response = append(response, ToLinkResponse(link, shortURLBase))
	}

	return response
}

func ToLinkVisitResponses(visits []db.LinkVisit) []LinkVisitResponse {
	response := make([]LinkVisitResponse, 0, len(visits))
	for _, visit := range visits {
		response = append(response, ToLinkVisitResponse(visit))
	}

	return response
}

func ToLinkVisitResponse(visit db.LinkVisit) LinkVisitResponse {
	return LinkVisitResponse{
		ID:        visit.ID,
		LinkID:    visit.LinkID,
		CreatedAt: visit.CreatedAt.Time,
		IP:        visit.Ip,
		UserAgent: visit.UserAgent,
		Referer:   visit.Referer,
		Status:    visit.Status,
	}
}

func ToLinkResponse(link db.Link, shortURLBase string) LinkResponse {
	return LinkResponse{
		ID:          link.ID,
		OriginalURL: link.OriginalUrl,
		ShortName:   link.ShortName,
		ShortURL:    shortURLBase + "/" + link.ShortName,
	}
}

func SetLinksContentRange(c *gin.Context, input *db.ListLinksByRangeParams, returned int, total int64) {
	start := int64(0)
	if input != nil {
		start = int64(input.Offset)
	}

	end := start + int64(returned) - 1
	if returned == 0 {
		end = start
	}

	c.Header("Content-Range", fmt.Sprintf("links %d-%d/%d", start, end, total))
}

func SetLinkVisitsContentRange(c *gin.Context, input *db.ListLinkVisitsByRangeParams, returned int, total int64) {
	start := int64(0)
	if input != nil {
		start = int64(input.Offset)
	}

	end := start + int64(returned) - 1
	if returned == 0 {
		end = start
	}

	c.Header("Content-Range", fmt.Sprintf("link_visits %d-%d/%d", start, end, total))
}

func NormalizeShortURLBase(shortURLBase string) string {
	shortURLBase = strings.TrimRight(strings.TrimSpace(shortURLBase), "/")
	if shortURLBase == "" {
		return defaultShortURLBase
	}

	return shortURLBase
}
