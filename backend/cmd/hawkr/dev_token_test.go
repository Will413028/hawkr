package main

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/Will413028/hawkr/backend/internal/platform/apperr"
)

type fakeDevSessions struct {
	live    map[int64]bool
	created int
	next    int64
}

func (f *fakeDevSessions) DevSession(_ context.Context, id int64) (string, error) {
	if !f.live[id] {
		return "", apperr.NotFound("帳號 %d 不存在", id)
	}
	return "tok-for-account", nil
}

func (f *fakeDevSessions) DevNewAccount(context.Context) (int64, error) {
	f.created++
	f.next++
	id := 500 + f.next
	f.live[id] = true
	return id, nil
}

func TestIssueCustomerTokenForExistingAccount(t *testing.T) {
	f := &fakeDevSessions{live: map[int64]bool{2001: true}}
	var out, errOut bytes.Buffer
	if err := issueCustomerToken(t.Context(), f, 2001, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	if out.String() != "tok-for-account\n" || errOut.Len() != 0 || f.created != 0 {
		t.Errorf("out = %q, err = %q, 建立 %d 個帳號", out.String(), errOut.String(), f.created)
	}
}

// 帳號不存在是錯誤，不能默默建立一個（建了就變成拿不到預期帳號的 token）。
func TestIssueCustomerTokenRefusesAMissingAccount(t *testing.T) {
	f := &fakeDevSessions{live: map[int64]bool{}}
	var out, errOut bytes.Buffer
	err := issueCustomerToken(t.Context(), f, 999999, &out, &errOut)
	if err == nil || !strings.Contains(err.Error(), "999999") {
		t.Fatalf("err = %v，預期帶帳號 ID 的錯誤", err)
	}
	if f.created != 0 || out.Len() != 0 {
		t.Errorf("建立 %d 個帳號、stdout = %q，預期都沒有", f.created, out.String())
	}
}

func TestIssueCustomerTokenCreatesAnAccountOnlyWithoutAccountFlag(t *testing.T) {
	f := &fakeDevSessions{live: map[int64]bool{}}
	var out, errOut bytes.Buffer
	if err := issueCustomerToken(t.Context(), f, 0, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	if f.created != 1 || out.String() != "tok-for-account\n" || !strings.Contains(errOut.String(), "501") {
		t.Errorf("建立 %d、out = %q、err = %q", f.created, out.String(), errOut.String())
	}
}

func TestParseDevTokenArgs(t *testing.T) {
	for _, tc := range []struct {
		args    []string
		want    devTokenOptions
		wantErr string
	}{
		{nil, devTokenOptions{user: "dev_buyer"}, ""},
		{[]string{"-account", "2001"}, devTokenOptions{user: "dev_buyer", account: 2001}, ""},
		{[]string{"-admin", "-user", "dev_ops"}, devTokenOptions{admin: true, user: "dev_ops"}, ""},
		{[]string{"-user", "dev_account_2001"}, devTokenOptions{}, "-account"},
		{[]string{"-admin", "-account", "1"}, devTokenOptions{}, "不能與 -admin"},
		{[]string{"-account", "0"}, devTokenOptions{}, "正的帳號 ID"},
		{[]string{"-account", "-3"}, devTokenOptions{}, "正的帳號 ID"},
	} {
		got, err := parseDevTokenArgs(tc.args)
		if tc.wantErr != "" {
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("%v：err = %v，預期含 %q", tc.args, err, tc.wantErr)
			}
			continue
		}
		if err != nil || got != tc.want {
			t.Errorf("%v：got %+v, %v，預期 %+v", tc.args, got, err, tc.want)
		}
	}
}
