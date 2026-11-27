
CREATE SCHEMA IF NOT EXISTS public;

CREATE SCHEMA accountdeletion;

CREATE TABLE accountdeletion.deletion_requests (
    id UUID PRIMARY KEY,
    account_id BIGINT NOT NULL,
    status TEXT NOT NULL,
    requested_at TIMESTAMPTZ NOT NULL,
    completed_at TIMESTAMPTZ,
    CONSTRAINT deletion_requests_account_check CHECK (account_id > 0),
    CONSTRAINT deletion_requests_status_check CHECK (status IN ('PENDING_ORDERS', 'COMPLETED')),
    CONSTRAINT deletion_requests_completed_check CHECK ((status = 'COMPLETED') = (completed_at IS NOT NULL))
);
CREATE UNIQUE INDEX deletion_requests_one_pending ON accountdeletion.deletion_requests (account_id) WHERE status = 'PENDING_ORDERS';

CREATE TABLE accountdeletion.request_attempts (
    id UUID PRIMARY KEY,
    request_id UUID NOT NULL REFERENCES accountdeletion.deletion_requests (id),
    requester TEXT NOT NULL,
    operator_admin_id BIGINT,
    reason TEXT NOT NULL,
    attempted_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT request_attempts_requester_check CHECK (requester IN ('ACCOUNT_OWNER', 'ADMIN')),
    CONSTRAINT request_attempts_operator_check CHECK ((requester = 'ADMIN') = (operator_admin_id IS NOT NULL) AND (operator_admin_id IS NULL OR operator_admin_id > 0)),
    CONSTRAINT request_attempts_reason_check CHECK (char_length(reason) <= 1000 AND (requester <> 'ADMIN' OR (char_length(reason) >= 1 AND reason !~ '^[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]|[\u0009\u000a\u000b\u000c\u000d\u0020\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000]$')))
);
CREATE INDEX request_attempts_request_time ON accountdeletion.request_attempts (request_id, attempted_at, id);
