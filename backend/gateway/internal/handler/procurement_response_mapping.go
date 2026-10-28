package handler

import (
	"encoding/json"

	procurementwire "github.com/Will413028/hawkr/gateway/internal/procurement"
	pb "github.com/Will413028/hawkr/go-pkg/proto/ai"
)

type ProcurementTurnRequest struct {
	RequestID       string `json:"requestId"`
	Message         string `json:"message"`
	ClientRequestID string `json:"clientRequestId"`
}

type ProcurementSelectionRequest struct {
	CandidateSetID string `json:"candidateSetId"`
	Lane           string `json:"lane"`
	CandidateID    string `json:"candidateId"`
	Selected       bool   `json:"selected"`
}

type ProcurementAnswerRequest struct {
	SlotKey            string `json:"slotKey"`
	OptionID           string `json:"optionId"`
	FreeText           string `json:"freeText"`
	UnitOptionID       string `json:"unitOptionId"`
	Skip               bool   `json:"skip"`
	ExpectedRevisionID string `json:"expectedRevisionId"`
	ClientRequestID    string `json:"clientRequestId"`
}

type ProcurementRewindRequest struct {
	SlotKey         string `json:"slotKey"`
	ClientRequestID string `json:"clientRequestId"`
}

type ProcurementBuyerRequestedRangeDTO struct {
	Currency         string `json:"currency"`
	PriceComponent   string `json:"priceComponent"`
	LowerBoundAmount string `json:"lowerBoundAmount"`
	UpperBoundAmount string `json:"upperBoundAmount"`
	BasisQuantity    string `json:"basisQuantity"`
	BasisUnit        string `json:"basisUnit"`
}

type ProcurementRfqPricePreviewRequest struct {
	ExpectedRevisionID  string                             `json:"expectedRevisionId"`
	ClientRequestID     string                             `json:"clientRequestId"`
	DeliveryCity        string                             `json:"deliveryCity"`
	BuyerRequestedRange *ProcurementBuyerRequestedRangeDTO `json:"buyerRequestedRange"`
}

func (r ProcurementRfqPricePreviewRequest) toProto(requestID string) *pb.RfqPricePreviewRequest {
	req := &pb.RfqPricePreviewRequest{
		RequestId: requestID, ExpectedRevisionId: r.ExpectedRevisionID,
		ClientRequestId: r.ClientRequestID, DeliveryCity: r.DeliveryCity,
	}
	if r.BuyerRequestedRange != nil {
		req.BuyerRequestedRange = &pb.BuyerRequestedRange{
			Currency: r.BuyerRequestedRange.Currency, PriceComponent: r.BuyerRequestedRange.PriceComponent,
			LowerBoundAmount: r.BuyerRequestedRange.LowerBoundAmount, UpperBoundAmount: r.BuyerRequestedRange.UpperBoundAmount,
		}
	}
	return req
}

type ProcurementRenameConversationRequest struct {
	Title           string `json:"title"`
	ClientRequestID string `json:"clientRequestId"`
}

type ProcurementDeleteConversationRequest struct {
	ClientRequestID string `json:"clientRequestId"`
}

type ProcurementRunCancelRequest struct {
	ClientRequestID    string `json:"clientRequestId"`
	ExpectedRunVersion string `json:"expectedRunVersion"`
	Reason             string `json:"reason"`
}

type ProcurementRunResumeRequest struct {
	InterruptID     string          `json:"interruptId"`
	ClientRequestID string          `json:"clientRequestId"`
	Payload         json.RawMessage `json:"payload"`
}

type ProcurementPublicConfirmRequest struct {
	Token string `json:"token"`
}

func turnResultFromProto(r *pb.TurnResult) procurementwire.TurnResultDTO {
	return procurementwire.TurnResultFromProto(r)
}

type ProcurementCropDisambiguationOptionDTO struct {
	Label string `json:"label"`
}

func procurementCropDisambiguationOptionsFromProto(opts []*pb.CropDisambiguationOption) []ProcurementCropDisambiguationOptionDTO {
	out := make([]ProcurementCropDisambiguationOptionDTO, 0, len(opts))
	for _, o := range opts {
		out = append(out, ProcurementCropDisambiguationOptionDTO{Label: o.GetLabel()})
	}
	return out
}

type ProcurementCropResolutionDTO struct {
	Candidates []ProcurementCropDisambiguationOptionDTO `json:"candidates"`
}

type ProcurementAgentRunSummaryDTO struct {
	RunID             string `json:"runId"`
	RunType           string `json:"runType"`
	Status            string `json:"status"`
	LastSeqID         string `json:"lastSeqId"`
	MinAvailableSeqID string `json:"minAvailableSeqId"`
	Terminal          bool   `json:"terminal"`
	// RunVersion 是 POST /runs/{runId}/cancel 的必填 expectedRunVersion 在
	// 瀏覽器端的唯一來源（doc57 §5.4 的 CAS token）。欄位名與 cancel 回應的
	// ProcurementRunCancelResponseDTO.RunVersion 刻意相同：兩處指的是同一個
	// procurement_agent_runs.version，呼叫前讀這裡、呼叫後從回應拿新值。
	RunVersion string `json:"runVersion"`
}

func procurementAgentRunSummariesFromProto(runs []*pb.AgentRunSummary) []ProcurementAgentRunSummaryDTO {
	out := make([]ProcurementAgentRunSummaryDTO, 0, len(runs))
	for _, r := range runs {
		out = append(out, ProcurementAgentRunSummaryDTO{
			RunID: r.GetRunId(), RunType: r.GetRunType(), Status: r.GetStatus(),
			LastSeqID: r.GetLastSeqId(), MinAvailableSeqID: r.GetMinAvailableSeqId(),
			Terminal: r.GetTerminal(), RunVersion: r.GetVersion(),
		})
	}
	return out
}

type ProcurementAgentRunOpenConfirmationDTO struct {
	InterruptID      string                              `json:"interruptId"`
	ConfirmationKind string                              `json:"confirmationKind"`
	ExpiresAt        string                              `json:"expiresAt"`
	Question         *procurementwire.ClarifyQuestionDTO `json:"question,omitempty"`
}

type ProcurementSnapshotResponseDTO struct {
	RequestID            string                                  `json:"requestId"`
	RevisionID           string                                  `json:"revisionId"`
	Status               string                                  `json:"status"`
	Slots                procurementwire.SlotsDTO                `json:"slots"`
	MissingRequired      []string                                `json:"missingRequired"`
	CropResolution       ProcurementCropResolutionDTO            `json:"cropResolution"`
	Runs                 []ProcurementAgentRunSummaryDTO         `json:"runs"`
	ActiveRunID          string                                  `json:"activeRunId,omitempty"`
	OpenConfirmation     *ProcurementAgentRunOpenConfirmationDTO `json:"openConfirmation,omitempty"`
	PublishedPublication *ProcurementPublishedPublicationDTO     `json:"publishedPublication,omitempty"`
}

type ProcurementPublishedPublicationDTO struct {
	PublicationID string `json:"publicationId"`
	Version       string `json:"version"`
	PublishedAt   string `json:"publishedAt"`
}

func snapshotResponseFromProto(r *pb.SnapshotResponse) ProcurementSnapshotResponseDTO {
	dto := ProcurementSnapshotResponseDTO{
		RequestID: r.GetRequestId(), RevisionID: r.GetRevisionId(), Status: r.GetStatus(),
		Slots:           procurementwire.SlotsFromProto(r.GetSlots()),
		MissingRequired: procurementwire.MissingRequiredFromProto(r.GetMissingRequired()),
		CropResolution:  ProcurementCropResolutionDTO{Candidates: []ProcurementCropDisambiguationOptionDTO{}},
		Runs:            procurementAgentRunSummariesFromProto(r.GetRuns()),
		ActiveRunID:     r.GetActiveRunId(),
	}
	if cr := r.GetCropResolution(); cr != nil {
		dto.CropResolution = ProcurementCropResolutionDTO{
			Candidates: procurementCropDisambiguationOptionsFromProto(cr.GetCandidates()),
		}
	}
	if oc := r.GetOpenConfirmation(); oc != nil {
		dto.OpenConfirmation = &ProcurementAgentRunOpenConfirmationDTO{
			InterruptID: oc.GetInterruptId(), ConfirmationKind: oc.GetConfirmationKind(), ExpiresAt: oc.GetExpiresAt(),
			Question: procurementwire.ClarifyQuestionFromProto(oc.GetQuestion()),
		}
	}
	if pp := r.GetPublishedPublication(); pp != nil {
		dto.PublishedPublication = &ProcurementPublishedPublicationDTO{
			PublicationID: pp.GetPublicationId(), Version: pp.GetVersion(), PublishedAt: pp.GetPublishedAt(),
		}
	}
	return dto
}

type ProcurementCandidateItemDTO struct {
	CandidateID       string          `json:"candidateId"`
	SourceID          string          `json:"sourceId"`
	Lane              string          `json:"lane"`
	DisplayName       string          `json:"displayName"`
	Score             float64         `json:"score"`
	MatchedFacts      json.RawMessage `json:"matchedFacts"`
	UnverifiableFacts json.RawMessage `json:"unverifiableFacts"`
	FactsAsOf         string          `json:"factsAsOf"`
	Selected          bool            `json:"selected"`

	StoreID    string `json:"storeId,omitempty"`
	SellerName string `json:"sellerName,omitempty"`
	StoreCode  string `json:"storeCode,omitempty"`
	LogoKey    string `json:"logoKey,omitempty"`
	LogoURL    string `json:"logoUrl,omitempty"`

	// 站外業者的品牌素材。**與 logoKey／logoUrl 是兩個不同的軸，消費端不得
	// 互相 fallback**：那一組是平台商家的身分，這一組只是「這家站外業者自己的
	// 品牌圖」，不表示平台已驗證、已認證或已入駐。同一張卡上恆有
	// unverifiableFacts 的常設揭露，呈現時不得把它拿掉或折疊成看不見。
	BrandLogoKey  string `json:"brandLogoKey,omitempty"`
	BrandLogoURL  string `json:"brandLogoUrl,omitempty"`
	BrandCoverKey string `json:"brandCoverKey,omitempty"`
	BrandCoverURL string `json:"brandCoverUrl,omitempty"`
}

func procurementCandidateItemsFromProto(items []*pb.CandidateItem, mediaBaseURL string) []ProcurementCandidateItemDTO {
	out := make([]ProcurementCandidateItemDTO, 0, len(items))
	for _, it := range items {
		out = append(out, ProcurementCandidateItemDTO{
			CandidateID: it.GetCandidateId(), SourceID: it.GetSourceId(), Lane: it.GetLane(),
			DisplayName:       it.GetDisplayName(),
			Score:             it.GetScore(),
			MatchedFacts:      json.RawMessage(it.GetMatchedFactsJson()),
			UnverifiableFacts: json.RawMessage(it.GetUnverifiableFactsJson()),
			FactsAsOf:         it.GetFactsAsOf(), Selected: it.GetSelected(),
			StoreID:    it.GetStoreId(),
			SellerName: it.GetSellerName(),
			StoreCode:  it.GetStoreCode(),
			LogoKey:    it.GetLogoKey(),
			LogoURL:    buildURL(mediaBaseURL, it.GetLogoKey()),

			BrandLogoKey:  it.GetBrandLogoKey(),
			BrandLogoURL:  buildURL(mediaBaseURL, it.GetBrandLogoKey()),
			BrandCoverKey: it.GetBrandCoverKey(),
			BrandCoverURL: buildURL(mediaBaseURL, it.GetBrandCoverKey()),
		})
	}
	return out
}

type ProcurementCandidateLanesDTO struct {
	Platform []ProcurementCandidateItemDTO `json:"platform"`
	External []ProcurementCandidateItemDTO `json:"external"`
}

type ProcurementCandidateSetResponseDTO struct {
	RequestID          string                       `json:"requestId"`
	RunID              string                       `json:"runId"`
	CandidateSetID     string                       `json:"candidateSetId"`
	Status             string                       `json:"status"`
	PlatformLaneStatus string                       `json:"platformLaneStatus"`
	ExternalLaneStatus string                       `json:"externalLaneStatus"`
	FailureReason      string                       `json:"failureReason"`
	Candidates         ProcurementCandidateLanesDTO `json:"candidates"`
}

func candidateSetResponseFromProto(r *pb.CandidateSetResponse, mediaBaseURL string) ProcurementCandidateSetResponseDTO {
	lanes := r.GetCandidates()
	return ProcurementCandidateSetResponseDTO{
		RequestID: r.GetRequestId(), RunID: r.GetRunId(), CandidateSetID: r.GetCandidateSetId(),
		Status: r.GetStatus(), PlatformLaneStatus: r.GetPlatformLaneStatus(), ExternalLaneStatus: r.GetExternalLaneStatus(),
		FailureReason: r.GetFailureReason(),
		Candidates: ProcurementCandidateLanesDTO{
			Platform: procurementCandidateItemsFromProto(lanes.GetPlatform(), mediaBaseURL),
			External: procurementCandidateItemsFromProto(lanes.GetExternal(), mediaBaseURL),
		},
	}
}

type ProcurementQuoteTotalRangeDTO struct {
	Currency         string `json:"currency"`
	LowerBoundAmount string `json:"lowerBoundAmount"`
	UpperBoundAmount string `json:"upperBoundAmount"`
	Quantity         string `json:"quantity"`
	QuantityUnit     string `json:"quantityUnit"`
}

type ProcurementRfqPricePreviewResponseDTO struct {
	PricePreviewID        string                             `json:"pricePreviewId"`
	Version               string                             `json:"version"`
	QualityKnowledgeState string                             `json:"qualityKnowledgeState"`
	BuyerRequestedRange   *ProcurementBuyerRequestedRangeDTO `json:"buyerRequestedRange"`
	PayloadHash           string                             `json:"payloadHash"`
	QuoteTotalRange       *ProcurementQuoteTotalRangeDTO     `json:"quoteTotalRange"`
}

func rfqPricePreviewResponseFromProto(r *pb.RfqPricePreviewResponse) ProcurementRfqPricePreviewResponseDTO {
	dto := ProcurementRfqPricePreviewResponseDTO{
		PricePreviewID: r.GetPricePreviewId(), Version: r.GetVersion(),
		QualityKnowledgeState: r.GetQualityKnowledgeState(), PayloadHash: r.GetPayloadHash(),
	}
	if br := r.GetBuyerRequestedRange(); br != nil {
		dto.BuyerRequestedRange = &ProcurementBuyerRequestedRangeDTO{
			Currency: br.GetCurrency(), PriceComponent: br.GetPriceComponent(),
			LowerBoundAmount: br.GetLowerBoundAmount(), UpperBoundAmount: br.GetUpperBoundAmount(),
			BasisQuantity: br.GetBasisQuantity(), BasisUnit: br.GetBasisUnit(),
		}
	}
	if q := r.GetQuoteTotalRange(); q != nil {
		dto.QuoteTotalRange = &ProcurementQuoteTotalRangeDTO{
			Currency: q.GetCurrency(), LowerBoundAmount: q.GetLowerBoundAmount(),
			UpperBoundAmount: q.GetUpperBoundAmount(), Quantity: q.GetQuantity(),
			QuantityUnit: q.GetQuantityUnit(),
		}
	}
	return dto
}

type ProcurementRfqConfirmationRequest struct {
	ClientRequestID     string `json:"clientRequestId"`
	ExpectedRevisionID  string `json:"expectedRevisionId"`
	PricePreviewID      string `json:"pricePreviewId"`
	PricePreviewVersion string `json:"pricePreviewVersion"`
	PayloadHash         string `json:"payloadHash"`
	BuyerNote           string `json:"buyerNote"`
}

func (r ProcurementRfqConfirmationRequest) toProto(requestID string) *pb.RfqConfirmationRequest {
	return &pb.RfqConfirmationRequest{
		RequestId: requestID, ClientRequestId: r.ClientRequestID, ExpectedRevisionId: r.ExpectedRevisionID,
		PricePreviewId: r.PricePreviewID, PricePreviewVersion: r.PricePreviewVersion, PayloadHash: r.PayloadHash,
		BuyerNote: r.BuyerNote,
	}
}

type ProcurementRfqConfirmationResponseDTO struct {
	ConfirmedRfqSnapshotID string `json:"confirmedRfqSnapshotId"`
	PayloadHash            string `json:"payloadHash"`
	ExpiresAt              string `json:"expiresAt"`
}

func rfqConfirmationResponseFromProto(r *pb.RfqConfirmationResponse) ProcurementRfqConfirmationResponseDTO {
	return ProcurementRfqConfirmationResponseDTO{
		ConfirmedRfqSnapshotID: r.GetConfirmedRfqSnapshotId(), PayloadHash: r.GetPayloadHash(),
		ExpiresAt: r.GetExpiresAt(),
	}
}

type ProcurementRfqPublicationPreviewRequest struct {
	PreviewClientRequestID string   `json:"previewClientRequestId"`
	ExpectedRevisionID     string   `json:"expectedRevisionId"`
	ConfirmedRfqSnapshotID string   `json:"confirmedRfqSnapshotId"`
	AudienceMode           string   `json:"audienceMode"`
	CandidateSetID         string   `json:"candidateSetId"`
	SelectedCandidateIDs   []string `json:"selectedCandidateIds"`
	ResponseDeadline       *string  `json:"responseDeadline,omitempty"`
}

func (r ProcurementRfqPublicationPreviewRequest) toProto(requestID string) *pb.RfqPublicationPreviewRequest {
	return &pb.RfqPublicationPreviewRequest{
		RequestId:              requestID,
		PreviewClientRequestId: r.PreviewClientRequestID,
		ExpectedRevisionId:     r.ExpectedRevisionID,
		ConfirmedRfqSnapshotId: r.ConfirmedRfqSnapshotID,
		AudienceMode:           r.AudienceMode,
		CandidateSetId:         r.CandidateSetID,
		SelectedCandidateIds:   r.SelectedCandidateIDs,
		ResponseDeadline:       r.ResponseDeadline,
	}
}

type ProcurementRfqPublicationPreviewResponseDTO struct {
	ApprovalID              string `json:"approvalId"`
	ApprovalVersion         string `json:"approvalVersion"`
	AudienceHash            string `json:"audienceHash"`
	SellerFacingPayloadHash string `json:"sellerFacingPayloadHash"`
	SellerFacingPayloadJSON string `json:"sellerFacingPayloadJson"`
	ExpiresAt               string `json:"expiresAt"`
	AudiencePolicyID        string `json:"audiencePolicyId"`
	AudiencePolicyVersion   string `json:"audiencePolicyVersion"`
	RecipientCount          string `json:"recipientCount"`
}

func rfqPublicationPreviewResponseFromProto(r *pb.RfqPublicationPreviewResponse) ProcurementRfqPublicationPreviewResponseDTO {
	return ProcurementRfqPublicationPreviewResponseDTO{
		ApprovalID:              r.GetApprovalId(),
		ApprovalVersion:         r.GetApprovalVersion(),
		AudienceHash:            r.GetAudienceHash(),
		SellerFacingPayloadHash: r.GetSellerFacingPayloadHash(),
		SellerFacingPayloadJSON: r.GetSellerFacingPayloadJson(),
		ExpiresAt:               r.GetExpiresAt(),
		AudiencePolicyID:        r.GetAudiencePolicyId(),
		AudiencePolicyVersion:   r.GetAudiencePolicyVersion(),
		RecipientCount:          r.GetRecipientCount(),
	}
}

type ProcurementRfqPublicationExecuteRequest struct {
	ClientRequestID    string `json:"clientRequestId"`
	ApprovalID         string `json:"approvalId"`
	ApprovalVersion    string `json:"approvalVersion"`
	ExpectedRevisionID string `json:"expectedRevisionId"`
	PayloadHash        string `json:"payloadHash"`
}

func (r ProcurementRfqPublicationExecuteRequest) toProto(requestID string) *pb.RfqPublicationExecuteRequest {
	return &pb.RfqPublicationExecuteRequest{
		RequestId:          requestID,
		ClientRequestId:    r.ClientRequestID,
		ApprovalId:         r.ApprovalID,
		ApprovalVersion:    r.ApprovalVersion,
		ExpectedRevisionId: r.ExpectedRevisionID,
		PayloadHash:        r.PayloadHash,
	}
}

type ProcurementRfqPublicationExecuteResponseDTO struct {
	PublicationID           string `json:"publicationId"`
	PublicationVersion      string `json:"publicationVersion"`
	State                   string `json:"state"`
	AudienceMode            string `json:"audienceMode"`
	AudienceHash            string `json:"audienceHash"`
	SellerFacingPayloadHash string `json:"sellerFacingPayloadHash"`
	PublishedAt             string `json:"publishedAt"`
	Replayed                bool   `json:"replayed"`
}

func rfqPublicationExecuteResponseFromProto(r *pb.RfqPublicationExecuteResponse) ProcurementRfqPublicationExecuteResponseDTO {
	return ProcurementRfqPublicationExecuteResponseDTO{
		PublicationID:           r.GetPublicationId(),
		PublicationVersion:      r.GetPublicationVersion(),
		State:                   r.GetState(),
		AudienceMode:            r.GetAudienceMode(),
		AudienceHash:            r.GetAudienceHash(),
		SellerFacingPayloadHash: r.GetSellerFacingPayloadHash(),
		PublishedAt:             r.GetPublishedAt(),
		Replayed:                r.GetReplayed(),
	}
}

type ProcurementConversationSummaryDTO struct {
	ConversationID     string `json:"conversationId"`
	Title              string `json:"title"`
	LastMessagePreview string `json:"lastMessagePreview"`
	UpdatedAt          string `json:"updatedAt"`
}

func procurementConversationSummariesFromProto(records []*pb.ConversationSummary) []ProcurementConversationSummaryDTO {
	out := make([]ProcurementConversationSummaryDTO, 0, len(records))
	for _, r := range records {
		out = append(out, ProcurementConversationSummaryDTO{
			ConversationID: r.GetConversationId(), Title: r.GetTitle(),
			LastMessagePreview: r.GetLastMessagePreview(), UpdatedAt: r.GetUpdatedAt(),
		})
	}
	return out
}

type ProcurementListConversationsResponseDTO struct {
	Records      []ProcurementConversationSummaryDTO `json:"records"`
	HasMore      bool                                `json:"hasMore"`
	NextCursorID string                              `json:"nextCursorId"`
}

func listConversationsResponseFromProto(r *pb.ListConversationsResponse) ProcurementListConversationsResponseDTO {
	return ProcurementListConversationsResponseDTO{
		Records: procurementConversationSummariesFromProto(r.GetRecords()), HasMore: r.GetHasMore(), NextCursorID: r.GetNextCursorId(),
	}
}

type ProcurementConversationMessageDTO struct {
	ID               string                             `json:"id"`
	Role             string                             `json:"role"`
	Text             string                             `json:"text"`
	Steps            []procurementwire.StepDTO          `json:"steps"`
	RecordedThisTurn []procurementwire.RecordedFieldDTO `json:"recordedThisTurn"`
	CreatedAt        string                             `json:"createdAt"`
}

func procurementConversationMessagesFromProto(records []*pb.ConversationMessage) []ProcurementConversationMessageDTO {
	out := make([]ProcurementConversationMessageDTO, 0, len(records))
	for _, m := range records {
		out = append(out, ProcurementConversationMessageDTO{
			ID: m.GetId(), Role: m.GetRole(), Text: m.GetText(),
			Steps:            procurementwire.StepsFromProto(m.GetSteps()),
			RecordedThisTurn: procurementwire.RecordedFieldsFromProto(m.GetRecordedThisTurn()),
			CreatedAt:        m.GetCreatedAt(),
		})
	}
	return out
}

type ProcurementConversationMessagesResponseDTO struct {
	ConversationID string                              `json:"conversationId"`
	RequestID      string                              `json:"requestId"`
	Status         string                              `json:"status"`
	Title          string                              `json:"title"`
	Records        []ProcurementConversationMessageDTO `json:"records"`
}

func conversationMessagesResponseFromProto(r *pb.ConversationMessagesResponse) ProcurementConversationMessagesResponseDTO {
	return ProcurementConversationMessagesResponseDTO{
		ConversationID: r.GetConversationId(), RequestID: r.GetRequestId(), Status: r.GetStatus(), Title: r.GetTitle(),
		Records: procurementConversationMessagesFromProto(r.GetRecords()),
	}
}

type ProcurementAgentRunArtifactDTO struct {
	ArtifactID    string          `json:"artifactId"`
	RunID         string          `json:"runId"`
	ArtifactType  string          `json:"artifactType"`
	Version       string          `json:"version"`
	Status        string          `json:"status"`
	Data          json.RawMessage `json:"data"`
	Checksum      string          `json:"checksum"`
	DomainRefType string          `json:"domainRefType,omitempty"`
	DomainRefID   string          `json:"domainRefId,omitempty"`
	AsOf          string          `json:"asOf"`
	CreatedAt     string          `json:"createdAt"`
}

func procurementAgentRunArtifactsFromProto(items []*pb.AgentRunArtifactView) []ProcurementAgentRunArtifactDTO {
	out := make([]ProcurementAgentRunArtifactDTO, 0, len(items))
	for _, a := range items {
		out = append(out, ProcurementAgentRunArtifactDTO{
			ArtifactID: a.GetArtifactId(), RunID: a.GetRunId(),
			ArtifactType: a.GetArtifactType(), Version: a.GetVersion(), Status: a.GetStatus(),
			Data:          json.RawMessage(a.GetDataJson()),
			Checksum:      a.GetChecksum(),
			DomainRefType: a.GetDomainRefType(), DomainRefID: a.GetDomainRefId(),
			AsOf: a.GetAsOf(), CreatedAt: a.GetCreatedAt(),
		})
	}
	return out
}

type ProcurementRuntimeSnapshotResponseDTO struct {
	ConversationID   string                                  `json:"conversationId"`
	RequestID        string                                  `json:"requestId"`
	Status           string                                  `json:"status"`
	Title            string                                  `json:"title"`
	Messages         []ProcurementConversationMessageDTO     `json:"messages"`
	Runs             []ProcurementAgentRunSummaryDTO         `json:"runs"`
	ActiveRunID      string                                  `json:"activeRunId,omitempty"`
	Artifacts        []ProcurementAgentRunArtifactDTO        `json:"artifacts"`
	OpenConfirmation *ProcurementAgentRunOpenConfirmationDTO `json:"openConfirmation,omitempty"`
}

func runtimeSnapshotResponseFromProto(r *pb.RuntimeSnapshotResponse) ProcurementRuntimeSnapshotResponseDTO {
	dto := ProcurementRuntimeSnapshotResponseDTO{
		ConversationID: r.GetConversationId(), RequestID: r.GetRequestId(),
		Status: r.GetStatus(), Title: r.GetTitle(),
		Messages:    procurementConversationMessagesFromProto(r.GetMessages()),
		Runs:        procurementAgentRunSummariesFromProto(r.GetRuns()),
		ActiveRunID: r.GetActiveRunId(),
		Artifacts:   procurementAgentRunArtifactsFromProto(r.GetArtifacts()),
	}
	if oc := r.GetOpenConfirmation(); oc != nil {
		dto.OpenConfirmation = &ProcurementAgentRunOpenConfirmationDTO{
			InterruptID: oc.GetInterruptId(), ConfirmationKind: oc.GetConfirmationKind(), ExpiresAt: oc.GetExpiresAt(),
		}
	}
	return dto
}

type ProcurementStartCandidateSearchResponseDTO struct {
	CandidateSetID string `json:"candidateSetId"`
	RequestID      string `json:"requestId"`
	RunID          string `json:"runId"`
	Status         string `json:"status"`
}

func startCandidateSearchResponseFromProto(r *pb.StartCandidateSearchResponse) ProcurementStartCandidateSearchResponseDTO {
	return ProcurementStartCandidateSearchResponseDTO{
		CandidateSetID: r.GetCandidateSetId(), RequestID: r.GetRequestId(),
		RunID: r.GetRunId(), Status: r.GetStatus(),
	}
}

type ProcurementSelectionResponseDTO struct {
	CandidateID    string `json:"candidateId"`
	CandidateSetID string `json:"candidateSetId"`
	Lane           string `json:"lane"`
	RequestID      string `json:"requestId"`
	Selected       bool   `json:"selected"`
}

func selectionResponseFromProto(r *pb.ToggleSelectionResponse) ProcurementSelectionResponseDTO {
	return ProcurementSelectionResponseDTO{
		CandidateID: r.GetCandidateId(), CandidateSetID: r.GetCandidateSetId(),
		Lane: r.GetLane(), RequestID: r.GetRequestId(), Selected: r.GetSelected(),
	}
}

type ProcurementRunCancelResponseDTO struct {
	CancelDisposition string `json:"cancelDisposition"`
	RunID             string `json:"runId"`
	RunStatus         string `json:"runStatus"`
	RunVersion        string `json:"runVersion"`
	SchemaVersion     string `json:"schemaVersion"`
}

func runCancelResponseFromProto(r *pb.CancelRunResponse) ProcurementRunCancelResponseDTO {
	return ProcurementRunCancelResponseDTO{
		CancelDisposition: r.GetCancelDisposition(), RunID: r.GetRunId(),
		RunStatus: r.GetRunStatus(), RunVersion: r.GetRunVersion(),
		SchemaVersion: "hawkr.agent.cancel.v1",
	}
}

type ProcurementRunResumeResponseDTO struct {
	RunID     string                         `json:"runId"`
	RunStatus string                         `json:"runStatus"`
	Result    *procurementwire.TurnResultDTO `json:"result,omitempty"`
}

func runResumeResponseFromProto(r *pb.ResumeRunResponse) ProcurementRunResumeResponseDTO {
	dto := ProcurementRunResumeResponseDTO{RunID: r.GetRunId(), RunStatus: r.GetRunStatus()}
	if tr := r.GetResult(); tr != nil {
		mapped := turnResultFromProto(tr)
		dto.Result = &mapped
	}
	return dto
}
