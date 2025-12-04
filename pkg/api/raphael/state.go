package raphael_api

import "github.com/opencontainers/runtime-spec/specs-go"

type StateRequest struct{}

type StateResponse = specs.State
