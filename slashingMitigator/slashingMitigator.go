package slashingMitigator

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slashingMitigator/client"
	"slices"
	"strconv"
	"time"
)

type SlashingMitigator struct {
	logger *slog.Logger

	client           *client.BeaconHttpProvider
	indexesToMonitor []uint64

	eventLoopCancelFucn context.CancelFunc
}

func NewSlashingMitigator(ctx context.Context, logger *slog.Logger, beaconNode string, indexesToMonitor []uint64) (*SlashingMitigator, error) {
	client := client.NewBeaconHttpProvider(beaconNode, time.Second*12)

	syncStatus, err := client.Node_Syncing(ctx)
	if err != nil {
		return nil, errors.Join(errors.New("error connecting to beacon node"), err)
	}

	if syncStatus.Data.IsSyncing {
		return nil, fmt.Errorf("beacon node is syncing, sync distance: %d", syncStatus.Data.SyncDistance)
	}

	return &SlashingMitigator{
		logger:           logger.With("module", "slashingMitigator"),
		client:           client,
		indexesToMonitor: indexesToMonitor,
	}, nil
}

func (sm *SlashingMitigator) Start(ctx context.Context) error {
	// create a child context to cancel the event stream
	var eventLoopCtx context.Context
	eventLoopCtx, sm.eventLoopCancelFucn = context.WithCancel(ctx)

	// create event stream for new heads
	newHeadStream := make(chan client.Event)
	err := sm.client.Beacon_Event_Stream(eventLoopCtx, sm.logger, []string{"head"}, newHeadStream)
	if err != nil {
		return errors.Join(errors.New("error creating event stream for new heads"), err)
	}

	// start monitoring new heads
	go sm.monitorNewHeads(eventLoopCtx, newHeadStream)
	return nil
}

func (sm *SlashingMitigator) Stop(ctx context.Context) {
	if sm.eventLoopCancelFucn != nil {
		sm.eventLoopCancelFucn()
	}
}

func (sm *SlashingMitigator) monitorNewHeads(ctx context.Context, newHeadStream chan client.Event) {
	for {
		select {
		case <-ctx.Done():
			return
		case event := <-newHeadStream:
			if event.Data == nil {
				sm.logger.Warn("Received event with no data")
				continue
			}

			eventData, ok := event.Data.(*client.HeadEvent)
			if !ok {
				sm.logger.Warn("Received event with unexpected data type", slog.Any("data", event.Data))
				continue
			}

			sm.logger.Debug("Received new head event", slog.String("slot", eventData.Slot))

			foundSlashing, err := sm.CheckBeaconBlock(ctx, eventData.Block)
			if err != nil {
				sm.logger.Error("Error checking beacon block", slog.String("blockId", eventData.Block), slog.String("error", err.Error()))
				continue
			}

			if foundSlashing {
				sm.logger.Warn("Slashing detected in block", slog.String("blockId", eventData.Block))
			}
		}
	}
}

func (sm *SlashingMitigator) CheckBeaconBlock(ctx context.Context, blockId string) (bool, error) {
	// get block from beacon node
	block, found, err := sm.client.Beacon_Block(ctx, blockId)
	if err != nil {
		return false, errors.Join(errors.New("error getting block from beacon node"), err)
	}

	if !found {
		return false, nil
	}

	return sm.checkProposerSlashings(block.ProposerSlashings()) || sm.checkAttesterSlashings(block.AttesterSlashings()), nil
}

// reference: https://eth2book.info/capella/part3/transition/block/#attester-slashings
func (sm *SlashingMitigator) checkAttesterSlashings(slashings []client.AttesterSlashing) bool {
	for _, slashing := range slashings {
		attestingIndices1 := slashing.Attestation1.AttestingIndices
		attestingIndices2 := slashing.Attestation2.AttestingIndices
		slashedIndices := intersection(attestingIndices1, attestingIndices2)

		for _, index := range slashedIndices {
			if sm.isIndexToMonitor(strconv.FormatUint(uint64(index), 10)) {
				sm.logger.Warn("Attester slashing detected for validator index", slog.Uint64("validatorIndex", index.Uint64()))
				return true
			} else {
				sm.logger.Debug("Attester slashing of other validator detected", slog.Uint64("slashedValidatorIndex", index.Uint64()))
			}
		}
	}

	return false
}

// reference: https://eth2book.info/capella/part3/transition/block/#proposer-slashings
func (sm *SlashingMitigator) checkProposerSlashings(slashings []client.ProposerSlashing) bool {
	for _, slashing := range slashings {
		slashedValidatorIndex := slashing.SignedHeader1.Message.ProposerIndex
		if sm.isIndexToMonitor(slashedValidatorIndex) {
			sm.logger.Warn("Proposer slashing detected for validator index", slog.String("validatorIndex", slashedValidatorIndex))
			return true
		} else {
			sm.logger.Debug("Proposer slashing of other validator detected", slog.String("slashedValidatorIndex", slashedValidatorIndex))
		}
	}

	return false
}

func (sm *SlashingMitigator) isIndexToMonitor(indexStr string) bool {
	index, err := strconv.ParseUint(indexStr, 10, 64)
	if err != nil {
		sm.logger.Error("Error parsing validator index", slog.String("index", indexStr))
		return false
	}

	return slices.Contains(sm.indexesToMonitor, index)
}

func intersection(a, b []client.Uinteger) []client.Uinteger {
	slices.Sort(a)
	slices.Sort(b)

	out := make([]client.Uinteger, 0)

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
