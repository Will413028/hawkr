package grpc

import (
	"github.com/Will413028/hawkr/go-pkg/codemeta"
	pb "github.com/Will413028/hawkr/go-pkg/proto/invoice"
	"github.com/Will413028/hawkr/invoice-service/internal/usecase"
)

const complaintActorAdmin = "ADMIN"

func toBuyerComplaintInfo(v usecase.ComplaintView) *pb.InvoiceComplaintInfo {
	return projectComplaintForBuyer(toComplaintInfo(v))
}

func projectComplaintForBuyer(info *pb.InvoiceComplaintInfo) *pb.InvoiceComplaintInfo {
	if info == nil {
		return nil
	}
	if info.GetStatus() == codemeta.InvoiceComplaintStatusViolationConfirmed.Value {
		info.Status = codemeta.InvoiceComplaintStatusResolved.Value
	}
	for _, r := range info.GetResponses() {
		if r == nil || r.GetActor() != complaintActorAdmin {
			continue
		}
		if r.GetAction() == codemeta.InvoiceComplaintActionDecidedViolation.Value {
			r.Action = codemeta.InvoiceComplaintActionDecidedResolved.Value
		}
		r.Note = ""
	}
	return info
}
