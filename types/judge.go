package types

type (
	Judge struct {
		Name        string    `json:"name"`
		Version     string    `json:"version"`
		Memory      uint32    `json:"memory"`
		OS          string    `json:"os"`
		Parallelism uint16    `json:"parallelism"`
		BootedSince int64     `json:"bootedSince"`
		Runtimes    []Runtime `json:"runtimes"`
	}
	Runtime struct {
		ID        string `json:"id"`
		Compiler  string `json:"compiler"`
		Arguments string `json:"arguments"`
		Version   string `json:"version"`
	}
)
