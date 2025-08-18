package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	logs "github.com/ls1intum/hades/shared/buildlogs"
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

func ConsumeLog(nc *nats.Conn, ctx context.Context) {
	consumer, err := logs.NewHadesLogConsumer(nc)

	if err != nil {
		slog.Error("something")
	}

	err = consumer.ConsumeLog(ctx, func(buildLog logs.Log) {

		// log processing logic here
		fmt.Printf("Received log for job %s: %s\n", buildLog.JobID, buildLog.Logs)
	})
	if err != nil {
		slog.Error("something")
	}
}
