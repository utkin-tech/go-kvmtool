package raphael_api

import "github.com/opencontainers/runtime-spec/specs-go"

type StatusRequest struct{}

type StatusResponse struct {
	Status specs.ContainerState `json:"status"`
}
