package internal

// Must is a helper function to return the value or panic if an error is not nil.
func Must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}
