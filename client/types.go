package client

type SyncStatusResponse struct {
	Data struct {
		IsSyncing    bool     `json:"is_syncing"`
		HeadSlot     Uinteger `json:"head_slot"`
		SyncDistance Uinteger `json:"sync_distance"`
	} `json:"data"`
}

type Attestation struct {
	AggregationBits string `json:"aggregation_bits"`
	Data            struct {
		Slot  Uinteger `json:"slot"`
		Index Uinteger `json:"index"`
	} `json:"data"`
}

type AttestationsResponse struct {
	Data []Attestation `json:"data"`
}

type SignedBeaconBlockHeader struct {
	Message struct {
		Slot          Uinteger  `json:"slot"`
		ProposerIndex string    `json:"proposer_index"`
		ParentRoot    ByteArray `json:"parent_root"`
		StateRoot     ByteArray `json:"state_root"`
		BodyRoot      ByteArray `json:"body_root"`
	} `json:"message"`
	Signature ByteArray `json:"signature"`
}

type ProposerSlashing struct {
	SignedHeader1 SignedBeaconBlockHeader `json:"signed_header_1"`
	SignedHeader2 SignedBeaconBlockHeader `json:"signed_header_2"`
}

type SignedAttestationData struct {
	AttestingIndices []Uinteger `json:"attesting_indices"`
	Data             struct {
		Slot            Uinteger  `json:"slot"`
		Index           Uinteger  `json:"index"`
		BeaconBlockRoot ByteArray `json:"beacon_block_root"`
		Source          struct {
			Epoch Uinteger  `json:"epoch"`
			Root  ByteArray `json:"root"`
		} `json:"source"`
		Target struct {
			Epoch Uinteger  `json:"epoch"`
			Root  ByteArray `json:"root"`
		} `json:"target"`
	} `json:"data"`
	Signature ByteArray `json:"signature"`
}

type AttesterSlashing struct {
	Attestation1 SignedAttestationData `json:"attestation_1"`
	Attestation2 SignedAttestationData `json:"attestation_2"`
}

type BeaconBlockResponse struct {
	Data struct {
		Message struct {
			Slot          Uinteger `json:"slot"`
			ProposerIndex string   `json:"proposer_index"`
			Body          struct {
				Eth1Data struct {
					DepositRoot  ByteArray `json:"deposit_root"`
					DepositCount Uinteger  `json:"deposit_count"`
					BlockHash    ByteArray `json:"block_hash"`
				} `json:"eth1_data"`
				Attestations      []Attestation      `json:"attestations"`
				ProposerSlashings []ProposerSlashing `json:"proposer_slashings"`
				AttesterSlashings []AttesterSlashing `json:"attester_slashings"`
				ExecutionPayload  *struct {
					FeeRecipient ByteArray `json:"fee_recipient"`
					BlockNumber  Uinteger  `json:"block_number"`
				} `json:"execution_payload"`
			} `json:"body"`
		} `json:"message"`
	} `json:"data"`
}

func (b *BeaconBlockResponse) ProposerSlashings() []ProposerSlashing {
	return b.Data.Message.Body.ProposerSlashings
}

func (b *BeaconBlockResponse) AttesterSlashings() []AttesterSlashing {
	return b.Data.Message.Body.AttesterSlashings
}
