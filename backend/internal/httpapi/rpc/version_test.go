package rpc

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

func TestVersionEtagSemantics(t *testing.T) {
	if Etag(42) != "42" {
		t.Fatal("etag encoding")
	}
	if version, err := VersionFromEtag(""); err != nil || version != 0 {
		t.Fatalf("optional version=%d err=%v", version, err)
	}
	if version, err := RequiredVersion("42"); err != nil || version != 42 {
		t.Fatalf("required version=%d err=%v", version, err)
	}
	if _, err := RequiredVersion(""); toConnect(context.Background(), err).err.Code() != connect.CodeInvalidArgument {
		t.Fatalf("missing etag=%v", err)
	}
	if _, err := VersionFromEtag("bad"); apperr.KindOf(err) != apperr.KindVersionConflict {
		t.Fatalf("malformed etag=%v", err)
	}
	if _, err := RequiredVersionField("", "etag"); toConnect(context.Background(), err).err.Code() != connect.CodeInvalidArgument {
		t.Fatalf("field etag=%v", err)
	}
}

func TestFormatID(t *testing.T) {
	if got := FormatID(42); got != "42" {
		t.Fatalf("FormatID = %q", got)
	}
}

func TestRequiredFieldIsInvalidArgument(t *testing.T) {
	if err := RequiredField("store.etag", "etag 必填"); toConnect(context.Background(), err).err.Code() != connect.CodeInvalidArgument {
		t.Fatalf("err = %v", err)
	}
}
