package usecase

func seqIDs(start int64) func() int64 {
	next := start
	return func() int64 {
		id := next
		next++
		return id
	}
}
