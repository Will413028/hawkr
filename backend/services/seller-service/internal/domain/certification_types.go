package domain

//go:generate enumer -type=CertificationStatus -trimprefix=CertStatus -json -text -transform=snake --output=zzz_enumer_certificationStatus.go
type CertificationStatus int

const (
	CertStatusDraft CertificationStatus = iota
	CertStatusSubmitted
	CertStatusForwarded
	CertStatusRejected
	CertStatusCancelled
	CertStatusForwardFailed
)

//go:generate enumer -type=ReviewAction -trimprefix=ReviewAction -json -text -transform=snake --output=zzz_enumer_reviewAction.go
type ReviewAction int

const (
	ReviewActionSubmit ReviewAction = iota
	ReviewActionRequestInfo
	ReviewActionApprove
	ReviewActionReject
	ReviewActionCancel
)

//go:generate enumer -type=ReviewResult -trimprefix=ReviewResult -json -text -transform=snake --output=zzz_enumer_reviewResult.go
type ReviewResult int

const (
	ReviewResultPending ReviewResult = iota
	ReviewResultApproved
	ReviewResultRejected
	ReviewResultForwardFailed
)
