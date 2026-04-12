package handler

import (
	"github.com/gin-gonic/gin"

	"link-shortener/internal/handler/middleware"
	"link-shortener/internal/service"
)

type RouterOptions struct {
	SentryDSN    string
	ShortURLBase string
}

func NewRouter(
	linkService *service.LinkService,
	linkVisitService *service.LinkVisitService,
	options RouterOptions,
) *gin.Engine {
	middleware.InitSentry(options.SentryDSN)

	router := gin.New()
	router.TrustedPlatform = gin.PlatformCloudflare
	router.Use(middleware.CORS())
	router.Use(gin.Recovery())
	router.Use(gin.Logger())
	router.Use(middleware.Sentry())

	linkHandler := NewLinkHandler(linkService, linkVisitService, options.ShortURLBase)
	linkVisitHandler := NewLinkVisitHandler(linkVisitService)

	router.GET("/ping", linkHandler.Ping)
	router.GET("/r/:code", linkHandler.Redirect)

	api := router.Group("/api")
	api.GET("/links", linkHandler.List)
	api.POST("/links", linkHandler.Create)
	api.GET("/links/:id", linkHandler.Get)
	api.PUT("/links/:id", linkHandler.Update)
	api.DELETE("/links/:id", linkHandler.Delete)
	api.GET("/link_visits", linkVisitHandler.List)

	return router
}
