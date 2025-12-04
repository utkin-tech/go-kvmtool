package state

import (
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/utkin-tech/go-kvmtool/pkg/event"
)

var Observable = event.NewObservableState(specs.StateCreating)
