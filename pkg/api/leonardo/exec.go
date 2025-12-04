package leonardo

type ExecRequest struct {
	Command string `json:"command"`
	Port    uint32 `json:"port"`
}
