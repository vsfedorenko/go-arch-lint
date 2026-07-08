package dsl

var current contextStack

type contextStack struct {
	spec    *SpecBuilder
	dep     *DepEntry
	inAllow bool
}
