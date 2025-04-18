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

// RequiredFlagError represents the error returned when a value for a flag
// encountered an error during parsing. The underlying parsing error is embedded
// for unwrapping.
type ParseError struct {
	Flag  string
	Value string
	Err   error
}

// Error implements the error interface.
func (p *ParseError) Error() string {
	return fmt.Sprintf("failed parsing %q for flag %q: %s", p.Flag, p.Flag, p.Err)
}

// Unwrap allows the `ParseError` to be unwrapped using `errors.Is` and
// `errors.As` to get access to its embedded error.
func (p *ParseError) Unwrap() error {
	return p.Err
}

// NewParseError constructs a `ParseError` for the given flag, value, and
// embedded err.
func NewParseError(flag string, data string, err error) *ParseError {
	return &ParseError{Flag: flag, Value: data}
}
