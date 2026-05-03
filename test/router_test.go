package test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"link-shortener/internal/db"
	"link-shortener/internal/handler"
	"link-shortener/internal/service"
)

const shortURLBase = "https://short.io/r"

type linkResponse struct {
	ID          int64  `json:"id"`
	OriginalURL string `json:"original_url"`
	ShortName   string `json:"short_name"`
	ShortURL    string `json:"short_url"`
}

type linkVisitResponse struct {
	ID        int64     `json:"id"`
	LinkID    int64     `json:"link_id"`
	CreatedAt time.Time `json:"created_at"`
	IP        string    `json:"ip"`
	UserAgent string    `json:"user_agent"`
	Referer   string    `json:"referer"`
	Status    int32     `json:"status"`
}

type apiErrorResponse struct {
	Error string `json:"error"`
}

type validationErrorResponse struct {
	Errors map[string]string `json:"errors"`
}

type memoryStore struct {
	links       map[int64]db.Link
	linkVisits  map[int64]db.LinkVisit
	nextID      int64
	nextVisitID int64
}

type memoryRow struct {
	link      db.Link
	linkVisit db.LinkVisit
	id        int64
	err       error
}

type memoryRows struct {
	links      []db.Link
	linkVisits []db.LinkVisit
	index      int
	closed     bool
	err        error
}

func init() {
	gin.SetMode(gin.TestMode)
	gin.DefaultWriter = io.Discard
}

func TestPingReturnsPong(t *testing.T) {
	router := newTestRouter()
	request := httptest.NewRequest(http.MethodGet, "/ping", nil)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, response.Code)
	}

	if body := response.Body.String(); body != "pong" {
		t.Fatalf("expected body %q, got %q", "pong", body)
	}
}

func TestUnknownRouteReturnsNotFound(t *testing.T) {
	router := newTestRouter()
	request := httptest.NewRequest(http.MethodGet, "/unknown", nil)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, response.Code)
	}
}

func TestCORSPreflightReturnsHeaders(t *testing.T) {
	router := newTestRouter()
	request := httptest.NewRequest(http.MethodOptions, "/api/links", nil)
	request.Header.Set("Origin", "http://localhost:5173")
	request.Header.Set("Access-Control-Request-Method", http.MethodPost)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d", http.StatusNoContent, response.Code)
	}
	if actual := response.Header().Get("Access-Control-Allow-Origin"); actual != "http://localhost:5173" {
		t.Fatalf("expected Access-Control-Allow-Origin %q, got %q", "http://localhost:5173", actual)
	}
	if actual := response.Header().Get("Access-Control-Expose-Headers"); actual != "Content-Range" {
		t.Fatalf("expected Access-Control-Expose-Headers %q, got %q", "Content-Range", actual)
	}
}

func TestCreateLinkReturnsCreatedObject(t *testing.T) {
	router := newTestRouter()
	response := performRequest(router, http.MethodPost, "/api/links", map[string]string{
		"original_url": "https://example.com/long-url",
		"short_name":   "exmpl",
	})

	if response.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, response.Code)
	}

	var body linkResponse
	decodeResponse(t, response, &body)

	assertLinkResponse(t, body, linkResponse{
		ID:          1,
		OriginalURL: "https://example.com/long-url",
		ShortName:   "exmpl",
		ShortURL:    "https://short.io/r/exmpl",
	})
}

func TestCreateLinkGeneratesShortName(t *testing.T) {
	router := newTestRouter()
	response := performRequest(router, http.MethodPost, "/api/links", map[string]string{
		"original_url": "https://example.com/long-url",
	})

	if response.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, response.Code)
	}

	var body linkResponse
	decodeResponse(t, response, &body)

	if body.ShortName == "" {
		t.Fatal("expected generated short_name")
	}
	if body.ShortURL != shortURLBase+"/"+body.ShortName {
		t.Fatalf("expected short_url %q, got %q", shortURLBase+"/"+body.ShortName, body.ShortURL)
	}
}

func TestCreateLinkReturnsBadRequestForInvalidJSON(t *testing.T) {
	router := newTestRouter()
	response := performRawRequest(router, http.MethodPost, "/api/links", `{"original_url":`)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, response.Code)
	}

	var body apiErrorResponse
	decodeResponse(t, response, &body)
	if body.Error != "invalid request" {
		t.Fatalf("expected error %q, got %q", "invalid request", body.Error)
	}
}

func TestCreateLinkReturnsValidationErrorForInvalidURL(t *testing.T) {
	router := newTestRouter()
	response := performRequest(router, http.MethodPost, "/api/links", map[string]string{
		"original_url": "not-a-url",
		"short_name":   "exmpl",
	})

	assertValidationError(t, response, "original_url", "url")
}

func TestCreateLinkReturnsValidationErrorForShortNameLength(t *testing.T) {
	router := newTestRouter()
	response := performRequest(router, http.MethodPost, "/api/links", map[string]string{
		"original_url": "https://example.com/long-url",
		"short_name":   "ab",
	})

	assertValidationError(t, response, "short_name", "min")
}

func TestCreateLinkReturnsConflictForDuplicateShortName(t *testing.T) {
	router := newTestRouter()
	performRequest(router, http.MethodPost, "/api/links", map[string]string{
		"original_url": "https://example.com/long-url",
		"short_name":   "exmpl",
	})

	response := performRequest(router, http.MethodPost, "/api/links", map[string]string{
		"original_url": "https://example.com/another-url",
		"short_name":   "exmpl",
	})

	assertValidationError(t, response, "short_name", "short name already in use")
}

func TestListLinksReturnsAllLinks(t *testing.T) {
	router := newTestRouter()
	performRequest(router, http.MethodPost, "/api/links", map[string]string{
		"original_url": "https://example.com/long-url",
		"short_name":   "exmpl",
	})
	performRequest(router, http.MethodPost, "/api/links", map[string]string{
		"original_url": "https://example.com/long-url2",
		"short_name":   "exmpl2",
	})

	response := performRequest(router, http.MethodGet, "/api/links", nil)
	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, response.Code)
	}
	assertContentRange(t, response, "links 0-1/2")
	assertExposedHeaders(t, response, "Content-Range")

	var body []linkResponse
	decodeResponse(t, response, &body)

	if len(body) != 2 {
		t.Fatalf("expected 2 links, got %d", len(body))
	}
	assertLinkResponse(t, body[0], linkResponse{
		ID:          1,
		OriginalURL: "https://example.com/long-url",
		ShortName:   "exmpl",
		ShortURL:    "https://short.io/r/exmpl",
	})
	assertLinkResponse(t, body[1], linkResponse{
		ID:          2,
		OriginalURL: "https://example.com/long-url2",
		ShortName:   "exmpl2",
		ShortURL:    "https://short.io/r/exmpl2",
	})
}

func TestListLinksReturnsContentRangeForEmptyList(t *testing.T) {
	router := newTestRouter()

	response := performRequest(router, http.MethodGet, "/api/links", nil)
	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, response.Code)
	}
	assertContentRange(t, response, "links 0-0/0")
}

func TestListLinksReturnsFirstTenLinksByRange(t *testing.T) {
	router := newTestRouter()
	createNumberedLinks(t, router, 12)

	response := performRequest(router, http.MethodGet, "/api/links?range=[0,9]", nil)
	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, response.Code)
	}
	assertContentRange(t, response, "links 0-9/12")

	var body []linkResponse
	decodeResponse(t, response, &body)

	if len(body) != 10 {
		t.Fatalf("expected 10 links, got %d", len(body))
	}
	if body[0].ID != 1 || body[9].ID != 10 {
		t.Fatalf("expected ids from 1 to 10, got %d and %d", body[0].ID, body[9].ID)
	}
}

func TestListLinksReturnsInclusiveRange(t *testing.T) {
	router := newTestRouter()
	createNumberedLinks(t, router, 12)

	response := performRequest(router, http.MethodGet, "/api/links?range=[5,10]", nil)
	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, response.Code)
	}
	assertContentRange(t, response, "links 5-10/12")

	var body []linkResponse
	decodeResponse(t, response, &body)

	if len(body) != 6 {
		t.Fatalf("expected 6 links, got %d", len(body))
	}
	if body[0].ID != 6 || body[5].ID != 11 {
		t.Fatalf("expected ids from 6 to 11, got %d and %d", body[0].ID, body[5].ID)
	}
}

func TestListLinksReturnsBadRequestForInvalidRange(t *testing.T) {
	router := newTestRouter()

	response := performRequest(router, http.MethodGet, "/api/links?range=[10,5]", nil)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, response.Code)
	}
}

func TestGetLinkByIDReturnsLink(t *testing.T) {
	router := newTestRouter()
	performRequest(router, http.MethodPost, "/api/links", map[string]string{
		"original_url": "https://example.com/long-url",
		"short_name":   "exmpl",
	})

	response := performRequest(router, http.MethodGet, "/api/links/1", nil)
	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, response.Code)
	}

	var body linkResponse
	decodeResponse(t, response, &body)

	assertLinkResponse(t, body, linkResponse{
		ID:          1,
		OriginalURL: "https://example.com/long-url",
		ShortName:   "exmpl",
		ShortURL:    "https://short.io/r/exmpl",
	})
}

func TestGetLinkByIDReturnsNotFound(t *testing.T) {
	router := newTestRouter()
	response := performRequest(router, http.MethodGet, "/api/links/404", nil)

	if response.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, response.Code)
	}
}

func TestUpdateLinkReturnsUpdatedLink(t *testing.T) {
	router := newTestRouter()
	performRequest(router, http.MethodPost, "/api/links", map[string]string{
		"original_url": "https://example.com/long-url",
		"short_name":   "exmpl",
	})

	response := performRequest(router, http.MethodPut, "/api/links/1", map[string]string{
		"original_url": "https://example.com/updated-url",
		"short_name":   "upd",
	})
	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, response.Code)
	}

	var body linkResponse
	decodeResponse(t, response, &body)

	assertLinkResponse(t, body, linkResponse{
		ID:          1,
		OriginalURL: "https://example.com/updated-url",
		ShortName:   "upd",
		ShortURL:    "https://short.io/r/upd",
	})
}

func TestUpdateLinkKeepsShortNameWhenItIsMissing(t *testing.T) {
	router := newTestRouter()
	performRequest(router, http.MethodPost, "/api/links", map[string]string{
		"original_url": "https://example.com/long-url",
		"short_name":   "exmpl",
	})

	response := performRequest(router, http.MethodPut, "/api/links/1", map[string]string{
		"original_url": "https://example.com/updated-url",
	})
	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, response.Code)
	}

	var body linkResponse
	decodeResponse(t, response, &body)

	assertLinkResponse(t, body, linkResponse{
		ID:          1,
		OriginalURL: "https://example.com/updated-url",
		ShortName:   "exmpl",
		ShortURL:    "https://short.io/r/exmpl",
	})
}

func TestUpdateLinkReturnsValidationErrorForInvalidURL(t *testing.T) {
	router := newTestRouter()
	performRequest(router, http.MethodPost, "/api/links", map[string]string{
		"original_url": "https://example.com/long-url",
		"short_name":   "exmpl",
	})

	response := performRequest(router, http.MethodPut, "/api/links/1", map[string]string{
		"original_url": "not-a-url",
		"short_name":   "upd",
	})

	assertValidationError(t, response, "original_url", "url")
}

func TestUpdateLinkReturnsValidationErrorForDuplicateShortName(t *testing.T) {
	router := newTestRouter()
	performRequest(router, http.MethodPost, "/api/links", map[string]string{
		"original_url": "https://example.com/long-url",
		"short_name":   "first",
	})
	performRequest(router, http.MethodPost, "/api/links", map[string]string{
		"original_url": "https://example.com/another-url",
		"short_name":   "second",
	})

	response := performRequest(router, http.MethodPut, "/api/links/2", map[string]string{
		"original_url": "https://example.com/updated-url",
		"short_name":   "first",
	})

	assertValidationError(t, response, "short_name", "short name already in use")
}

func TestUpdateLinkReturnsNotFound(t *testing.T) {
	router := newTestRouter()
	response := performRequest(router, http.MethodPut, "/api/links/404", map[string]string{
		"original_url": "https://example.com/updated-url",
		"short_name":   "upd",
	})

	if response.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, response.Code)
	}
}

func TestDeleteLinkRemovesLink(t *testing.T) {
	router := newTestRouter()
	performRequest(router, http.MethodPost, "/api/links", map[string]string{
		"original_url": "https://example.com/long-url",
		"short_name":   "exmpl",
	})

	deleteResponse := performRequest(router, http.MethodDelete, "/api/links/1", nil)
	if deleteResponse.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d", http.StatusNoContent, deleteResponse.Code)
	}
	if deleteResponse.Body.Len() != 0 {
		t.Fatalf("expected empty response body, got %q", deleteResponse.Body.String())
	}

	getResponse := performRequest(router, http.MethodGet, "/api/links/1", nil)
	if getResponse.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, getResponse.Code)
	}
}

func TestDeleteLinkReturnsNotFound(t *testing.T) {
	router := newTestRouter()
	response := performRequest(router, http.MethodDelete, "/api/links/404", nil)

	if response.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, response.Code)
	}
}

func TestRedirectByShortNameCreatesLinkVisit(t *testing.T) {
	router := newTestRouter()
	performRequest(router, http.MethodPost, "/api/links", map[string]string{
		"original_url": "https://example.com/long-url",
		"short_name":   "exmpl",
	})

	request := httptest.NewRequest(http.MethodGet, "/r/exmpl", nil)
	request.Header.Set("CF-Connecting-IP", "172.18.0.1")
	request.Header.Set("User-Agent", "curl/8.5.0")
	request.Header.Set("Referer", "https://source.example/page")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusFound {
		t.Fatalf("expected status %d, got %d", http.StatusFound, response.Code)
	}
	if location := response.Header().Get("Location"); location != "https://example.com/long-url" {
		t.Fatalf("expected Location %q, got %q", "https://example.com/long-url", location)
	}

	visitsResponse := performRequest(router, http.MethodGet, "/api/link_visits", nil)
	if visitsResponse.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, visitsResponse.Code)
	}
	assertContentRange(t, visitsResponse, "link_visits 0-0/1")

	var visits []linkVisitResponse
	decodeResponse(t, visitsResponse, &visits)
	if len(visits) != 1 {
		t.Fatalf("expected 1 link visit, got %d", len(visits))
	}

	expected := linkVisitResponse{
		ID:        1,
		LinkID:    1,
		IP:        "172.18.0.1",
		UserAgent: "curl/8.5.0",
		Referer:   "https://source.example/page",
		Status:    http.StatusFound,
	}
	if visits[0].ID != expected.ID ||
		visits[0].LinkID != expected.LinkID ||
		visits[0].IP != expected.IP ||
		visits[0].UserAgent != expected.UserAgent ||
		visits[0].Referer != expected.Referer ||
		visits[0].Status != expected.Status {
		t.Fatalf("expected visit %+v, got %+v", expected, visits[0])
	}
	if visits[0].CreatedAt.IsZero() {
		t.Fatal("expected created_at to be set")
	}
}

func TestRedirectByUnknownShortNameReturnsNotFound(t *testing.T) {
	router := newTestRouter()

	response := performRequest(router, http.MethodGet, "/r/unknown", nil)

	if response.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, response.Code)
	}
}

func TestListLinkVisitsReturnsInclusiveRange(t *testing.T) {
	router := newTestRouter()
	performRequest(router, http.MethodPost, "/api/links", map[string]string{
		"original_url": "https://example.com/long-url",
		"short_name":   "exmpl",
	})
	createLinkVisits(t, router, "exmpl", 12)

	response := performRequest(router, http.MethodGet, "/api/link_visits?range=[5,10]", nil)
	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, response.Code)
	}
	assertContentRange(t, response, "link_visits 5-10/12")

	var body []linkVisitResponse
	decodeResponse(t, response, &body)

	if len(body) != 6 {
		t.Fatalf("expected 6 link visits, got %d", len(body))
	}
	if body[0].ID != 6 || body[5].ID != 11 {
		t.Fatalf("expected ids from 6 to 11, got %d and %d", body[0].ID, body[5].ID)
	}

	headerRequest := httptest.NewRequest(http.MethodGet, "/api/link_visits", nil)
	headerRequest.Header.Set("Range", "[5,10]")
	headerResponse := httptest.NewRecorder()
	router.ServeHTTP(headerResponse, headerRequest)

	if headerResponse.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, headerResponse.Code)
	}
	assertContentRange(t, headerResponse, "link_visits 5-10/12")
}

func newTestRouter() *gin.Engine {
	store := &memoryStore{
		links:       make(map[int64]db.Link),
		linkVisits:  make(map[int64]db.LinkVisit),
		nextID:      1,
		nextVisitID: 1,
	}
	linkService := service.NewLinkService(db.New(store))
	linkVisitService := service.NewLinkVisitService(db.New(store))

	return handler.NewRouter(linkService, linkVisitService, handler.RouterOptions{
		ShortURLBase: shortURLBase,
	})
}

func performRequest(router *gin.Engine, method string, path string, payload any) *httptest.ResponseRecorder {
	var body io.Reader
	if payload != nil {
		payloadBytes, err := json.Marshal(payload)
		if err != nil {
			panic(err)
		}
		body = bytes.NewReader(payloadBytes)
	}

	request := httptest.NewRequest(method, path, body)
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	return response
}

func performRawRequest(router *gin.Engine, method string, path string, payload string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, strings.NewReader(payload))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	return response
}

func decodeResponse(t *testing.T, response *httptest.ResponseRecorder, target any) {
	t.Helper()

	if err := json.Unmarshal(response.Body.Bytes(), target); err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}
}

func assertValidationError(t *testing.T, response *httptest.ResponseRecorder, field string, expectedMessagePart string) {
	t.Helper()

	if response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected status %d, got %d", http.StatusUnprocessableEntity, response.Code)
	}

	var body validationErrorResponse
	decodeResponse(t, response, &body)

	message := body.Errors[field]
	if message == "" {
		t.Fatalf("expected validation error for %q, got %+v", field, body.Errors)
	}
	if expectedMessagePart != "" && !strings.Contains(message, expectedMessagePart) {
		t.Fatalf("expected validation error for %q to contain %q, got %q", field, expectedMessagePart, message)
	}
}

func assertLinkResponse(t *testing.T, actual linkResponse, expected linkResponse) {
	t.Helper()

	if actual != expected {
		t.Fatalf("expected link %+v, got %+v", expected, actual)
	}
}

func assertContentRange(t *testing.T, response *httptest.ResponseRecorder, expected string) {
	t.Helper()

	if actual := response.Header().Get("Content-Range"); actual != expected {
		t.Fatalf("expected Content-Range %q, got %q", expected, actual)
	}
}

func assertExposedHeaders(t *testing.T, response *httptest.ResponseRecorder, expected string) {
	t.Helper()

	if actual := response.Header().Get("Access-Control-Expose-Headers"); actual != expected {
		t.Fatalf("expected Access-Control-Expose-Headers %q, got %q", expected, actual)
	}
}

func createNumberedLinks(t *testing.T, router *gin.Engine, count int) {
	t.Helper()

	for index := 1; index <= count; index++ {
		response := performRequest(router, http.MethodPost, "/api/links", map[string]string{
			"original_url": "https://example.com/long-url-" + strconv.Itoa(index),
			"short_name":   "exmpl" + strconv.Itoa(index),
		})
		if response.Code != http.StatusCreated {
			t.Fatalf("expected status %d, got %d", http.StatusCreated, response.Code)
		}
	}
}

func createLinkVisits(t *testing.T, router *gin.Engine, shortName string, count int) {
	t.Helper()

	for index := 1; index <= count; index++ {
		request := httptest.NewRequest(http.MethodGet, "/r/"+shortName, nil)
		request.Header.Set("CF-Connecting-IP", "172.18.0."+strconv.Itoa(index))
		request.Header.Set("User-Agent", "curl/8.5.0")
		response := httptest.NewRecorder()

		router.ServeHTTP(response, request)

		if response.Code != http.StatusFound {
			t.Fatalf("expected status %d, got %d", http.StatusFound, response.Code)
		}
	}
}

func (store *memoryStore) Exec(context.Context, string, ...interface{}) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}

func (store *memoryStore) Query(_ context.Context, query string, args ...interface{}) (pgx.Rows, error) {
	if strings.Contains(query, "FROM link_visits") && strings.Contains(query, "ORDER BY id") {
		linkVisits := make([]db.LinkVisit, 0, len(store.linkVisits))
		for _, visit := range store.linkVisits {
			linkVisits = append(linkVisits, visit)
		}

		sort.Slice(linkVisits, func(i int, j int) bool {
			return linkVisits[i].ID < linkVisits[j].ID
		})

		if strings.Contains(query, "LIMIT") {
			limit := int(args[0].(int32))
			offset := int(args[1].(int32))

			if offset >= len(linkVisits) {
				return &memoryRows{}, nil
			}

			end := offset + limit
			if end > len(linkVisits) {
				end = len(linkVisits)
			}

			linkVisits = linkVisits[offset:end]
		}

		return &memoryRows{linkVisits: linkVisits}, nil
	}

	if strings.Contains(query, "FROM links") && strings.Contains(query, "ORDER BY id") {
		links := make([]db.Link, 0, len(store.links))
		for _, link := range store.links {
			links = append(links, link)
		}

		sort.Slice(links, func(i int, j int) bool {
			return links[i].ID < links[j].ID
		})

		if strings.Contains(query, "LIMIT") {
			limit := int(args[0].(int32))
			offset := int(args[1].(int32))

			if offset >= len(links) {
				return &memoryRows{}, nil
			}

			end := offset + limit
			if end > len(links) {
				end = len(links)
			}

			links = links[offset:end]
		}

		return &memoryRows{links: links}, nil
	}

	return nil, errors.New("unexpected query")
}

func (store *memoryStore) QueryRow(_ context.Context, query string, args ...interface{}) pgx.Row {
	switch {
	case strings.Contains(query, "SELECT COUNT(*)") && strings.Contains(query, "FROM link_visits"):
		return memoryRow{id: int64(len(store.linkVisits))}
	case strings.Contains(query, "SELECT COUNT(*)") && strings.Contains(query, "FROM links"):
		return memoryRow{id: int64(len(store.links))}
	case strings.Contains(query, "INSERT INTO link_visits"):
		linkID := args[0].(int64)
		if _, ok := store.links[linkID]; !ok {
			return memoryRow{err: pgx.ErrNoRows}
		}

		createdVisit := db.LinkVisit{
			ID:        store.nextVisitID,
			LinkID:    linkID,
			Ip:        args[1].(string),
			UserAgent: args[2].(string),
			Referer:   args[3].(string),
			Status:    args[4].(int32),
			CreatedAt: pgtype.Timestamptz{
				Time:  time.Now().UTC(),
				Valid: true,
			},
		}
		store.linkVisits[createdVisit.ID] = createdVisit
		store.nextVisitID++

		return memoryRow{linkVisit: createdVisit}
	case strings.Contains(query, "INSERT INTO links"):
		originalURL := args[0].(string)
		shortName := args[1].(string)
		if store.hasShortName(shortName, 0) {
			return memoryRow{err: &pgconn.PgError{Code: "23505"}}
		}

		createdLink := db.Link{
			ID:          store.nextID,
			OriginalUrl: originalURL,
			ShortName:   shortName,
		}
		store.links[createdLink.ID] = createdLink
		store.nextID++

		return memoryRow{link: createdLink}
	case strings.Contains(query, "SELECT id") && strings.Contains(query, "WHERE short_name = $1"):
		shortName := args[0].(string)
		for _, link := range store.links {
			if link.ShortName == shortName {
				return memoryRow{link: link}
			}
		}

		return memoryRow{err: pgx.ErrNoRows}
	case strings.Contains(query, "SELECT id") && strings.Contains(query, "WHERE id = $1"):
		id := args[0].(int64)
		foundLink, ok := store.links[id]
		if !ok {
			return memoryRow{err: pgx.ErrNoRows}
		}

		return memoryRow{link: foundLink}
	case strings.Contains(query, "UPDATE links"):
		id := args[0].(int64)
		originalURL := args[1].(string)
		shortName := args[2].(string)
		foundLink, ok := store.links[id]
		if !ok {
			return memoryRow{err: pgx.ErrNoRows}
		}
		if shortName == "" {
			shortName = foundLink.ShortName
		}
		if store.hasShortName(shortName, id) {
			return memoryRow{err: &pgconn.PgError{Code: "23505"}}
		}

		foundLink.OriginalUrl = originalURL
		foundLink.ShortName = shortName
		store.links[id] = foundLink

		return memoryRow{link: foundLink}
	case strings.Contains(query, "DELETE FROM links"):
		id := args[0].(int64)
		if _, ok := store.links[id]; !ok {
			return memoryRow{err: pgx.ErrNoRows}
		}

		delete(store.links, id)

		return memoryRow{id: id}
	default:
		return memoryRow{err: errors.New("unexpected query")}
	}
}

func (row memoryRow) Scan(dest ...any) error {
	if row.err != nil {
		return row.err
	}
	if len(dest) == 1 {
		*dest[0].(*int64) = row.id
		return nil
	}
	if len(dest) == 7 {
		return scanLinkVisit(dest, row.linkVisit)
	}

	return scanLink(dest, row.link)
}

func (rows *memoryRows) Close() {
	rows.closed = true
}

func (rows *memoryRows) Err() error {
	return rows.err
}

func (rows *memoryRows) CommandTag() pgconn.CommandTag {
	return pgconn.CommandTag{}
}

func (rows *memoryRows) FieldDescriptions() []pgconn.FieldDescription {
	return nil
}

func (rows *memoryRows) Next() bool {
	length := len(rows.links)
	if rows.linkVisits != nil {
		length = len(rows.linkVisits)
	}

	if rows.index >= length {
		rows.Close()
		return false
	}

	rows.index++

	return true
}

func (rows *memoryRows) Scan(dest ...any) error {
	if len(dest) == 7 {
		return scanLinkVisit(dest, rows.linkVisits[rows.index-1])
	}

	return scanLink(dest, rows.links[rows.index-1])
}

func (rows *memoryRows) Values() ([]any, error) {
	if len(rows.linkVisits) > 0 {
		visit := rows.linkVisits[rows.index-1]
		return []any{visit.ID, visit.LinkID, visit.Ip, visit.UserAgent, visit.Referer, visit.Status, visit.CreatedAt}, nil
	}

	link := rows.links[rows.index-1]
	return []any{link.ID, link.OriginalUrl, link.ShortName, link.CreatedAt}, nil
}

func (rows *memoryRows) RawValues() [][]byte {
	return nil
}

func (rows *memoryRows) Conn() *pgx.Conn {
	return nil
}

func scanLink(dest []any, link db.Link) error {
	*dest[0].(*int64) = link.ID
	*dest[1].(*string) = link.OriginalUrl
	*dest[2].(*string) = link.ShortName
	*dest[3].(*pgtype.Timestamptz) = link.CreatedAt

	return nil
}

func scanLinkVisit(dest []any, visit db.LinkVisit) error {
	*dest[0].(*int64) = visit.ID
	*dest[1].(*int64) = visit.LinkID
	*dest[2].(*string) = visit.Ip
	*dest[3].(*string) = visit.UserAgent
	*dest[4].(*string) = visit.Referer
	*dest[5].(*int32) = visit.Status
	*dest[6].(*pgtype.Timestamptz) = visit.CreatedAt

	return nil
}

func (store *memoryStore) hasShortName(shortName string, ignoredID int64) bool {
	for id, link := range store.links {
		if id != ignoredID && link.ShortName == shortName {
			return true
		}
	}

	return false
}
