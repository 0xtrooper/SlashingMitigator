## Usage

The `SlashingMitigator` is a tool designed to monitor and mitigate slashing events in an Ethereum 2.0 beacon chain. It connects to a beacon node and listens for new block events to detect any slashing activities. It can be used by any validator, as the tool allows personal shutdown commands that are executed in case a slashing event is detected for one of the validator's nodes. Stopping the validator client in such a scenario **could** help mitigate further damage by preventing **additional** slashable actions. This, in turn, **could** limit financial losses. Note that this only applies when the misbehavior was isolated to one validator.

**Note:** This project is beeing worked on as part of the Rocketpool bounty [Slashing Mitigator](https://rpbountyboard.com/BA092403).

### Flags

- `--beacon-node`: Address of the beacon node (default: `http://localhost:5052`).
- `--debug`: Enable debug logging (default: `false`).
- `--wait-for-sync`: Wait for the beacon node to sync before starting (default: `false`).
- `--shutdown-cmd`: Command to run on shutdown (default: `rocketpool service stop -y`).
- `--shutdown-test`: Run the shutdown command and exit (default: `false`).
- `--validator-index`: Validator index to monitor. If testing shutdown, this can be omitted; otherwise, it must be provided.

### Example

To run the `SlashingMitigator`, use the following command:

```sh
go run main.go --validator-index "1,2,3"
```

If you want to test the shutdown command, you can run:

```sh
go run main.go --shutdown-test
```

### Custom options

- **`--beacon-node`**  
  **Usage:** `--beacon-node http://your-beacon-node:5052`  
  **Explanation:** This flag sets the address of the Ethereum 2.0 beacon node that the SlashingMitigator will connect to. It defaults to `http://localhost:5052`, so if your node is running on a different host or port, specify the correct URL.

- **`--wait-for-sync`**  
  **Usage:** `--wait-for-sync`  
  **Explanation:** This flag instructs the tool to wait until the beacon node is fully synchronized before starting to monitor for slashing events. Otherwise, the tool will report a failed startup. **Use with caution, as the tool does not function until the node has finished syncing.**

- **`--shutdown-cmd`**  
  **Usage:** `--shutdown-cmd "custom shutdown command"`  
  **Explanation:** This flag specifies the command that should be executed when a shutdown is triggered. The default command is `"rocketpool service stop -y"`. You can customize it if your shutdown procedure differs.

- **`--shutdown-test`**  
  **Usage:** `--shutdown-test`  
  **Explanation:** When this flag is provided, the application will execute the shutdown command specified by `--shutdown-cmd` and then exit. This is useful for testing the shutdown process without running the full monitoring loop. Note that when this flag is used, the validator index isn't required.

- **`--validator-index`**  
  **Usage:** `--validator-index "1,2,3"`
  **Explanation:** This flag defines one or more validator indices to monitor for slashing events. This flag is required. If multiple indices are provided, they should be comma-separated without spaces.

- **`--debug`**  
  **Usage:** `--debug`  
  **Explanation:** When this flag is set, the application enables debug logging. This is useful for troubleshooting and understanding the internal behavior of the SlashingMitigator. By default, debug logging is disabled.


### Testing

The tests for `SlashingMitigator` are located in the `slashingMitigator_test.go` file. The tests cover various scenarios to ensure the correct detection of slashing events.

To run the tests, use the following command:

```sh
go test -v ./slashingMitigator
```

You can also specify the beacon node address for the tests using the `--beacon-node` flag:

```sh
go test -v ./slashingMitigator --beacon-node http://your-beacon-node:5052
```