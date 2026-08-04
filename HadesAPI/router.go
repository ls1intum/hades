package main

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"reflect"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	hades "github.com/ls1intum/hades/shared"
	"github.com/ls1intum/hades/shared/payload"
	"github.com/ls1intum/hades/shared/utils"
)

func setupRouter(authKey string, producer hades.JobPublisher) *gin.Engine {
	r := gin.New()
	r.Use(gin.ErrorLogger())
	r.Use(gin.Recovery())

	// Report validation errors using the JSON field names (e.g. "name")
	// rather than the Go struct field names (e.g. "Name").
	if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
		v.RegisterTagNameFunc(func(fld reflect.StructField) string {
			name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]
			if name == "-" {
				return ""
			}
			return name
		})
	}

	r.GET("/ping", ping)

	api := r.Group("/")
	if authKey == "" {
		slog.Warn("No auth key set")
	} else {
		slog.Info("Auth key set")
		api.Use(gin.BasicAuth(gin.Accounts{"hades": authKey}))
	}

	api.POST("/build", func(c *gin.Context) {
		addBuildToQueue(c, producer)
	})
	return r
}

func ping(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "ok",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}

// bindErrorMessage turns a request-binding error into a human-readable message
// that names the offending field(s). Validation failures (e.g. a missing
// required field) are reported per field; any other error is treated as a
// malformed request body.
func bindErrorMessage(err error) string {
	var verrs validator.ValidationErrors
	if errors.As(err, &verrs) {
		parts := make([]string, 0, len(verrs))
		for _, fe := range verrs {
			switch fe.Tag() {
			case "required":
				parts = append(parts, fmt.Sprintf("%q is required", fe.Field()))
			default:
				parts = append(parts, fmt.Sprintf("%q failed %q validation", fe.Field(), fe.Tag()))
			}
		}
		return "Invalid request payload: " + strings.Join(parts, "; ")
	}
	return "Invalid JSON body"
}

func addBuildToQueue(c *gin.Context, producer hades.JobPublisher) {
	var p payload.RESTPayload
	p.Priority = 3
	if err := c.ShouldBind(&p); err != nil {
		msg := bindErrorMessage(err)
		slog.Error("Failed to bind request payload", "error", err)
		c.String(http.StatusBadRequest, msg)
		return
	}

	for _, step := range p.QueuePayload.Steps {
		if step.MemoryLimit != "" {
			if _, err := utils.ParseMemoryLimit(step.MemoryLimit); err != nil {
				slog.Error("Failed to parse RAM limit", "error", err)
				c.String(http.StatusBadRequest, "Failed to parse RAM limit")
				return
			}
		}
	}

	p.QueuePayload.ID = uuid.New()
	slog.Debug("Received build request ", "payload", SafePayloadFormat(p.QueuePayload))

	queuePrio := hades.PriorityFromInt(p.Priority)

	err := producer.EnqueueJobWithPriority(c.Request.Context(), p.QueuePayload, queuePrio)
	if err != nil {
		slog.Error("Failed to enqueue job", "error", err)
		c.String(http.StatusInternalServerError, "Failed to enqueue job")
		return
	}

	slog.Info("Successfully enqueued job", "job_id", p.QueuePayload.ID.String())
	c.JSON(http.StatusOK, gin.H{
		"message": "Successfully enqueued job",
		"job_id":  p.QueuePayload.ID.String(),
	})
}
