package main

import "fmt"

// RequiredFlagError represents the error returned when a value for required
// flag has not been provided.
type RequiredFlagError struct {
	Flag string
}

// Error implements the error interface.
func (r *RequiredFlagError) Error() string {
	return fmt.Sprintf("required option %q not set", r.Flag)
}

// NewRequiredFlagError constructs a `RequiredFlagError` for the given option.
func NewRequiredFlagError(option string) *RequiredFlagError {
	return &RequiredFlagError{Flag: option}
}

// FlagParseError represents the error encountered when parsing a value for a
// flag. The underlying error is embedded for unwrapping.
type FlagParseError struct {
	Flag string
	Err  error
}

// Error implements the error interface.
func (p *FlagParseError) Error() string {
	return fmt.Sprintf("failed parsing flag %q: %s", p.Flag, p.Err.Error())
}

// Unwrap allows the `ParseError` to be unwrapped using `errors.Is` and
// `errors.As` to get access to its embedded error.
func (p *FlagParseError) Unwrap() error {
	return p.Err
}

// NewFlagParseError constructs a `FlagParseError` for the given flag and
// embedded err.
func NewFlagParseError(flag string, err error) *FlagParseError {
	return &FlagParseError{Flag: flag, Err: err}
}
