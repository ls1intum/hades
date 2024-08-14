package payload

import (
	"strconv"
	"time"

	"github.com/google/uuid"
)

type RESTPayload struct {
	Priority int `json:"priority"`
	QueuePayload
}

type QueuePayload struct {
	ID        uuid.UUID         `json:"id"`
	Name      string            `json:"name" binding:"required"`
	Timestamp time.Time         `json:"timestamp"`
	Metadata  map[string]string `json:"metadata"`
	Steps     []Step            `json:"steps"`
}

type Step struct {
	ID          int               `json:"id"`
	Name        string            `json:"name"`
	Image       string            `json:"image"`
	Script      string            `json:"script"`
	Metadata    map[string]string `json:"metadata"`
	CPULimit    uint              `json:"cpu_limit"`
	MemoryLimit string            `json:"memory_limit"`
}

func (s Step) IDstring() string {
	return strconv.Itoa(s.ID)
}
