package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ls1intum/hades/shared/payload"
	"github.com/ls1intum/hades/shared/utils"

	"github.com/gin-gonic/gin"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	log "github.com/sirupsen/logrus"
)

const NATS_IMAGE = "nats:2.11.4"

type APISuite struct {
	suite.Suite
	router         *gin.Engine
	natsC          testcontainers.Container
	natsConnection *nats.Conn
	hadesProducer  *utils.HadesProducer
}

func (suite *APISuite) SetupSuite() {
	gin.SetMode(gin.TestMode)
	suite.router = setupRouter("")

	ctx := context.Background()
	req := testcontainers.ContainerRequest{
		Image:        NATS_IMAGE,
		ExposedPorts: []string{"4222/tcp", "8222/tcp"},
		Cmd:          []string{"-js", "-m", "8222"},
		WaitingFor:   wait.ForHTTP("/healthz").WithPort("8222/tcp"),
	}
	var err error
	suite.natsC, err = testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		log.Fatalf("Could not start NATS: %s", err)
	}

	endpoint, err := suite.natsC.Endpoint(ctx, "4222/tcp")
	log.Infof("NATS endpoint: %s", endpoint)
	if err != nil {
		log.Fatalf("Could not get NATS endpoint: %s", err)
	}

	// Setup NATS connection
	natsConfig := utils.NatsConfig{
		URL:      "nats://" + endpoint,
		Username: "",
		Password: "",
	}

	suite.natsConnection, err = utils.SetupNatsConnection(natsConfig)
	if err != nil {
		log.Fatalf("Failed to connect to NATS: %v", err)
	}

	// Create producer for tests
	suite.hadesProducer, err = utils.NewHadesProducer(suite.natsConnection)
	if err != nil {
		log.Fatalf("Failed to create HadesProducer: %v", err)
	}

	// Set the global HadesProducer for tests
	HadesProducer = suite.hadesProducer
}

func (suite *APISuite) TearDownSuite() {
	// Close NATS connection
	if suite.natsConnection != nil {
		suite.natsConnection.Close()
	}

	// Stop NATS container
	ctx := context.Background()
	if err := suite.natsC.Terminate(ctx); err != nil {
		log.Fatalf("Could not stop NATS: %s", err)
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
