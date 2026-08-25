package common

import "errors"

var (
	ErrNotImplemented     = errors.New("not implemented")
	ErrConnecting         = errors.New("connecting")
	ErrInvalidScheme      = errors.New("invalid scheme")
	ErrInvalidRuntimeType = errors.New("unknown runtime type")
	ErrInvalidStoreType   = errors.New("unknown store type")
	ErrWorkloadUnhealthy  = errors.New("not healthy")
	ErrSyscallFailed      = errors.New("not a syscall.Stat_t")
	ErrDevNotFound        = errors.New("device not found")
	ErrJournalDisabled    = errors.New("journal disabled")
	ErrInvalidContainer   = errors.New("invalid container")
	ErrGetLockFailed      = errors.New("get lock failed")
	ErrInvalidVM          = errors.New("invalid vm")
)
