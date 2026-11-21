// Package bizcalendar 是 BizCalendarAdminService 的 handler：工作日行事曆（platform/bizcalendar）的匯入與涵蓋狀態。
package bizcalendar

import (
	"context"

	"connectrpc.com/connect"
	"google.golang.org/genproto/googleapis/type/date"

	adminpb "github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/bizcalendar/v1alpha1"
	"github.com/Will413028/hawkr/backend/internal/httpapi/gen/hawkr/admin/bizcalendar/v1alpha1/bizcalendarv1alpha1connect"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	"github.com/Will413028/hawkr/backend/internal/platform/adminaudit"
	"github.com/Will413028/hawkr/backend/internal/platform/bizcalendar"
)

// Source 是 handler 需要的行事曆方法（*bizcalendar.Store）。
type Source interface {
	Import(ctx context.Context, actor adminaudit.Actor, year int, content string) (bizcalendar.ImportResult, error)
	Load(ctx context.Context) (*bizcalendar.Calendar, error)
}

// Admin 實作 BizCalendarAdminService。授權（只有管理員、各方法的權限）由 rpc 的 interceptor 與 adminauth 的宣告表檢查。
type Admin struct{ src Source }

var _ bizcalendarv1alpha1connect.BizCalendarAdminServiceHandler = (*Admin)(nil)

// NewAdmin 建立後台的 handler。
func NewAdmin(src Source) *Admin { return &Admin{src: src} }

// ImportHolidayCalendar 匯入一年的辦公日曆表；操作的管理員從 ctx 取、動作是 procedure 名稱，後台紀錄與資料在 platform 的同一個 transaction。
func (h *Admin) ImportHolidayCalendar(ctx context.Context, req *connect.Request[adminpb.ImportHolidayCalendarRequest]) (*connect.Response[adminpb.ImportHolidayCalendarResponse], error) {
	admin, err := rpc.RequireAdmin(ctx)
	if err != nil {
		return nil, err
	}
	res, err := h.src.Import(ctx, adminaudit.Actor{AdminID: admin, Action: req.Spec().Procedure}, int(req.Msg.GetYear()), req.Msg.GetCsvContent())
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&adminpb.ImportHolidayCalendarResponse{
		Year:             &adminpb.HolidayCalendarYear{Year: req.Msg.GetYear(), DayCount: int32(res.DayCount), HolidayCount: int32(res.HolidayCount)}, //nolint:gosec // 一年最多 366 天
		ReplacedDayCount: int32(res.ReplacedDayCount),                                                                                                 //nolint:gosec // 同上
	}), nil
}

// GetHolidayCalendarCoverage 讀已匯入的年份。
func (h *Admin) GetHolidayCalendarCoverage(ctx context.Context, _ *connect.Request[adminpb.GetHolidayCalendarCoverageRequest]) (*connect.Response[adminpb.GetHolidayCalendarCoverageResponse], error) {
	cal, err := h.src.Load(ctx)
	if err != nil {
		return nil, err
	}
	out := &adminpb.GetHolidayCalendarCoverageResponse{}
	for _, y := range cal.Coverage() {
		out.Years = append(out.Years, &adminpb.HolidayCalendarYear{Year: int32(y.Year), DayCount: int32(y.DayCount), HolidayCount: int32(y.HolidayCount)}) //nolint:gosec // 年份在 1..9999、天數至多 366
	}
	if last, ok := cal.LastCoveredDate(); ok {
		out.LastCoveredDate = &date.Date{Year: int32(last.Year()), Month: int32(last.Month()), Day: int32(last.Day())} //nolint:gosec // time.Time 的日曆欄位在 Date 的範圍內
	}
	return connect.NewResponse(out), nil
}
