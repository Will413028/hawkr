package model

import "time"

func IdleSince(now time.Time) time.Time { return now.Add(-DelegationIdleTimeout) }

func IdleExpired(lastActiveAt, now time.Time) bool {
	return !now.Before(lastActiveAt.Add(DelegationIdleTimeout))
}

func SessionStatus(status string, expiresAt, lastActiveAt, now time.Time) DelegationStatus {
	switch {
	case status == string(DelegationRevoked):
		return DelegationRevoked
	case !now.Before(expiresAt), IdleExpired(lastActiveAt, now):
		return DelegationExpired
	default:
		return DelegationActive
	}
}

func IsAuthorizationEffective(revokedAt *time.Time, effectiveFrom time.Time, effectiveUntil *time.Time, now time.Time) bool {
	return revokedAt == nil && !now.Before(effectiveFrom) && (effectiveUntil == nil || now.Before(*effectiveUntil))
}

func Optional(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
