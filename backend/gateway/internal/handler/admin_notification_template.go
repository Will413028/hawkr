package handler

import (
	"github.com/Will413028/hawkr/gateway/internal/response"
	pb "github.com/Will413028/hawkr/go-pkg/proto/notification"
	"github.com/gin-gonic/gin"
)

func dataFieldsProtoToDTO(in map[string]*pb.FieldSpec) map[string]fieldSpecDTO {
	if in == nil {
		return nil
	}
	out := make(map[string]fieldSpecDTO, len(in))
	for k, spec := range in {
		if spec == nil {
			out[k] = fieldSpecDTO{}
			continue
		}
		switch s := spec.Spec.(type) {
		case *pb.FieldSpec_Const:
			v := s.Const
			out[k] = fieldSpecDTO{Const: &v}
		case *pb.FieldSpec_Field:
			v := s.Field
			out[k] = fieldSpecDTO{Field: &v}
		default:
			out[k] = fieldSpecDTO{}
		}
	}
	return out
}

// fieldSpecDTO mirrors notification.FieldSpec for admin REST API serialization.
// Exactly one of Const / Field is set:
//   - Const: literal string value (e.g. {"const":"ORDER"}).
//   - Field: payload field name to look up (e.g. {"field":"orderNo"}).
type fieldSpecDTO struct {
	Const *string `json:"const,omitempty"`
	Field *string `json:"field,omitempty"`
}

// notificationTemplateDTO is the HTTP representation of a notification template.
type notificationTemplateDTO struct {
	DataFields          map[string]fieldSpecDTO `json:"dataFields"`
	Exchange            string                  `json:"exchange"`
	RoutingKey          string                  `json:"routingKey"`
	EventType           string                  `json:"eventType"`
	TargetUserType      string                  `json:"targetUserType"`
	TargetUserField     string                  `json:"targetUserField"`
	Category            string                  `json:"category"`
	TitleTemplate       string                  `json:"titleTemplate"`
	BodyTemplate        string                  `json:"bodyTemplate"`
	BodyFallback        string                  `json:"bodyFallback,omitempty"`
	DataType            string                  `json:"dataType"`
	SchemaURL           string                  `json:"schemaUrl,omitempty"`
	ReferenceIDTemplate string                  `json:"referenceIdTemplate,omitempty"`
	CreatedAt           string                  `json:"createdAt,omitempty"`
	UpdatedAt           string                  `json:"updatedAt,omitempty"`
	ID                  string                  `json:"id,omitempty"`
	UpdatedBy           string                  `json:"updatedBy,omitempty"`
	Enabled             bool                    `json:"enabled"`
}

func protoToDTO(p *pb.NotificationTemplateProto) notificationTemplateDTO {
	return notificationTemplateDTO{
		ID:                  p.Id,
		Exchange:            p.Exchange,
		RoutingKey:          p.RoutingKey,
		EventType:           p.EventType,
		TargetUserType:      p.TargetUserType,
		TargetUserField:     p.TargetUserField,
		Category:            p.Category,
		TitleTemplate:       p.TitleTemplate,
		BodyTemplate:        p.BodyTemplate,
		BodyFallback:        p.BodyFallback,
		DataType:            p.DataType,
		DataFields:          dataFieldsProtoToDTO(p.DataFields),
		SchemaURL:           p.SchemaUrl,
		ReferenceIDTemplate: p.ReferenceIdTemplate,
		Enabled:             p.Enabled,
		UpdatedBy:           p.UpdatedBy,
		CreatedAt:           p.CreatedAt,
		UpdatedAt:           p.UpdatedAt,
	}
}

// AdminReloadTemplates
// @Summary      重新載入通知模板快取
// @Description  通知 notification-service 立刻從 DB 重抓所有模板並更新記憶體快取（部署 / 模板批次匯入後使用）
// @Tags         通知模板 Admin (Admin Notification Template)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  object{code=int,message=string,data=object{reloadedCount=string}}
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      403  {object}  object{code=int,message=string}
// @Failure      503  {object}  object{code=int,message=string}
// @Router       /admin/notification-templates/reload [post]
func (h *Holder) AdminReloadTemplates(c *gin.Context) {
	resp, err := h.Notification.ReloadTemplates(c.Request.Context(), &pb.ReloadTemplatesRequest{})
	if err != nil {
		response.GrpcErr(c, "notification", err)
		return
	}
	if resp.Code != 0 {
		response.GrpcResult(c, resp.Code, "", nil)
		return
	}
	response.OK(c, gin.H{"reloadedCount": resp.ReloadedCount})
}

// AdminListTemplates returns notification templates with optional filters.
// GET /api/v1/admin/notification-templates
func (h *Holder) AdminListTemplates(c *gin.Context) {
	resp, err := h.Notification.ListTemplates(c.Request.Context(), &pb.ListTemplatesRequest{
		EventType:      c.Query("eventType"),
		TargetUserType: c.Query("targetUserType"),
		Enabled:        c.Query("enabled"),
	})
	if err != nil {
		response.GrpcErr(c, "notification", err)
		return
	}
	if resp.Code != 0 {
		response.GrpcResult(c, resp.Code, "", nil)
		return
	}

	items := make([]notificationTemplateDTO, len(resp.Templates))
	for i, t := range resp.Templates {
		items[i] = protoToDTO(t)
	}
	response.OK(c, items)
}

// AdminGetTemplate
// @Summary      取得單筆通知模板
// @Description  依 ID 取回完整的通知模板設定（含 title/body template、data fields、routing key 等）
// @Tags         通知模板 Admin (Admin Notification Template)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id  path  string  true  "通知模板 ID"
// @Success      200  {object}  object{code=int,data=object{id=string,eventType=string,targetUserType=string,targetUserField=string,category=string,titleTemplate=string,bodyTemplate=string,bodyFallback=string,dataType=string,schemaUrl=string,referenceIdTemplate=string,enabled=bool,exchange=string,routingKey=string,updatedBy=string,createdAt=string,updatedAt=string}}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      403  {object}  object{code=int,message=string}
// @Failure      404  {object}  object{code=int,message=string}
// @Failure      503  {object}  object{code=int,message=string}
// @Router       /admin/notification-templates/{id} [get]
func (h *Holder) AdminGetTemplate(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		response.BadRequest(c, "invalid id")
		return
	}

	resp, err := h.Notification.GetTemplate(c.Request.Context(), &pb.GetTemplateRequest{Id: id})
	if err != nil {
		response.GrpcErr(c, "notification", err)
		return
	}
	if resp.Code != 0 {
		response.GrpcResult(c, resp.Code, "", nil)
		return
	}
	response.OK(c, protoToDTO(resp.Template))
}

// AdminCreateTemplate / AdminUpdateTemplate 已移除（issue #1176 SSOT 政策）：
// template 內容變更唯一入口 = Atlas migration（13 號設計書 §10.1）。

// AdminUpdateTemplateEnabled
// @Summary      切換通知模板啟用狀態
// @Description  僅切換 enabled flag，不變動其他欄位；快取會在下一次 reload 或 TTL 過期後生效
// @Tags         通知模板 Admin (Admin Notification Template)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id       path  string  true  "通知模板 ID"
// @Param        request  body  object{enabled=bool}  true  "true=啟用 / false=停用"
// @Success      200  {object}  object{code=int,message=string}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      403  {object}  object{code=int,message=string}
// @Failure      404  {object}  object{code=int,message=string}
// @Failure      503  {object}  object{code=int,message=string}
// @Router       /admin/notification-templates/{id}/enabled [patch]
func (h *Holder) AdminUpdateTemplateEnabled(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		response.BadRequest(c, "invalid id")
		return
	}

	var body struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	_, err := h.Notification.UpdateTemplateEnabled(c.Request.Context(), &pb.UpdateTemplateEnabledRequest{
		Id:      id,
		Enabled: body.Enabled,
	})
	if err != nil {
		response.GrpcErr(c, "notification", err)
		return
	}
	response.OK(c, nil)
}

// AdminPreviewTemplate
// @Summary      預覽通知模板渲染結果
// @Description  以提供的 sample_payload（JSON 字串）渲染指定模板，回傳實際 title/body/data/schema_url/reference_id，不會發送推播
// @Tags         通知模板 Admin (Admin Notification Template)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id       path  string  true  "通知模板 ID"
// @Param        request  body  object{samplePayload=string}  true  "JSON 字串格式的模擬 payload"
// @Success      200  {object}  object{code=int,message=string,data=object{renderedTitle=string,renderedBody=string,renderedData=object,renderedSchemaUrl=string,renderedReferenceId=string,missingFields=[]string}}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      403  {object}  object{code=int,message=string}
// @Failure      404  {object}  object{code=int,message=string}
// @Failure      503  {object}  object{code=int,message=string}
// @Router       /admin/notification-templates/{id}/preview [post]
func (h *Holder) AdminPreviewTemplate(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		response.BadRequest(c, "invalid id")
		return
	}

	var body struct {
		SamplePayload string `json:"samplePayload" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	resp, err := h.Notification.PreviewTemplate(c.Request.Context(), &pb.PreviewTemplateRequest{
		Id:            id,
		SamplePayload: body.SamplePayload,
	})
	if err != nil {
		response.GrpcErr(c, "notification", err)
		return
	}
	if resp.Code != 0 {
		response.GrpcResult(c, resp.Code, "", nil)
		return
	}

	response.OK(c, gin.H{
		"renderedTitle":       resp.RenderedTitle,
		"renderedBody":        resp.RenderedBody,
		"renderedData":        resp.RenderedData,
		"renderedSchemaUrl":   resp.RenderedSchemaUrl,
		"renderedReferenceId": resp.RenderedReferenceId,
		"missingFields":       resp.MissingFields,
	})
}

// AdminTestSendTemplate
// @Summary      測試發送通知模板
// @Description  以 sample_payload 渲染模板並真實推播給指定使用者，用於模板上線前的整合測試
// @Tags         通知模板 Admin (Admin Notification Template)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        id       path  string  true  "通知模板 ID"
// @Param        request  body  object{samplePayload=string,targetUserType=string,targetAccountId=string}  true  "samplePayload 為 JSON 字串；targetUserType 可省略，targetAccountId 必填"
// @Success      200  {object}  object{code=int,message=string}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      403  {object}  object{code=int,message=string}
// @Failure      404  {object}  object{code=int,message=string}
// @Failure      503  {object}  object{code=int,message=string}
// @Router       /admin/notification-templates/{id}/test-send [post]
func (h *Holder) AdminTestSendTemplate(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		response.BadRequest(c, "invalid id")
		return
	}

	var body struct {
		SamplePayload   string `json:"samplePayload" binding:"required"`
		TargetUserType  string `json:"targetUserType"`
		TargetAccountID string `json:"targetAccountId" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	_, err := h.Notification.TestSendTemplate(c.Request.Context(), &pb.TestSendTemplateRequest{
		Id:             id,
		SamplePayload:  body.SamplePayload,
		TargetUserId:   body.TargetAccountID,
		TargetUserType: body.TargetUserType,
	})
	if err != nil {
		response.GrpcErr(c, "notification", err)
		return
	}
	response.OK(c, nil)
}

// AdminBroadcastAnnouncement
// @Summary      系統公告廣播推播
// @Description  對指定使用者類型（buyer / seller / all）發送系統公告推播；targetUserType 省略時預設 all
// @Tags         通知模板 Admin (Admin Notification Template)
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        request  body  object{title=string,body=string,targetUserType=string}  true  "公告標題、內容與目標使用者類型 (buyer / seller / all)"
// @Success      200  {object}  object{code=int,message=string,data=object{broadcastId=string,recipientCount=string}}
// @Failure      400  {object}  object{code=int,message=string}
// @Failure      401  {object}  object{code=int,message=string}
// @Failure      403  {object}  object{code=int,message=string}
// @Failure      503  {object}  object{code=int,message=string}
// @Router       /admin/notifications/broadcast [post]
func (h *Holder) AdminBroadcastAnnouncement(c *gin.Context) {
	var body struct {
		Title          string `json:"title" binding:"required"`
		Body           string `json:"body" binding:"required"`
		TargetUserType string `json:"targetUserType"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	if body.TargetUserType == "" {
		body.TargetUserType = "all"
	}

	accountID := c.GetHeader("X-Account-Id")

	resp, err := h.Notification.BroadcastAnnouncement(c.Request.Context(), &pb.BroadcastAnnouncementRequest{
		Title:          body.Title,
		Body:           body.Body,
		TargetUserType: body.TargetUserType,
		OperatorId:     accountID,
	})
	if err != nil {
		response.GrpcErr(c, "notification", err)
		return
	}
	if resp.Code != 0 {
		response.GrpcResult(c, resp.Code, "", nil)
		return
	}

	// 公告識別與收件人數走 data 的具名欄位，不放進 message：message 恆為 "success"，
	// 而下游原本把人數包在「已推播給 N 位使用者」裡，呼叫端得剖析中文字串才拿得到。
	response.OK(c, gin.H{
		"broadcastId":    resp.BroadcastId,
		"recipientCount": resp.RecipientCount,
	})
}
