package models

import (
	"errors"

	"github.com/vsfedorenko/go-arch-lint/v2/internal/models/domain"
)

type (
	UserSpaceError struct {
		msg string
	}

	// ConfigError marks "the check could not run because the arch spec (or
	// the project layout it references) is invalid". Distinct from
	// UserSpaceError so entry points can map it onto a dedicated exit code
	// (2) — a broken config lints nothing and must not be mistaken for
	// "violations found" (1).
	ConfigError struct {
		msg string
	}

	ReferableError struct {
		original  error
		reference domain.Reference
	}
)

func (u UserSpaceError) Error() string {
	return u.msg
}

func (c ConfigError) Error() string {
	return c.msg
}

func (r ReferableError) Error() string {
	return r.original.Error()
}

func (r ReferableError) Reference() domain.Reference {
	return r.reference
}

func (u UserSpaceError) Is(err error) bool {
	if err == nil {
		return false
	}

	if _, ok := err.(UserSpaceError); ok {
		return true
	}

	return false
}

func (c ConfigError) Is(err error) bool {
	if err == nil {
		return false
	}

	if _, ok := err.(ConfigError); ok {
		return true
	}

	return false
}

// IsConfigError reports whether err (or anything it wraps) is a ConfigError.
func IsConfigError(err error) bool {
	if err == nil {
		return false
	}

	var target ConfigError
	if errors.As(err, &target) {
		return true
	}

	if target := (ConfigError{}); errors.Is(err, target) {
		return true
	}

	return false
}

func (r ReferableError) Is(err error) bool {
	if err == nil {
		return false
	}

	if _, ok := err.(ReferableError); ok {
		return true
	}

	return false
}

func NewUserSpaceError(msg string) UserSpaceError {
	return UserSpaceError{
		msg: msg,
	}
}

func NewConfigError(msg string) ConfigError {
	return ConfigError{
		msg: msg,
	}
}

// IsUserSpaceError reports whether err (or anything it wraps) is a
// UserSpaceError — i.e. the check ran successfully and the error describes
// user-visible findings (violations, notices), not a system failure.
// Public so entry points can map it onto linter-conventional exit codes
// without importing internal packages.
func IsUserSpaceError(err error) bool {
	if err == nil {
		return false
	}

	var target UserSpaceError
	if errors.As(err, &target) {
		return true
	}

	// UserSpaceError values are compared by value (its Is method), and some
	// call sites return it unwrapped from an error chain built with %w.
	if target := (UserSpaceError{}); errors.Is(err, target) {
		return true
	}

	return false
}

func NewReferableErr(err error, ref domain.Reference) ReferableError {
	return ReferableError{
		original:  err,
		reference: ref,
	}
}
