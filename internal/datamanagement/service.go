// Package datamanagement provides the admin-only "clear transaction data"
// feature: exporting every transaction and audit trail table to JSON, and
// permanently wiping them in one irreversible operation.
package datamanagement

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"sbs-bsp-cctv/internal/audit"
)

// transactionTables lists every table considered operational/transactional
// data (as opposed to reference data like users, roles, or site locations),
// plus the audit trail itself. Order doesn't matter for TRUNCATE ... CASCADE
// since it resolves dependencies across the whole listed set at once.
var transactionTables = []string{
	"goods_receipts",
	"device_configurations",
	"firmware_configurations",
	"store_custody_logs",
	"shipments",
	"site_installations",
	"client_acceptances",
	"defect_reports",
	"delivery_docket_tracking_events",
	"delivery_docket_items",
	"delivery_dockets",
	"hardware_units",
	"audit_trails",
}

// WipeConfirmationPhrase must be sent by the caller to confirm a wipe,
// mirroring the phrase the admin is asked to type in the UI.
const WipeConfirmationPhrase = "DELETE ALL TRANSACTIONS"

type Service struct {
	pool  *pgxpool.Pool
	audit *audit.Service
}

func NewService(pool *pgxpool.Pool, auditSvc *audit.Service) *Service {
	return &Service{pool: pool, audit: auditSvc}
}

// Export returns the full contents of every transaction and audit trail
// table, keyed by table name, for admins to archive before a wipe.
func (s *Service) Export(ctx context.Context) (map[string]any, error) {
	out := make(map[string]any, len(transactionTables))
	for _, table := range transactionTables {
		rows, err := s.pool.Query(ctx, "SELECT * FROM "+table)
		if err != nil {
			return nil, err
		}
		records, err := scanRows(rows)
		rows.Close()
		if err != nil {
			return nil, err
		}
		out[table] = records
	}
	return out, nil
}

func scanRows(rows pgx.Rows) ([]map[string]any, error) {
	fields := rows.FieldDescriptions()
	records := []map[string]any{}
	for rows.Next() {
		values, err := rows.Values()
		if err != nil {
			return nil, err
		}
		record := make(map[string]any, len(fields))
		for i, f := range fields {
			record[f.Name] = values[i]
		}
		records = append(records, record)
	}
	return records, rows.Err()
}

// WipeAll permanently deletes every row from every transaction and audit
// trail table, in a single atomic statement. It is irreversible; callers
// should export the data first via Export. The final audit entry is written
// after the wipe commits, since audit_trails is one of the wiped tables —
// it becomes the sole surviving record that the wipe happened.
func (s *Service) WipeAll(ctx context.Context, actor uuid.UUID, ipAddress string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	query := "TRUNCATE TABLE " + strings.Join(transactionTables, ", ") + " RESTART IDENTITY CASCADE"
	if _, err := tx.Exec(ctx, query); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}

	return s.audit.Record(ctx, audit.Entry{
		EntityType:  "system",
		EntityID:    "transaction_data",
		Action:      "wipe_all",
		PerformedBy: &actor,
		IPAddress:   ipAddress,
		Notes:       "All transaction and audit trail records were permanently deleted by an admin.",
	})
}
