// Package common implements utilities & functionality commonly consumed by the
// rest of the packages.
package common

import "errors"

var (
	// ErrNotImplemented is raised throughout the codebase of the challenge to
	// denote implementations to be done by the candidate.
	ErrNotImplemented = errors.New("not implemented")

	// ErrClientUnauthorized indicates the client connection has not logged in.
	ErrClientUnauthorized = errors.New("client unauthorized")
)

// Login represents the expected "login" tcp packet payload.
var Login = []byte("login")
