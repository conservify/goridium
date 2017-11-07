package main

import (
	"github.com/conservify/goridium"
	"log"
)

func main() {
	rb, err := goridium.NewRockBlock("/dev/ttyUSB0")
	if err != nil {
		log.Fatalf("Unable to open RockBlock: %v", err)
	}

	defer rb.Close()

	err = rb.Ping()
	if err != nil {
		log.Fatalf("Unable to ping RockBlock: %v", err)
	}

	rb.EnableEcho()

	rb.DisableRingAlerts()

	rb.DisableFlowControl()

	_, err = rb.GetSignalStrength()
	if err != nil {
		log.Fatalf("Unable to get signal strength: %v", err)
	}

	_, err = rb.GetNetworkTime()
	if err != nil {
		log.Printf("Unable to get network time: %v", err)
	}

	_, err = rb.GetSerialIdentifier()
	if err != nil {
		log.Fatalf("Unable to get serial id: %v", err)
	}

	err = rb.QueueMessage("Hello, World")
	if err != nil {
		log.Fatalf("Unable to queue message: %v", err)
	}

	err = rb.AttemptConnection()
	if err != nil {
		log.Fatalf("Unable to establish connection: %v", err)
	}

	err = rb.AttemptSession()
	if err != nil {
		log.Fatalf("Unable to establish session: %v", err)
	}
}
