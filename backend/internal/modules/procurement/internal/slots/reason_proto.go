package slots

var protoReasons = map[Reason]string{
	ReasonEmpty:                 "REJECTION_REASON_EMPTY",
	ReasonTruncated:             "REJECTION_REASON_TRUNCATED",
	ReasonQuantityRange:         "REJECTION_REASON_QUANTITY_RANGE",
	ReasonBudgetRange:           "REJECTION_REASON_BUDGET_RANGE",
	ReasonUnitNotAllowed:        "REJECTION_REASON_UNIT_NOT_ALLOWED",
	ReasonCityUnrecognized:      "REJECTION_REASON_CITY_UNRECOGNIZED",
	ReasonDateFormat:            "REJECTION_REASON_DATE_FORMAT",
	ReasonDateMissingYear:       "REJECTION_REASON_DATE_MISSING_YEAR",
	ReasonDatePast:              "REJECTION_REASON_DATE_PAST",
	ReasonDateTooFar:            "REJECTION_REASON_DATE_TOO_FAR",
	ReasonResolutionNoItem:      "REJECTION_REASON_RESOLUTION_NO_ITEM",
	ReasonResolutionNoLabel:     "REJECTION_REASON_RESOLUTION_NO_LABEL",
	ReasonResolutionUnknownKind: "REJECTION_REASON_RESOLUTION_UNKNOWN_KIND",
}

// ProtoName 回傳 RejectionReason 的成員名；未知的 Reason 回空字串（呼叫端視為程式錯誤）。
func (r Reason) ProtoName() string { return protoReasons[r] }

// Reasons 回傳所有 Reason。
func Reasons() []Reason {
	out := make([]Reason, 0, len(protoReasons))
	for r := range protoReasons {
		out = append(out, r)
	}
	return out
}
