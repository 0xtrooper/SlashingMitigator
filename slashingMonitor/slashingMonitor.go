package slashingMonitor

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"time"

	nmc_client "github.com/rocket-pool/node-manager-core/beacon/client"
	nmc_utils "github.com/rocket-pool/node-manager-core/utils"
)

const (
	SyncSleepTime = 30
)

type SlashingMonitor struct {
	logger *slog.Logger

	lastSlot uint64

	shutdownCmd string

	beaconClient     *nmc_client.BeaconHttpProvider
	indexesToMonitor []uint64

	eventLoopCancelFucn context.CancelFunc
}

func NewSlashingMonitor(ctx context.Context, logger *slog.Logger, beaconNode string, shutdownCmd string, indexesToMonitor []uint64) (*SlashingMonitor, error) {
	return &SlashingMonitor{
		logger:           logger.With("module", "slashingMitigator"),
		shutdownCmd:      shutdownCmd,
		beaconClient:     nmc_client.NewBeaconHttpProvider(beaconNode, time.Second*12),
		indexesToMonitor: indexesToMonitor,
	}, nil
}

func (sm *SlashingMonitor) CheckBeaconNode(ctx context.Context, waitForSync bool) (err error) {
	var syncStatus nmc_client.SyncStatusResponse

	if !waitForSync {
		// Non-blocking mode: try once, return an error if unsynced or unreachable.
		syncStatus, err = sm.beaconClient.Node_Syncing(ctx)
		if err != nil {
			return fmt.Errorf("error connecting to beacon node: %w", err)
		}
		if syncStatus.Data.IsSyncing {
			return fmt.Errorf("beacon node is not synced, sync distance: %d", syncStatus.Data.SyncDistance)
		}
	} else {
		// Blocking mode: keep polling until the node is synced.
		for {
			// Check if the context has been canceled.
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			syncStatus, err = sm.beaconClient.Node_Syncing(ctx)
			if err != nil {
				sm.logger.Warn("error connecting to beacon node, waiting for a connection...", slog.String("error", err.Error()))
				time.Sleep(time.Second * SyncSleepTime)
				continue
			}

			if !syncStatus.Data.IsSyncing {
				break
			}

			sm.logger.Info("waiting for beacon node to sync...", slog.Uint64("syncDistance", uint64(syncStatus.Data.SyncDistance)))
			time.Sleep(time.Second * SyncSleepTime)
		}
	}

	sm.lastSlot = uint64(syncStatus.Data.HeadSlot)
	sm.logger.Info("beacon node connected and synced", slog.Uint64("headSlot", sm.lastSlot))
	return nil
}

func (sm *SlashingMonitor) Start(ctx context.Context, waitForSync bool) error {
	err := sm.CheckBeaconNode(ctx, waitForSync)
	if err != nil {
		return err
	}

	// create event stream for new heads
	newHeadStream, streamCancelFunc, err := sm.beaconClient.Beacon_Event_Stream(ctx, []string{"head"})
	if err != nil {
		return errors.Join(errors.New("error creating event stream for new heads"), err)
	}
	sm.eventLoopCancelFucn = streamCancelFunc

	// start monitoring new heads
	go sm.monitorNewHeads(ctx, newHeadStream)
	return nil
}

func (sm *SlashingMonitor) Stop() {
	if sm.eventLoopCancelFucn != nil {
		sm.eventLoopCancelFucn()
	}
}

func (sm *SlashingMonitor) ExecuteShutdown(ctx context.Context) error {
	logger := sm.logger.With("function", "shutdown")

	if sm.shutdownCmd == "" {
		logger.Error("shutdown command is empty")
		return fmt.Errorf("shutdown command is empty, cannot execute showdown")
	}

	parts := strings.Fields(sm.shutdownCmd)
	logger.Info("Running shutdown command", slog.String("cmd", parts[0]), slog.String("args", strings.Join(parts[1:], " ")))
	output, err := exec.Command(parts[0], parts[1:]...).Output()
	if err != nil {
		logger.Error("Error executing shutdown command",
			slog.String("command", sm.shutdownCmd),
			slog.String("output", string(output)),
			slog.String("error", err.Error()),
		)
		return fmt.Errorf("error executing shutdown command: %v", err)
	}

	logger.Info("Shutdown command executed successfully")
	sm.Stop() // stop the event loop
	return nil
}

func (sm *SlashingMonitor) monitorNewHeads(ctx context.Context, newHeadStream chan nmc_client.Event) {
	logger := sm.logger.With("function", "monitorNewHeads")
	for {
		select {
		case <-ctx.Done():
			sm.eventLoopCancelFucn() // stop the event stream
			return
		case event := <-newHeadStream:
			if event.Error != nil {
				logger.Warn("Received event with error", slog.String("error", event.Error.Error()))
				continue
			}

			if event.Data == nil {
				logger.Warn("Received event with no data")
				continue
			}

			eventData, ok := event.Data.(*nmc_client.HeadEvent)
			if !ok {
				logger.Warn("Received event with unexpected data type", slog.Any("data", event.Data))
				continue
			}

			logger.Debug("Received new head event", slog.String("slot", eventData.Slot))

			foundSlashing, err := sm.CheckBeaconBlock(ctx, eventData.Slot, eventData.Block)
			if err != nil {
				logger.Error("Error checking beacon block", slog.String("blockId", eventData.Block), slog.String("error", err.Error()))
				continue
			}

			if foundSlashing {
				logger.Info("Slashing detected in block", slog.String("blockId", eventData.Block))
				sm.ExecuteShutdown(ctx)
			}
		}
	}
}

func (sm *SlashingMonitor) CheckBeaconBlock(ctx context.Context, newSlotStr, blockID string) (bool, error) {
	newSlot, err := strconv.ParseUint(newSlotStr, 10, 64)
	if err != nil {
		// this should not happen, but if it does, try to get the slot number from the block ID
		sm.logger.Error("Error parsing slot number", slog.String("slot", newSlotStr))

		block, found, err := sm.beaconClient.Beacon_Block(ctx, blockID)
		if err != nil {
			return false, errors.Join(errors.New("error getting block from beacon node"), err)
		}

		if !found {
			slog.Warn("Block not found in beacon node", slog.String("blockId", blockID))
			return false, nil
		}

		newSlot = uint64(block.Data.Message.Slot)
	}

	// check for slashing in all blocks between the last processed slot and the new slot
	for slot := sm.lastSlot + 1; slot <= newSlot; slot++ {
		sm.logger.Debug("Checking block for slashing", slog.Uint64("slot", slot))
		slotStr := strconv.FormatUint(slot, 10)
		block, found, err := sm.beaconClient.Beacon_Block(ctx, slotStr)
		if err != nil {
			return false, errors.Join(errors.New("error getting block from beacon node"), err)
		}

		if found && sm.checkSlashing(&block) {
			return true, nil
		}

		// update the last processed slot
		sm.lastSlot = slot
	}

	return false, nil
}

func (sm *SlashingMonitor) checkSlashing(block *nmc_client.BeaconBlockResponse) bool {
	return sm.checkProposerSlashings(block.Data.Message.Body.ProposerSlashings) || sm.checkAttesterSlashings(block.Data.Message.Body.AttesterSlashings)
}

// reference: https://eth2book.info/capella/part3/transition/block/#attester-slashings
func (sm *SlashingMonitor) checkAttesterSlashings(slashings []nmc_client.AttesterSlashing) bool {
	for _, slashing := range slashings {
		attestingIndices1 := slashing.Attestation1.AttestingIndices
		attestingIndices2 := slashing.Attestation2.AttestingIndices
		slashedIndices := intersection(attestingIndices1, attestingIndices2)

		for _, index := range slashedIndices {
			if sm.isIndexMonitored(strconv.FormatUint(uint64(index), 10)) {
				sm.logger.Warn("Attester slashing detected for validator index", slog.Uint64("validatorIndex", uint64(index)))
				return true
			} else {
				sm.logger.Debug("Attester slashing of other validator detected", slog.Uint64("slashedValidatorIndex", uint64(index)))
			}
		}
	}

	return false
}

// reference: https://eth2book.info/capella/part3/transition/block/#proposer-slashings
func (sm *SlashingMonitor) checkProposerSlashings(slashings []nmc_client.ProposerSlashing) bool {
	for _, slashing := range slashings {
		slashedValidatorIndex := slashing.SignedHeader1.Message.ProposerIndex
		if sm.isIndexMonitored(slashedValidatorIndex) {
			sm.logger.Warn("Proposer slashing detected for validator index", slog.String("validatorIndex", slashedValidatorIndex))
			return true
		} else {
			sm.logger.Debug("Proposer slashing of other validator detected", slog.String("slashedValidatorIndex", slashedValidatorIndex))
		}
	}

	return false
}

func (sm *SlashingMonitor) isIndexMonitored(indexStr string) bool {
	index, err := strconv.ParseUint(indexStr, 10, 64)
	if err != nil {
		sm.logger.Error("Error parsing validator index", slog.String("index", indexStr))
		return false
	}

	return slices.Contains(sm.indexesToMonitor, index)
}

func intersection(a, b []nmc_utils.Uinteger) []nmc_utils.Uinteger {
	slices.Sort(a)
	slices.Sort(b)

	out := make([]nmc_utils.Uinteger, 0)

	var posA, posB int
	for posA < len(a) && posB < len(b) {
		if a[posA] == b[posB] {
			out = append(out, a[posA])
			posA++
			posB++
		} else if a[posA] < b[posB] {
			posA++
		} else {
			posB++
		}
	}
	return out
}
