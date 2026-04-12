package helper

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"link-shortener/internal/db"
)

const maxRangeValue = int64(1<<31 - 1)

func ParseID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id < 1 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid link id",
		})
		return 0, false
	}

	return id, true
}

func ParseListLinksInput(c *gin.Context) (*db.ListLinksByRangeParams, bool) {
	rangeValue := c.Query("range")
	if rangeValue == "" {
		return nil, true
	}

	var values []int64
	if err := json.Unmarshal([]byte(rangeValue), &values); err != nil {
		respondInvalidRange(c)
		return nil, false
	}
	if len(values) != 2 || values[0] < 0 || values[1] < values[0] {
		respondInvalidRange(c)
		return nil, false
	}

	limit := values[1] - values[0] + 1
	if values[0] > maxRangeValue || limit > maxRangeValue {
		respondInvalidRange(c)
		return nil, false
	}

	return &db.ListLinksByRangeParams{
		Offset: int32(values[0]),
		Limit:  int32(limit),
	}, true
}

func ParseListLinkVisitsInput(c *gin.Context) (*db.ListLinkVisitsByRangeParams, bool) {
	rangeValue := c.Query("range")
	if rangeValue == "" {
		return nil, true
	}

	var values []int64
	if err := json.Unmarshal([]byte(rangeValue), &values); err != nil {
		respondInvalidRange(c)
		return nil, false
	}
	if len(values) != 2 || values[0] < 0 || values[1] < values[0] {
		respondInvalidRange(c)
		return nil, false
	}

	limit := values[1] - values[0] + 1
	if values[0] > maxRangeValue || limit > maxRangeValue {
		respondInvalidRange(c)
		return nil, false
	}

	return &db.ListLinkVisitsByRangeParams{
		Offset: int32(values[0]),
		Limit:  int32(limit),
	}, true
}

func respondInvalidRange(c *gin.Context) {
	c.JSON(http.StatusBadRequest, gin.H{
		"error": "invalid range",
	})
}
