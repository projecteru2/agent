package common

import "errors"

var (
	ErrNotImplemented     = errors.New("not implemented")
	ErrConnecting         = errors.New("connecting")
	ErrInvalidScheme      = errors.New("invalid scheme")
	ErrInvalidRuntimeType = errors.New("unknown runtime type")
	ErrInvalidStoreType   = errors.New("unknown store type")
	ErrWorkloadUnhealthy  = errors.New("not healthy")
	ErrSyscallFailed      = errors.New("syscall fail, Not a syscall.Stat_t")
	ErrDevNotFound        = errors.New("device not found")
	ErrJournalDisable     = errors.New("journal disabled")
	ErrInvaildContainer   = errors.New("invaild container")
	ErrGetLockFailed      = errors.New("get lock failed")
	ErrInvaildVM          = errors.New("invaild vm")
)
