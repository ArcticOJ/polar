package pb

func (f Submission_Constraints_Flag) In(flags uint32) bool {
	return flags&uint32(f) != 0
}
