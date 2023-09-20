package queue

import "encoding/json"

type TypedMessage struct {
	Body string `json:"body"`
	Type int    `json:"type"`
}

func (t TypedMessage) MarshalJSON() ([]byte, error) {
	return json.Marshal(t)
}

func (t TypedMessage) UnmarshalJSON(data []byte) error {
	return json.Unmarshal(data, &t)
}
