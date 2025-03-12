package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"os/signal"
	"slashingMitigator/slashingMitigator"
	"syscall"
)

type InputData struct {
	beaconNode string
	logger     *slog.Logger
}

func parseInput() (*InputData, error) {
	data := &InputData{}

	beaconNode := flag.String("beacon-node", "http://localhost:5052", "Address of the beacon node")
	debugFlag := flag.Bool("debug", false, "Enable debug logging")
	flag.Parse()

	data.beaconNode = *beaconNode

	_, err := url.ParseRequestURI(data.beaconNode)
	if err != nil {
		return nil, fmt.Errorf("invalid beacon node address")
	}

	if *debugFlag {
		data.logger = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	} else {
		data.logger = slog.Default()
	}

	return data, nil
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		<-c
		cancel()
	}()

	data, err := parseInput()
	if err != nil {
		fmt.Println("Error parsing input: ", err.Error())
		return
	}

	sm, err := slashingMitigator.NewSlashingMitigator(ctx, data.logger, data.beaconNode, []uint64{791764, 5, 10})
	if err != nil {
		fmt.Println("Error creating slashing mitigator: ", err.Error())
		return
	}

	err = sm.Start(ctx)
	if err != nil {
		fmt.Println("Error starting slashing mitigator: ", err.Error())
		return
	}

	fmt.Println("Starting slashing mitigator...")

	// Wait for signal.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	for {
		sig := <-sigCh
		if sig == syscall.SIGINT || sig == syscall.SIGTERM || sig == os.Interrupt || sig == os.Kill {
			break
		}
	}

	sm.Stop()

	fmt.Println("Exiting...")
}
