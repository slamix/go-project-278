package service

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"link-shortener/internal/db"
)

const (
	generatedShortNameLength = 8
	maxGenerateAttempts      = 10
	uniqueViolationCode      = "23505"
)

var (
	ErrConflict     = errors.New("short_name already exists")
	ErrInvalidInput = errors.New("invalid link payload")
	ErrNotFound     = errors.New("link not found")
)

type LinkService struct {
	queries *db.Queries
}

func NewLinkService(queries *db.Queries) *LinkService {
	return &LinkService{
		queries: queries,
	}
}

func (service *LinkService) List(ctx context.Context, input *db.ListLinksByRangeParams) ([]db.Link, int64, error) {
	count, err := service.queries.CountLinks(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("can't count links: %w", err)
	}

	var (
		links []db.Link
	)

	if input != nil {
		links, err = service.queries.ListLinksByRange(ctx, *input)
	} else {
		links, err = service.queries.ListLinks(ctx)
	}
	if err != nil {
		return nil, 0, fmt.Errorf("can't list links: %w", err)
	}

	return links, count, nil
}

func (service *LinkService) Create(ctx context.Context, input db.CreateLinkParams) (db.Link, error) {
	input = normalizeCreateInput(input)
	if input.OriginalUrl == "" {
		return db.Link{}, ErrInvalidInput
	}

	if input.ShortName != "" {
		return service.create(ctx, input)
	}

	for range maxGenerateAttempts {
		shortName, err := generateShortName()
		if err != nil {
			return db.Link{}, err
		}

		link, err := service.create(ctx, db.CreateLinkParams{
			OriginalUrl: input.OriginalUrl,
			ShortName:   shortName,
		})
		if errors.Is(err, ErrConflict) {
			continue
		}

		return link, err
	}

	return db.Link{}, ErrConflict
}

func (service *LinkService) Get(ctx context.Context, id int64) (db.Link, error) {
	link, err := service.queries.GetLink(ctx, id)
	if err != nil {
		return db.Link{}, mapQueryError("can't get link", err)
	}

	return link, nil
}

func (service *LinkService) GetByShortName(ctx context.Context, shortName string) (db.Link, error) {
	link, err := service.queries.GetLinkByShortName(ctx, strings.TrimSpace(shortName))
	if err != nil {
		return db.Link{}, mapQueryError("can't get link by short name", err)
	}

	return link, nil
}

func (service *LinkService) Update(ctx context.Context, input db.UpdateLinkParams) (db.Link, error) {
	input = normalizeUpdateInput(input)
	if input.OriginalUrl == "" || input.ShortName == "" {
		return db.Link{}, ErrInvalidInput
	}

	link, err := service.queries.UpdateLink(ctx, input)
	if err != nil {
		return db.Link{}, mapQueryError("can't update link", err)
	}

	return link, nil
}

func (service *LinkService) Delete(ctx context.Context, id int64) error {
	_, err := service.queries.DeleteLink(ctx, id)
	if err != nil {
		return mapQueryError("can't delete link", err)
	}

	return nil
}

func (service *LinkService) create(ctx context.Context, input db.CreateLinkParams) (db.Link, error) {
	link, err := service.queries.CreateLink(ctx, input)
	if err != nil {
		return db.Link{}, mapQueryError("can't create link", err)
	}

	return link, nil
}

func normalizeCreateInput(input db.CreateLinkParams) db.CreateLinkParams {
	return db.CreateLinkParams{
		OriginalUrl: strings.TrimSpace(input.OriginalUrl),
		ShortName:   strings.TrimSpace(input.ShortName),
	}
}

func normalizeUpdateInput(input db.UpdateLinkParams) db.UpdateLinkParams {
	return db.UpdateLinkParams{
		ID:          input.ID,
		OriginalUrl: strings.TrimSpace(input.OriginalUrl),
		ShortName:   strings.TrimSpace(input.ShortName),
	}
}

func generateShortName() (string, error) {
	const alphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

	shortName := make([]byte, generatedShortNameLength)
	for index := range shortName {
		randomIndex, err := rand.Int(rand.Reader, big.NewInt(int64(len(alphabet))))
		if err != nil {
			return "", err
		}
		shortName[index] = alphabet[randomIndex.Int64()]
	}

	return string(shortName), nil
}

func mapQueryError(operation string, err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}

	var pgError *pgconn.PgError
	if errors.As(err, &pgError) && pgError.Code == uniqueViolationCode {
		return ErrConflict
	}

	return fmt.Errorf("%s: %w", operation, err)
}
