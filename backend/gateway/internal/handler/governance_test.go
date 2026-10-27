package handler

import (
	"encoding/json"
	"testing"

	sellerpb "github.com/Will413028/hawkr/go-pkg/proto/seller"
)

func TestGovernanceDTOs_EmitCamelCaseZeroValuesAndEmptyCollections(t *testing.T) {
	reportJSON, err := json.Marshal(toViolationReportDTO(&sellerpb.SellerViolationReport{
		Id:                "7490000000000000101",
		TargetSellerId:    "7490000000000000202",
		InvestigatingAtMs: 0,
		EvidenceAssetIds:  nil,
	}))
	if err != nil {
		t.Fatal(err)
	}
	report := map[string]any{}
	if err := json.Unmarshal(reportJSON, &report); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"targetSellerId", "reporterId", "investigatingAtMs", "evidenceAssetIds"} {
		if _, ok := report[key]; !ok {
			t.Errorf("violation report missing camelCase/zero-value key %q: %s", key, reportJSON)
		}
	}
	if _, ok := report["reporterAccountId"]; ok {
		t.Errorf("violation report leaked legacy identity key reporterAccountId: %s", reportJSON)
	}
	for _, key := range []string{"target_seller_id", "investigating_at_ms", "evidence_asset_ids"} {
		if _, ok := report[key]; ok {
			t.Errorf("violation report leaked proto key %q: %s", key, reportJSON)
		}
	}
	if got := report["investigatingAtMs"]; got != "0" {
		t.Errorf("investigatingAtMs = %#v, want string 0", got)
	}
	if got, ok := report["evidenceAssetIds"].([]any); !ok || len(got) != 0 {
		t.Errorf("evidenceAssetIds = %#v, want []", report["evidenceAssetIds"])
	}

	publicJSON, err := json.Marshal(toPublicViolationDTOs(nil))
	if err != nil {
		t.Fatal(err)
	}
	if string(publicJSON) != "[]" {
		t.Errorf("public violations = %s, want []", publicJSON)
	}
}
