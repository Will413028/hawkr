package main

import (
	"testing"

	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/platform/config"
)

func setMediaEnv(t *testing.T, keks string) {
	t.Helper()
	t.Setenv("MEDIA_STORAGE", "s3compat")
	t.Setenv("MEDIA_S3_ENDPOINT", "http://localhost:3900")
	t.Setenv("MEDIA_S3_REGION", "garage")
	t.Setenv("MEDIA_S3_BUCKET", "media")
	t.Setenv("MEDIA_S3_ACCESS_KEY_ID", "id")
	t.Setenv("MEDIA_S3_SECRET_ACCESS_KEY", "secret")
	t.Setenv("MEDIA_KEKS", keks)
}

func TestReadMediaFailsStartupWithoutAStorageSelection(t *testing.T) {
	t.Setenv("MEDIA_STORAGE", "")
	t.Setenv("MEDIA_KEKS", "")
	e := config.NewEnv()
	readMedia(e)
	if e.Err() == nil {
		t.Fatal("沒有 MEDIA_STORAGE 與 MEDIA_KEKS 卻沒有回報設定錯誤")
	}
}

func TestNewMediaRefusesAMalformedKeyring(t *testing.T) {
	setMediaEnv(t, "not-a-keyring")
	e := config.NewEnv()
	s := readMedia(e)
	if err := e.Err(); err != nil {
		t.Fatal(err)
	}
	if _, err := newMedia(s, nil, nil, nil); err == nil {
		t.Fatal("KEK 格式錯誤時應啟動失敗")
	}
}

func TestReadMediaAcceptsACompleteConfiguration(t *testing.T) {
	setMediaEnv(t, "dev1:ZGV2LW9ubHktbWVkaWEta2VrLTAwMDAwMDAwMDAwMDA=")
	e := config.NewEnv()
	s := readMedia(e)
	if err := e.Err(); err != nil {
		t.Fatal(err)
	}
	if _, err := media.OpenStore(s.store); err != nil {
		t.Fatal(err)
	}
	if _, err := media.ParseKeyring(s.keks); err != nil {
		t.Fatal(err)
	}
}
