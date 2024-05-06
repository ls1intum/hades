package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Mtze/HadesCI/shared/payload"
	"github.com/Mtze/HadesCI/shared/utils"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	log "github.com/sirupsen/logrus"
)

const REDIS_IMAGE = "redis:7.2"

type APISuite struct {
	suite.Suite
	router *gin.Engine
	redisC testcontainers.Container
}

func (suite *APISuite) SetupSuite() {
	gin.SetMode(gin.TestMode)
	suite.router = setupRouter("")

	ctx := context.Background()
	req := testcontainers.ContainerRequest{
		Image:        REDIS_IMAGE,
		ExposedPorts: []string{"6379/tcp"},
		WaitingFor:   wait.ForLog("Ready to accept connections"),
	}
	var err error
	suite.redisC, err = testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		log.Fatalf("Could not start redis: %s", err)
	}

	endpoint, err := suite.redisC.Endpoint(ctx, "")
	log.Infof("Redis endpoint: %s", endpoint)
	if err != nil {
		log.Fatalf("Could not get redis endpoint: %s", err)
	}
	AsynqClient = utils.SetupQueueClient(endpoint, "", false)
	if AsynqClient == nil {
		log.Fatalf("Could not connect queue to redis")
	}
}

func (suite *APISuite) TearDownSuite() {
	ctx := context.Background()
	if err := suite.redisC.Terminate(ctx); err != nil {
		log.Fatalf("Could not stop redis: %s", err)
	}
}

func (suite *APISuite) TestPingRoute() {
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/ping", nil)
	suite.router.ServeHTTP(w, req)

	assert.Equal(suite.T(), 200, w.Code)
	assert.Equal(suite.T(), "{\"message\":\"pong\"}", w.Body.String())
}

func (suite *APISuite) TestAddBuildToQueueRoute() {
	w := httptest.NewRecorder()
	payload := payload.RESTPayload{
		Priority: 1,
		QueuePayload: payload.QueuePayload{
			Name:      "example",
			Timestamp: time.Now(),
			Metadata: map[string]string{
				"key1": "value1",
				"key2": "value2",
			},
			Steps: []payload.Step{
				{
					ID:          1,
					Name:        "step1",
					Image:       "image1",
					Script:      "script1",
					Metadata:    map[string]string{},
					CPULimit:    1,
					MemoryLimit: "1G",
				},
				{
					ID:          2,
					Name:        "step2",
					Image:       "image2",
					Script:      "script2",
					Metadata:    map[string]string{},
					CPULimit:    2,
					MemoryLimit: "2G",
				},
			},
		},
	}
	jsonValue, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", "/build", bytes.NewBuffer(jsonValue))
	req.Header.Set("Content-Type", "application/json")
	suite.router.ServeHTTP(w, req)

	assert.Equal(suite.T(), 200, w.Code)
}

func (suite *APISuite) TestInvalidMemoryLimit() {
	w := httptest.NewRecorder()
	payload := payload.RESTPayload{
		Priority: 1,
		QueuePayload: payload.QueuePayload{
			Name:      "example",
			Timestamp: time.Now(),
			Steps: []payload.Step{
				{
					ID:          1,
					Name:        "step1",
					Image:       "image1",
					Script:      "script1",
					Metadata:    map[string]string{},
					CPULimit:    1,
					MemoryLimit: "1XXXX",
				},
			},
		},
	}
	jsonValue, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", "/build", bytes.NewBuffer(jsonValue))
	req.Header.Set("Content-Type", "application/json")
	suite.router.ServeHTTP(w, req)

	assert.Equal(suite.T(), 400, w.Code)
	assert.Equal(suite.T(), "Failed to parse RAM limit", w.Body.String())
}

func (suite *APISuite) TestInvalidJSON() {
	w := httptest.NewRecorder()
	payload := struct {
		Priority int    `json:"priority"`
		TaskName string `json:"task_name"`
	}{
		Priority: 1,
		TaskName: "example",
	}
	jsonValue, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", "/build", bytes.NewBuffer(jsonValue))
	req.Header.Set("Content-Type", "application/json")
	suite.router.ServeHTTP(w, req)

	assert.Equal(suite.T(), 400, w.Code)
	assert.Equal(suite.T(), "Failed to bind JSON", w.Body.String())
}

func TestAPISuite(t *testing.T) {
	log.SetLevel(log.DebugLevel)
	suite.Run(t, new(APISuite))
}
