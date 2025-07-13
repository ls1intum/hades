package hadesadapter

import (
	"fmt"

	"github.com/nats-io/nats.go"
)

func ListenLog(nc *nats.Conn) {
	nc.Subscribe("nats topic to be entered", func(m *nats.Msg) {
		//what ever message handling logic
		fmt.Printf("Received a message: %s\n", string(m.Data))
	})
}
