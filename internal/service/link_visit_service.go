package service

import (
	"context"
	"fmt"
	"strings"

	"link-shortener/internal/db"
)

type LinkVisitService struct {
	queries *db.Queries
}

func NewLinkVisitService(queries *db.Queries) *LinkVisitService {
	return &LinkVisitService{
		queries: queries,
	}
}

func (service *LinkVisitService) List(ctx context.Context, input *db.ListLinkVisitsByRangeParams) ([]db.LinkVisit, int64, error) {
	count, err := service.queries.CountLinkVisits(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("can't count link visits: %w", err)
	}

	var visits []db.LinkVisit
	if input != nil {
		visits, err = service.queries.ListLinkVisitsByRange(ctx, *input)
	} else {
		visits, err = service.queries.ListLinkVisits(ctx)
	}
	if err != nil {
		return nil, 0, fmt.Errorf("can't list link visits: %w", err)
	}

	return visits, count, nil
}

func (service *LinkVisitService) Create(ctx context.Context, input db.CreateLinkVisitParams) (db.LinkVisit, error) {
	input.Ip = strings.TrimSpace(input.Ip)
	input.UserAgent = strings.TrimSpace(input.UserAgent)
	input.Referer = strings.TrimSpace(input.Referer)

	visit, err := service.queries.CreateLinkVisit(ctx, input)
	if err != nil {
		return db.LinkVisit{}, fmt.Errorf("can't create link visit: %w", err)
	}

	return visit, nil
}
