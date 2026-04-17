package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"link-shortener/internal/db"
	"link-shortener/internal/handler/helper"
	"link-shortener/internal/service"
)

type LinkHandler struct {
	linkService      *service.LinkService
	linkVisitService *service.LinkVisitService
	shortURLBase     string
}

type createLinkPayload struct {
	OriginalURL string `json:"original_url" validate:"required,url"`
	ShortName   string `json:"short_name" validate:"omitempty,min=3,max=32"`
}

type updateLinkPayload struct {
	OriginalURL string `json:"original_url" validate:"required,url"`
	ShortName   string `json:"short_name" validate:"omitempty,min=3,max=32"`
}

func NewLinkHandler(
	linkService *service.LinkService,
	linkVisitService *service.LinkVisitService,
	shortURLBase string,
) *LinkHandler {
	return &LinkHandler{
		linkService:      linkService,
		linkVisitService: linkVisitService,
		shortURLBase:     helper.NormalizeShortURLBase(shortURLBase),
	}
}

func (handler *LinkHandler) Ping(c *gin.Context) {
	c.String(http.StatusOK, "pong")
}

func (handler *LinkHandler) List(c *gin.Context) {
	input, ok := helper.ParseListLinksInput(c)
	if !ok {
		return
	}

	links, total, err := handler.linkService.List(c.Request.Context(), input)
	if err != nil {
		helper.RespondError(c, err)
		return
	}

	helper.SetLinksContentRange(c, input, len(links), total)
	c.JSON(http.StatusOK, helper.ToLinkResponses(links, handler.shortURLBase))
}

func (handler *LinkHandler) Create(c *gin.Context) {
	var payload createLinkPayload
	if ok := helper.DecodeAndValidateJSON(c, &payload); !ok {
		return
	}

	params := db.CreateLinkParams{
		OriginalUrl: payload.OriginalURL,
		ShortName:   payload.ShortName,
	}

	link, err := handler.linkService.Create(c.Request.Context(), params)
	if err != nil {
		helper.RespondError(c, err)
		return
	}

	c.JSON(http.StatusCreated, helper.ToLinkResponse(link, handler.shortURLBase))
}

func (handler *LinkHandler) Get(c *gin.Context) {
	id, ok := helper.ParseID(c)
	if !ok {
		return
	}

	link, err := handler.linkService.Get(c.Request.Context(), id)
	if err != nil {
		helper.RespondError(c, err)
		return
	}

	c.JSON(http.StatusOK, helper.ToLinkResponse(link, handler.shortURLBase))
}

func (handler *LinkHandler) Update(c *gin.Context) {
	id, ok := helper.ParseID(c)
	if !ok {
		return
	}

	var payload updateLinkPayload
	if ok := helper.DecodeAndValidateJSON(c, &payload); !ok {
		return
	}

	params := db.UpdateLinkParams{
		ID:          id,
		OriginalUrl: payload.OriginalURL,
		ShortName:   payload.ShortName,
	}

	link, err := handler.linkService.Update(c.Request.Context(), params)
	if err != nil {
		helper.RespondError(c, err)
		return
	}

	c.JSON(http.StatusOK, helper.ToLinkResponse(link, handler.shortURLBase))
}

func (handler *LinkHandler) Delete(c *gin.Context) {
	id, ok := helper.ParseID(c)
	if !ok {
		return
	}

	if err := handler.linkService.Delete(c.Request.Context(), id); err != nil {
		helper.RespondError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

func (handler *LinkHandler) Redirect(c *gin.Context) {
	link, err := handler.linkService.GetByShortName(c.Request.Context(), c.Param("code"))
	if err != nil {
		helper.RespondError(c, err)
		return
	}

	status := http.StatusFound
	_, err = handler.linkVisitService.Create(c.Request.Context(), db.CreateLinkVisitParams{
		LinkID:    link.ID,
		Ip:        c.ClientIP(),
		UserAgent: c.Request.UserAgent(),
		Referer:   c.Request.Referer(),
		Status:    int32(status),
	})
	if err != nil {
		helper.RespondError(c, err)
		return
	}

	c.Redirect(status, link.OriginalUrl)
}
