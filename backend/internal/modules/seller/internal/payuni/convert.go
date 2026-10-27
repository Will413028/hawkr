package payuni

import (
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/model"
	"github.com/Will413028/hawkr/backend/internal/modules/seller/internal/store"
)

func toObservation(row store.SellerPayuniObservation) model.PayUniObservation {
	return model.PayUniObservation{
		ID: row.ObservationID, Source: model.PayUniSource(row.Source), EventKey: row.EventKey,
		MerStatus: row.MerStatus, MemberStatus: row.MemberStatus, CreditStatus: row.CreditStatus,
		ApplePayStatus: row.ApplepayStatus, GooglePayStatus: row.GooglepayStatus, ATMStatus: row.AtmStatus, Reason: row.Reason,
		ExecutedAt: row.ExecutedAt, ReceivedAt: row.ReceivedAt,
	}
}
