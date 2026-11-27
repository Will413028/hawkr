package usecase

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)


var platformCredentialIncomplete = promauto.NewGauge(prometheus.GaugeOpts{
	Name: "invoice_platform_credential_incomplete",
	Help: "1 when AMEGO platform credential is half-configured (invoice set, app key missing) and the mobile-barcode verifier is silently disabled",
})

var mobileBarcodeVerifierEnabled = promauto.NewGauge(prometheus.GaugeOpts{
	Name: "invoice_mobile_barcode_verifier_enabled",
	Help: "1 when the AMEGO platform credential pair is complete and the mobile-barcode existence verifier is wired; 0 means invalid carriers are accepted unchecked",
})

var adminPIIReveal = promauto.NewCounter(prometheus.CounterOpts{
	Name: "invoice_admin_pii_reveal_total",
	Help: "ADM reveals of P-class invoice fields (58 M13)",
})

var documentsPDFMissing = promauto.NewCounter(prometheus.CounterOpts{
	Name: "invoice_documents_pdf_missing_total",
	Help: "Reads that found the stored pdf_object_key missing from media (58 M15)",
})

var refundExceedsInvoiced = promauto.NewCounter(prometheus.CounterOpts{
	Name: "invoice_refund_exceeds_invoiced_total",
	Help: "Refunds whose cumulative amount exceeded the invoiced total and were clamped (58 M16)",
})

func init() {
	adminPIIReveal.Add(0)
	documentsPDFMissing.Add(0)
	refundExceedsInvoiced.Add(0)
}

func observeAdminPIIReveal() { adminPIIReveal.Inc() }

func observeDocumentPDFMissing() { documentsPDFMissing.Inc() }

func observeRefundExceedsInvoiced() { refundExceedsInvoiced.Inc() }
