package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"link-shortener/internal/handler/helper"
	"link-shortener/internal/service"
)

type LinkVisitHandler struct {
	linkVisitService *service.LinkVisitService
}

func NewLinkVisitHandler(linkVisitService *service.LinkVisitService) *LinkVisitHandler {
	return &LinkVisitHandler{
		linkVisitService: linkVisitService,
	}
}

func (handler *LinkVisitHandler) List(c *gin.Context) {
	input, ok := helper.ParseListLinkVisitsInput(c)
	if !ok {
		return
	}

	visits, total, err := handler.linkVisitService.List(c.Request.Context(), input)
	if err != nil {
		helper.RespondError(c, err)
		return
	}

	helper.SetLinkVisitsContentRange(c, input, len(visits), total)
	c.JSON(http.StatusOK, helper.ToLinkVisitResponses(visits))
}
