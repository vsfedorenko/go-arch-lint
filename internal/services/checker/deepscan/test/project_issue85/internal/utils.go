package internal

// Must mirrors the two-parameter (value, error) helper from issue #85. The
// error parameter is an interface, so deep-scan tracks it as a gate.
func Must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}

	return v
}
