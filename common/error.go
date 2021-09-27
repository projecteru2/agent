package common

import "errors"

// ErrNotImplemented .
var ErrNotImplemented = errors.New("not implemented")

// ErrConnecting means writer is in connecting status, waiting to be connected
var ErrConnecting = errors.New("connecting")

// ErrInvalidScheme .
var ErrInvalidScheme = errors.New("invalid scheme")
