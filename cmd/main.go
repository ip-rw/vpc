package main

import (
	"fmt"
	"go.uber.org/zap"
	"time"
	"vpc/pkg/broker"
	"vpc/pkg/wireguard"
)

func main() {
	add := func() (*wireguard.Wireguard, error) {
		return broker.CreateWireguard("blah")
	}
	var wgs = make([]*wireguard.Wireguard, 2)
	var err error
	for i := 0; i < 2; i++ {
		wgs[i], err = add()
		defer wgs[i].Stop()
		if err != nil {
			wgs[i].Logger.Error("error", zap.Error(err))
		}
	}

	fmt.Println("Waiting 5 seconds...\n")
	time.Sleep(5 * time.Second)
}
