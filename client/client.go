package client

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

const (
	RequestUrlFormat   = "%s%s"
	RequestContentType = "application/json"

	RequestSyncStatusPath                  = "/eth/v1/node/syncing"
	RequestEth2ConfigPath                  = "/eth/v1/config/spec"
	RequestEth2DepositContractMethod       = "/eth/v1/config/deposit_contract"
	RequestCommitteePath                   = "/eth/v1/beacon/states/%s/committees"
	RequestGenesisPath                     = "/eth/v1/beacon/genesis"
	RequestFinalityCheckpointsPath         = "/eth/v1/beacon/states/%s/finality_checkpoints"
	RequestForkPath                        = "/eth/v1/beacon/states/%s/fork"
	RequestValidatorsPath                  = "/eth/v1/beacon/states/%s/validators"
	RequestVoluntaryExitPath               = "/eth/v1/beacon/pool/voluntary_exits"
	RequestAttestationsPath                = "/eth/v1/beacon/blocks/%s/attestations"
	RequestBeaconBlockPath                 = "/eth/v2/beacon/blocks/%s"
	RequestBeaconBlockHeaderPath           = "/eth/v1/beacon/headers/%s"
	RequestValidatorSyncDuties             = "/eth/v1/validator/duties/sync/%s"
	RequestValidatorProposerDuties         = "/eth/v1/validator/duties/proposer/%s"
	RequestWithdrawalCredentialsChangePath = "/eth/v1/beacon/pool/bls_to_execution_changes"
	EventStreamPath                        = "/eth/v1/events"

	MaxRequestValidatorsCount = 600
)

type BeaconHttpProvider struct {
	providerAddress string
	client          http.Client
}

func NewBeaconHttpProvider(providerAddress string, timeout time.Duration) *BeaconHttpProvider {
	return &BeaconHttpProvider{
		providerAddress: providerAddress,
		client: http.Client{
			Timeout: timeout,
		},
	}
}

func (p *BeaconHttpProvider) Node_Syncing(ctx context.Context) (SyncStatusResponse, error) {
	responseBody, status, err := p.getRequest(ctx, RequestSyncStatusPath)
	if err != nil {
		return SyncStatusResponse{}, fmt.Errorf("error getting node sync status: %w", err)
	}
	if status != http.StatusOK {
		return SyncStatusResponse{}, fmt.Errorf("error getting node sync status: HTTP status %d; response body: '%s'", status, string(responseBody))
	}
	var syncStatus SyncStatusResponse
	if err := json.Unmarshal(responseBody, &syncStatus); err != nil {
		return SyncStatusResponse{}, fmt.Errorf("error decoding node sync status: %w", err)
	}
	return syncStatus, nil
}

func (p *BeaconHttpProvider) Beacon_Block(ctx context.Context, blockId string) (BeaconBlockResponse, bool, error) {
	responseBody, status, err := p.getRequest(ctx, fmt.Sprintf(RequestBeaconBlockPath, blockId))
	if err != nil {
		return BeaconBlockResponse{}, false, fmt.Errorf("error getting beacon block data: %w", err)
	}
	if status == http.StatusNotFound {
		return BeaconBlockResponse{}, false, nil
	}
	if status != http.StatusOK {
		return BeaconBlockResponse{}, false, fmt.Errorf("error getting beacon block data: HTTP status %d; response body: '%s'", status, string(responseBody))
	}
	var beaconBlock BeaconBlockResponse
	if err := json.Unmarshal(responseBody, &beaconBlock); err != nil {
		return BeaconBlockResponse{}, false, fmt.Errorf("error decoding beacon block data: %w", err)
	}
	return beaconBlock, true, nil
}

func (p *BeaconHttpProvider) Beacon_Event_Stream(ctx context.Context, logger *slog.Logger, topics []string, ch chan Event) error {
	for _, topic := range topics {
		if topic != "head" {
			return fmt.Errorf("unsupported topic: %s", topic)
		}
	}

	query := "?topics=" + strings.Join(topics, ",")
	url := fmt.Sprintf(RequestUrlFormat, p.providerAddress, EventStreamPath) + query

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}

	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Connection", "keep-alive")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status: %s", resp.Status)
	}

	// start event stream
	go startEventStream(ctx, logger, req, ch)

	return nil
}

func startEventStream(ctx context.Context, logger *slog.Logger, req *http.Request, ch chan Event) {
	logger = logger.With("module", "event_stream")
	logger.Info("Starting event stream")

	var newEvent strings.Builder

	for {
		// Exit if the context has been cancelled.
		select {
		case <-ctx.Done():
			return
		// delay for 5 seconds - we don't want to spam the server
		case <-time.After(5 * time.Second):
		}

		client := &http.Client{}

		resp, err := client.Do(req)
		if err != nil {
			logger.Warn("Error reading event stream", slog.String("error", err.Error()))
			continue
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			logger.Warn("Unexpected received status code", slog.String("status", resp.Status))
			continue
		}

		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			// Exit if the context has been cancelled.
			select {
			case <-ctx.Done():
				resp.Body.Close()
				return
			default:
			}

			line := scanner.Text()

			if line == "" {
				if newEvent.Len() > 0 {
					e, err := parseEvent(newEvent.String())
					if err != nil {
						logger.Warn("Error parsing event", slog.String("error", err.Error()))
					}
					ch <- *e
					newEvent.Reset()
				}
				continue
			}

			newEvent.WriteString(line + "\n")
		}
		resp.Body.Close()

		if err := scanner.Err(); err != nil {
			logger.Warn("Error reading event stream", slog.String("error", err.Error()))
			continue
		}
	}
}

func parseEvent(eventStr string) (*Event, error) {
	parts := strings.Split(eventStr, "\n")
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid event format: Number of parts is %d", len(parts))
	}

	if !strings.HasPrefix(parts[0], "event: ") {
		return nil, fmt.Errorf("invalid event format: Missing event data")
	}

	if !strings.HasPrefix(parts[1], "data: ") {
		return nil, fmt.Errorf("invalid event format: Missing event data")
	}

	// remove prefix and sanitize data
	eventType := strings.TrimSpace(strings.TrimPrefix(parts[0], "event: "))
	data := strings.TrimSpace(strings.TrimPrefix(parts[1], "data: "))

	switch eventType {
	case "head":
		var headEventData HeadEvent
		if err := json.Unmarshal([]byte(data), &headEventData); err != nil {
			return nil, fmt.Errorf("error decoding head event data: %s", err.Error())
		}
		return &Event{eventType, &headEventData}, nil
	default:
		return nil, fmt.Errorf("unsupported event type: %s", eventType)
	}
}

func (p *BeaconHttpProvider) getRequest(ctx context.Context, requestPath string) ([]byte, int, error) {
	return getRequestImpl(ctx, requestPath, p.providerAddress, p.client)
}

func getRequestImpl(ctx context.Context, requestPath string, providerAddress string, client http.Client) ([]byte, int, error) {
	// Send request
	reader, status, err := getRequestReader(ctx, requestPath, providerAddress, client)
	if err != nil {
		return []byte{}, 0, err
	}
	defer func() {
		_ = reader.Close()
	}()

	// Get response
	body, err := io.ReadAll(reader)
	if err != nil {
		return []byte{}, 0, err
	}

	// Return
	return body, status, nil
}

func getRequestReader(ctx context.Context, requestPath string, providerAddress string, client http.Client) (io.ReadCloser, int, error) {
	// Make the request
	path := fmt.Sprintf(RequestUrlFormat, providerAddress, requestPath)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, 0, fmt.Errorf("error creating GET request to [%s]: %w", path, err)
	}
	req.Header.Set("Content-Type", RequestContentType)

	// Submit the request
	response, err := client.Do(req)
	if err != nil {
		// Remove the query for readability
		trimmedPath, _, _ := strings.Cut(path, "?")
		return nil, 0, fmt.Errorf("error running GET request to [%s]: %w", trimmedPath, err)
	}
	return response.Body, response.StatusCode, nil
}
