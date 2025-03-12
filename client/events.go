package client

type Event struct {
	Topic string
	Data  any
}

type HeadEvent struct {
	Slot                      string `json:"slot"`
	Block                     string `json:"block"`
	State                     string `json:"state"`
	EpochTransition           bool   `json:"epoch_transition"`
	CurrentDutyDependentRoot  string `json:"current_duty_dependent_root,omitempty"`
	PreviousDutyDependentRoot string `json:"previous_duty_dependent_root,omitempty"`
}
