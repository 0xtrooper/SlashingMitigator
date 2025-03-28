package slashingMonitor

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"strconv"
	"testing"
)

const (
	slashedValidatorIndex = 791764
	otherValidatorIndexA  = 16
	otherValidatorIndexB  = 32

	slashingBlock = 3822593
)

var (
	ctx        = context.Background()
	logger     = slog.Default()
	beaconNode = flag.String("beacon-node", "http://localhost:5052", "Address of the beacon node")
)

func TestMain(m *testing.M) {
	flag.Parse()
	os.Exit(m.Run())
}

func getTestSlashingMonitor(t *testing.T, indexes []uint64) *SlashingMonitor {
	sm, err := NewSlashingMonitor(ctx, logger, *beaconNode, "", indexes)
	if err != nil {
		t.Errorf("Error creating slashing mitigator: %v", err)
		return nil
	}

	return sm
}

func TestSlashingMonitor_CheckBeaconBlock(t *testing.T) {
	tests := []struct {
		name    string
		sm      *SlashingMonitor
		blockId uint64
		want    bool
		wantErr bool
	}{
		{
			name:    "Test block before the slashing",
			sm:      getTestSlashingMonitor(t, []uint64{slashedValidatorIndex}),
			blockId: slashingBlock - 1,
			want:    false,
			wantErr: false,
		},
		{
			name:    "Test slashing block",
			sm:      getTestSlashingMonitor(t, []uint64{slashedValidatorIndex}),
			blockId: slashingBlock,
			want:    true,
			wantErr: false,
		},
		{
			name:    "Test block after the slashing",
			sm:      getTestSlashingMonitor(t, []uint64{slashedValidatorIndex}),
			blockId: slashingBlock + 1,
			want:    false,
			wantErr: false,
		},
		{
			name:    "Test monitoring multiple validators",
			sm:      getTestSlashingMonitor(t, []uint64{slashedValidatorIndex, otherValidatorIndexA, otherValidatorIndexB}),
			blockId: slashingBlock,
			want:    true,
			wantErr: false,
		},
		{
			name:    "Test monitor other validator",
			sm:      getTestSlashingMonitor(t, []uint64{otherValidatorIndexA}),
			blockId: slashingBlock,
			want:    false,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := tt.sm.CheckBeaconBlock(ctx, "", strconv.Itoa(int(tt.blockId)))
			if (err != nil) != tt.wantErr {
				t.Errorf("SlashingMonitor.CheckBeaconBlock(%d) error = %v, wantErr %v", tt.blockId, err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("SlashingMonitor.CheckBeaconBlock(%d) = %v, want %v", tt.blockId, got, tt.want)
			}
		})
	}
}
