**Note:** The /beaconClient code is highly inspired by https://github.com/rocket-pool/node-manager-core/blob/main/beacon/client but adapted to only include the necessary functions and to support additional fields in the block response.

## Usage

The `SlashingMitigator` is a tool designed to monitor and mitigate slashing events in an Ethereum 2.0 beacon chain. It connects to a beacon node and listens for new block events to detect any slashing activities.

**Note:** This project is a work in progress (WIP) and is intended to fulfill a Rocketpool bounty [Slashing Mitigator](https://rpbountyboard.com/BA092403). The next target for this project is to integrate the `SlashingMitigator` into the Rocketpool Smartnode V2.

### Functions

- `NewSlashingMitigator(..)`: Creates a new instance of `SlashingMitigator`.
- `Start(..) error`: Starts the slashing mitigator to begin monitoring for slashing events.
- `Stop()`: Stops the slashing mitigator.
- `CheckBeaconBlock(..)`: Checks a specific block for slashing events.
- `checkAttesterSlashings(..)`: Checks for attester slashings in a block.
- `checkProposerSlashings(..)`: Checks for proposer slashings in a block.

### Flags

- `--beacon-node`: Address of the beacon node (default: `http://localhost:5052`).
- `--debug`: Enable debug logging (default: `false`).

### Example

To run the `SlashingMitigator`, use the following command:

```sh
go run main.go --beacon-node http://your-beacon-node:5052 --debug
```

### Tests

The tests for `SlashingMitigator` are located in the `slashingMitigator_test.go` file. The tests cover various scenarios to ensure the correct detection of slashing events.

To run the tests, use the following command:

```sh
go test -v ./slashingMitigator
```

You can also specify the beacon node address for the tests using the `--beacon-node` flag:

```sh
go test -v ./slashingMitigator --beacon-node http://your-beacon-node:5052
```