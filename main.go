package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"os/signal"
	"slashingMitigator/slashingMonitor"
	"strconv"
	"strings"
	"syscall"
)

type InputData struct {
	beaconNode string
	logger     *slog.Logger

	waitForSync bool

	shutdownCmd  string
	testShutdown bool

	validatorIndex []uint64
}

func parseInput() (*InputData, error) {
	data := &InputData{}

	beaconNode := flag.String("beacon-node", "http://localhost:5052", "Address of the beacon node")
	debugFlag := flag.Bool("debug", false, "Enable debug logging")
	waitForSyncFlag := flag.Bool("wait-for-sync", false, "Wait for the beacon node to sync before starting")
	shutdownCmdFlag := flag.String("shutdown-cmd", "rocketpool service stop -y", "Command to run on shutdown")
	shutdownTestFlag := flag.Bool("shutdown-test", false, "Run the shutdown command and exit")
	validatorIndexFlag := flag.String("validator-index", "", "Validator index to monitor")
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

	data.waitForSync = *waitForSyncFlag

	data.shutdownCmd = *shutdownCmdFlag
	if data.shutdownCmd == "" {
		return nil, fmt.Errorf("shutdown command cannot be empty")
	}

	data.testShutdown = *shutdownTestFlag

	// we dont need to check for validator index if we are testing shutdown
	if *validatorIndexFlag == "" {
		if !data.testShutdown {
			return nil, fmt.Errorf("validator index cannot be empty")
		} else {
			data.validatorIndex = []uint64{1} // this is only a dummy value for testing
		}
	} else {
		validatorIndexes := strings.Split(strings.TrimSpace(*validatorIndexFlag), ",")
		for _, idx := range validatorIndexes {
			parsedIdx, err := strconv.ParseUint(strings.TrimSpace(idx), 10, 64)
			if err != nil {
				return nil, fmt.Errorf("validator index: \"%s\" is invalid: %v", strings.TrimSpace(idx), err)
			}
			data.validatorIndex = append(data.validatorIndex, parsedIdx)
		}
	}

	return data, nil
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	os.Exit(0)
}

func run() error {
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
		return fmt.Errorf("error parsing input: %v", err)
	}

	sm, err := slashingMonitor.NewSlashingMonitor(ctx, data.logger, data.beaconNode, data.shutdownCmd, data.validatorIndex)
	if err != nil {
		return fmt.Errorf("error creating slashing mitigator: %v", err)
	}

	if data.testShutdown {
		return sm.ExecuteShutdown(ctx)
	}

	err = sm.Start(ctx, data.waitForSync)
	if err != nil {
		return fmt.Errorf("error starting slashing mitigator: %v", err)
	}

	data.logger.Info("Slashing mitigator started")

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

	data.logger.Info("Slashing mitigator stopped")
	return nil
}
