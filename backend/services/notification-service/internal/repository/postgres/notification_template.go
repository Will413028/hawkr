package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Will413028/hawkr/go-pkg/database"
	"github.com/Will413028/hawkr/notification-service/internal/domain"
	"github.com/Will413028/hawkr/notification-service/internal/repository/postgres/gen"
)

type notificationTemplateRepository struct {
	pool *pgxpool.Pool
	tsid func() int64
}

func NewNotificationTemplateRepository(pool *pgxpool.Pool, tsid func() int64) domain.NotificationTemplateRepository {
	return &notificationTemplateRepository{pool: pool, tsid: tsid}
}

func (r *notificationTemplateRepository) FindAll(ctx context.Context) ([]*domain.NotificationTemplate, error) {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	rows, err := q.GetAllTemplates(ctx)
	if err != nil {
		return nil, err
	}
	return rowsToDomain(rows)
}

func (r *notificationTemplateRepository) FindEnabled(ctx context.Context) ([]*domain.NotificationTemplate, error) {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	rows, err := q.GetEnabledTemplates(ctx)
	if err != nil {
		return nil, err
	}
	return rowsToDomain(rows)
}

func (r *notificationTemplateRepository) FindByID(ctx context.Context, id int64) (*domain.NotificationTemplate, error) {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	row, err := q.GetTemplateByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return genToDomain(row)
}

func (r *notificationTemplateRepository) Insert(ctx context.Context, t *domain.NotificationTemplate) (int64, error) {
	params, err := domainToInsertParams(t, r.tsid())
	if err != nil {
		return 0, err
	}
	q := gen.New(database.GetDBTX(ctx, r.pool))
	result, err := q.InsertTemplate(ctx, params)
	if err != nil {
		return 0, err
	}
	return result.ID, nil
}

func (r *notificationTemplateRepository) Update(ctx context.Context, t *domain.NotificationTemplate) error {
	params, err := domainToUpdateParams(t)
	if err != nil {
		return err
	}
	q := gen.New(database.GetDBTX(ctx, r.pool))
	return q.UpdateTemplate(ctx, params)
}

func (r *notificationTemplateRepository) UpdateEnabled(ctx context.Context, id int64, enabled bool) error {
	q := gen.New(database.GetDBTX(ctx, r.pool))
	return q.UpdateTemplateEnabled(ctx, gen.UpdateTemplateEnabledParams{ID: id, Enabled: enabled})
}

// --- domain ↔ gen conversion helpers ---

func rowsToDomain(rows []gen.NotificationTemplate) ([]*domain.NotificationTemplate, error) {
	result := make([]*domain.NotificationTemplate, 0, len(rows))
	for i := range rows {
		t, err := genToDomain(rows[i])
		if err != nil {
			return nil, err
		}
		result = append(result, t)
	}
	return result, nil
}

func genToDomain(g gen.NotificationTemplate) (*domain.NotificationTemplate, error) {
	t := &domain.NotificationTemplate{
		ID:                  g.ID,
		Exchange:            g.Exchange,
		RoutingKey:          g.RoutingKey,
		EventType:           g.EventType,
		TargetUserType:      g.TargetUserType,
		TargetUserField:     g.TargetUserField,
		Category:            g.Category,
		TitleTemplate:       g.TitleTemplate,
		BodyTemplate:        g.BodyTemplate,
		BodyFallback:        g.BodyFallback,
		DataType:            g.DataType,
		SchemaURL:           g.SchemaUrl,
		ReferenceIDTemplate: g.ReferenceIDTemplate,
		ImageKeyTemplate:    g.ImageKeyTemplate,
		Enabled:             g.Enabled,
		PushMode:            g.PushMode,
		UpdatedBy:           g.UpdatedBy,
		IdentityMigrated:    g.IdentityMigrated,
	}
	if len(g.DataFields) > 0 {
		if err := json.Unmarshal(g.DataFields, &t.DataFields); err != nil {
			return nil, fmt.Errorf("unmarshal data_fields for template %d: %w", g.ID, err)
		}
	}
	if len(g.PayloadFilter) > 0 {
		if err := json.Unmarshal(g.PayloadFilter, &t.PayloadFilter); err != nil {
			return nil, fmt.Errorf("unmarshal payload_filter for template %d: %w", g.ID, err)
		}
	}
	return t, nil
}

func domainToInsertParams(t *domain.NotificationTemplate, id int64) (gen.InsertTemplateParams, error) {
	dataFieldsJSON, err := marshalNullableJSON(t.DataFields)
	if err != nil {
		return gen.InsertTemplateParams{}, fmt.Errorf("marshal data_fields: %w", err)
	}
	payloadFilterJSON, err := marshalNullableJSON(t.PayloadFilter)
	if err != nil {
		return gen.InsertTemplateParams{}, fmt.Errorf("marshal payload_filter: %w", err)
	}
	pushMode := t.PushMode
	if pushMode == "" {
		pushMode = "visible"
	}
	return gen.InsertTemplateParams{
		ID:                  id,
		Exchange:            t.Exchange,
		RoutingKey:          t.RoutingKey,
		EventType:           t.EventType,
		TargetUserType:      t.TargetUserType,
		TargetUserField:     t.TargetUserField,
		Category:            t.Category,
		TitleTemplate:       t.TitleTemplate,
		BodyTemplate:        t.BodyTemplate,
		BodyFallback:        t.BodyFallback,
		DataType:            t.DataType,
		DataFields:          dataFieldsJSON,
		SchemaUrl:           t.SchemaURL,
		ReferenceIDTemplate: t.ReferenceIDTemplate,
		ImageKeyTemplate:    t.ImageKeyTemplate,
		PayloadFilter:       payloadFilterJSON,
		Enabled:             t.Enabled,
		PushMode:            pushMode,
		UpdatedBy:           t.UpdatedBy,
	}, nil
}

func domainToUpdateParams(t *domain.NotificationTemplate) (gen.UpdateTemplateParams, error) {
	dataFieldsJSON, err := marshalNullableJSON(t.DataFields)
	if err != nil {
		return gen.UpdateTemplateParams{}, fmt.Errorf("marshal data_fields: %w", err)
	}
	payloadFilterJSON, err := marshalNullableJSON(t.PayloadFilter)
	if err != nil {
		return gen.UpdateTemplateParams{}, fmt.Errorf("marshal payload_filter: %w", err)
	}
	pushMode := t.PushMode
	if pushMode == "" {
		pushMode = "visible"
	}
	return gen.UpdateTemplateParams{
		ID:                  t.ID,
		Exchange:            t.Exchange,
		RoutingKey:          t.RoutingKey,
		EventType:           t.EventType,
		TargetUserType:      t.TargetUserType,
		TargetUserField:     t.TargetUserField,
		Category:            t.Category,
		TitleTemplate:       t.TitleTemplate,
		BodyTemplate:        t.BodyTemplate,
		BodyFallback:        t.BodyFallback,
		DataType:            t.DataType,
		DataFields:          dataFieldsJSON,
		SchemaUrl:           t.SchemaURL,
		ReferenceIDTemplate: t.ReferenceIDTemplate,
		ImageKeyTemplate:    t.ImageKeyTemplate,
		PayloadFilter:       payloadFilterJSON,
		Enabled:             t.Enabled,
		PushMode:            pushMode,
		UpdatedBy:           t.UpdatedBy,
	}, nil
}

// marshalNullableJSON marshal v 成 jsonb；nil/empty map 視為 SQL NULL（return nil []byte）。
func marshalNullableJSON(v interface{}) ([]byte, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	if string(b) == "null" {
		return nil, nil
	}
	return b, nil
}
