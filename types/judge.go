package types

import "sync"

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
	JudgeObj struct {
		Judge
		// internal properties
		Submissions map[uint32]struct{} `json:"-"`
		Mutex       sync.Mutex          `json:"-"`
	}
	Runtime struct {
		ID        string `json:"id"`
		Compiler  string `json:"compiler"`
		Arguments string `json:"arguments"`
		Version   string `json:"version"`
	}
)
