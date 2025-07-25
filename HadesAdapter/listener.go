package main

import (
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/nats-io/nats.go"
)

func ListenLog(nc *nats.Conn) {
	nc.Subscribe("nats topic to be entered", func(m *nats.Msg) {
		//Log strucutre from adapter, to be merged from PR
		var log Log
		if err := json.Unmarshal(m.Data, &log); err != nil {
			slog.Error("Failed to unmarshal log", slog.Any("error", err))
			return
		}
		//what ever message handling logic
		fmt.Printf("Received a message: %s\n", log)
	})
}
