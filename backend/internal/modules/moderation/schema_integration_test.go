//go:build integration

package moderation

import (
	"context"
	"regexp"
	"slices"
	"testing"
	"time"
	"uuid"

	"github.com/Will413028/hawkr/backend/internal/modules/moderation/internal/model"
	"github.com/Will413028/hawkr/backend/internal/platform/policy"
)

func stringsOf[T ~string](vs []T) []string {
	out := make([]string, len(vs))
	for i, v := range vs {
		out[i] = string(v)
	}
	return out
}

// DB 的 CHECK 列出的值與 Go 的值集合一致（儲存的代碼是 UPPER_SNAKE）。
func TestChecksMatchGo(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	f := setup(t)
	upperSnake := regexp.MustCompile(`^[A-Z][A-Z0-9]*(?:_[A-Z0-9]+)*$`)
	for _, tt := range []struct {
		table, name string
		values      []string
	}{
		{"cases", "cases_target_kind_check", stringsOf(model.TargetKinds())},
		{"cases", "cases_decision_check", []string{string(DecisionViolation), string(DecisionNoViolation)}},
		{"cases", "cases_decision_category_check", stringsOf(policy.Assessable())},
		{"cases", "cases_decision_action_check", stringsOf(model.Actions())},
		{"reports", "reports_category_check", stringsOf(policy.CommunityReporterSelectable())},
	} {
		for _, v := range tt.values {
			if !upperSnake.MatchString(v) {
				t.Fatalf("%s：%q 不是 UPPER_SNAKE", tt.name, v)
			}
		}
		var def string
		must(t, f.db.Conn(ctx).QueryRow(ctx, "SELECT pg_get_constraintdef(oid) FROM pg_constraint WHERE conrelid = $1::regclass AND conname = $2", "moderation."+tt.table, tt.name).Scan(&def))
		var actual []string
		for _, m := range regexp.MustCompile(`'([^']+)'`).FindAllStringSubmatch(def, -1) {
			actual = append(actual, m[1])
		}
		want := slices.Clone(tt.values)
		slices.Sort(actual)
		slices.Sort(want)
		if !slices.Equal(actual, want) {
			t.Errorf("%s：DB %v，Go %v", tt.name, actual, want)
		}
	}
}

// 案件的 CHECK 擋掉不合法的狀態：決定、管理員、結案時間同為 NULL 或同不為 NULL；違規帶類別與處置、不違規不帶；處置種類與對象種類要搭配；
// 合法的組合照常寫入。
func TestCaseChecksRejectIllegalStates(t *testing.T) {
	t.Parallel()
	f := setup(t)
	opened := time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC)
	closed := opened.Add(time.Hour)
	admin := int64(1)
	var none *string
	insert := func(kind string, decision, category, action *string, adminID *int64, closedAt *time.Time) error {
		var contentID *uuid.UUID
		var buyerID *int64
		if kind == "BUYER" {
			b := int64(7)
			buyerID = &b
		} else {
			c := uuid.NewV7()
			contentID = &c
		}
		_, err := f.db.Conn(context.Background()).Exec(context.Background(), `INSERT INTO moderation.cases
			(id, target_kind, content_id, buyer_id, opened_at, decision, decision_category, decision_action, decided_by_admin_id, closed_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`, uuid.NewV7(), kind, contentID, buyerID, opened, decision, category, action, adminID, closedAt)
		return err
	}
	violation, noViolation := ptr("VIOLATION"), ptr("NO_VIOLATION")
	for name, c := range map[string]struct {
		kind                    string
		decision, category, act *string
		admin                   *int64
		closedAt                *time.Time
	}{
		"決定沒有管理員與結案時間":  {"COMMENT", noViolation, none, none, nil, nil},
		"有結案時間沒有決定":     {"COMMENT", none, none, none, nil, &closed},
		"違規沒有類別":        {"COMMENT", violation, none, ptr("HIDE"), &admin, &closed},
		"違規沒有處置":        {"COMMENT", violation, ptr("SPAM"), none, &admin, &closed},
		"不違規帶了類別":       {"COMMENT", noViolation, ptr("SPAM"), none, &admin, &closed},
		"貼文不能隱藏":        {"POST", violation, ptr("SPAM"), ptr("HIDE"), &admin, &closed},
		"貼文不能停權":        {"POST", violation, ptr("SPAM"), ptr("BAN"), &admin, &closed},
		"買家不能下架":        {"BUYER", violation, ptr("SPAM"), ptr("TAKEDOWN"), &admin, &closed},
		"買家不能隱藏":        {"BUYER", violation, ptr("SPAM"), ptr("HIDE"), &admin, &closed},
		"OTHER 不能當認定類別": {"COMMENT", violation, ptr("OTHER"), ptr("HIDE"), &admin, &closed},
		"結案早於開案":        {"COMMENT", noViolation, none, none, &admin, ptr(opened.Add(-time.Hour))},
	} {
		if err := insert(c.kind, c.decision, c.category, c.act, c.admin, c.closedAt); err == nil {
			t.Errorf("%s：要被 CHECK 擋下", name)
		}
	}
	for name, c := range map[string]struct {
		kind                    string
		decision, category, act *string
	}{
		"進行中":  {"COMMENT", none, none, none},
		"不違規":  {"FORUM_ENTRY", noViolation, none, none},
		"留言隱藏": {"COMMENT", violation, ptr("HATE"), ptr("HIDE")},
		"買家警告": {"BUYER", violation, ptr("SEXUAL_CONTENT"), ptr("WARN")},
		"買家停權": {"BUYER", violation, ptr("HARASSMENT"), ptr("BAN")},
		"貼文下架": {"POST", violation, ptr("FOOD_SAFETY"), ptr("TAKEDOWN")},
		"留言警告": {"COMMENT", violation, ptr("SPAM"), ptr("WARN")},
	} {
		adminID, closedAt := &admin, &closed
		if c.decision == nil {
			adminID, closedAt = nil, nil
		}
		if err := insert(c.kind, c.decision, c.category, c.act, adminID, closedAt); err != nil {
			t.Errorf("%s：合法的組合要通過：%v", name, err)
		}
	}
}

func ptr[T any](v T) *T { return &v }
