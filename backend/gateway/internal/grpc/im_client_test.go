package grpc

import (
	"strings"
	"testing"

	"github.com/Will413028/hawkr/gateway/internal/config"
)

const imClientTestKey = "01234567890123456789012345678901"

func TestNewIMClient_GRPCRequiresDedicatedStreamCredential(t *testing.T) {
	client, err := NewIMClient(&config.Config{})
	if err == nil || client != nil {
		t.Fatalf("NewIMClient without stream key = client=%v err=%v, want fail closed", client, err)
	}
	if !strings.Contains(err.Error(), "IM chat stream authentication") {
		t.Fatalf("error = %v, want dedicated stream auth context", err)
	}
}

func TestNewIMChatStreamSigner_UsesDedicatedKeyAndRotationKey(t *testing.T) {
	signer, err := newIMChatStreamSigner(imClientTestKey, "abcdefghijklmnopqrstuvwxyz123456")
	if err != nil {
		t.Fatalf("newIMChatStreamSigner: %v", err)
	}
	if _, err := signer.Sign(); err != nil {
		t.Fatalf("signer.Sign: %v", err)
	}
}

func TestNewIMClient_RejectsNilConfig(t *testing.T) {
	client, err := NewIMClient(nil)
	if err == nil || client != nil {
		t.Fatalf("NewIMClient(nil) = client=%v err=%v, want config error", client, err)
	}
}
