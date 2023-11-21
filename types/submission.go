package types

type (
	Submission struct {
		ID            uint32
		SourcePath    string
		Runtime       string
		ProblemID     string
		TestCount     uint16
		PointsPerTest float64
		Constraints   Constraints
	}

	Constraints struct {
		IsInteractive bool
		TimeLimit     float32
		MemoryLimit   uint32
		OutputLimit   uint32
		AllowPartial  bool
		ShortCircuit  bool
	}
)
