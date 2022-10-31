package common

import "github.com/pkg/errors"

// ErrNotImplemented .
var (
	ErrNotImplemented = errors.New("not implemented")
	// ErrConnecting means writer is in connecting status, waiting to be connected
	ErrConnecting = errors.New("connecting")
	// ErrInvalidScheme .
	ErrInvalidScheme = errors.New("invalid scheme")
	// ErrGetRuntimeFailed .
	ErrGetRuntimeFailed = errors.New("failed to get runtime client")
	// ErrInvalidRuntimeType .
	ErrInvalidRuntimeType = errors.New("unknown runtime type")
	// ErrGetStoreFailed .
	ErrGetStoreFailed = errors.New("failed to get store client")
	// ErrInvalidStoreType .
	ErrInvalidStoreType = errors.New("unknown store type")
	// ErrWorkloadUnhealthy .
	ErrWorkloadUnhealthy = errors.New("not healthy")
	// ErrClosedSteam .
	ErrClosedSteam = errors.New("closed")
	// ErrSyscallFailed .
	ErrSyscallFailed = errors.New("syscall fail, Not a syscall.Stat_t")
	// ErrDevNotFound .
	ErrDevNotFound = errors.New("device not found")
	// ErrJournalDisable .
	ErrJournalDisable = errors.New("journal disabled")
	// ErrInvaildContainer .
	ErrInvaildContainer = errors.New("invaild container")
	// ErrGetLockFailed .
	ErrGetLockFailed = errors.New("get lock failed")
	// ErrInvaildVM .
	ErrInvaildVM = errors.New("invaild vm")
)
