package main

import "archlint_issue85/internal"

func retSomething() (int, error) {
	return 42, nil
}

// main passes a multi-value return as a single argument tuple (foo(bar())).
// deep-scan used to panic with index-out-of-range when walking such a call.
func main() {
	_ = internal.Must(retSomething())
}
