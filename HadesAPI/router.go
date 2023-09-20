package main

import (
	"net/http"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
)

type Payload struct {
	Credentials struct {
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
	} `json:"credentials" binding:"required"`
	BuildConfig struct {
		Repositories       []Repository `json:"repositories" binding:"required,dive"`
		ExecutionContainer string       `json:"executionContainer" binding:"required"`
	} `json:"buildConfig" binding:"required"`
}

type Repository struct {
	URL  string `json:"url" binding:"required,url"`
	Path string `json:"path" binding:"required,dirpath"`
}

func ping(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message": "pong",
	})
}

func AddBuildToQueue(c *gin.Context) {
	var payload Payload
	if err := c.ShouldBind(&payload); err != nil {
		log.WithError(err).Error("Failed to bind JSON")
		c.String(http.StatusBadRequest, "Failed to bind JSON")
		return
	}

	log.Debug("Received build request ", payload)
}
