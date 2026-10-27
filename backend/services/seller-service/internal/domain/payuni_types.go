package domain

//go:generate enumer -type=PayuniStatus -trimprefix=PayuniStatus -json -text -transform=snake --output=zzz_enumer_payuniStatus.go
type PayuniStatus int

const (
	PayuniStatusUnregistered PayuniStatus = iota
	PayuniStatusPending
	PayuniStatusActive
	PayuniStatusFailed
)
