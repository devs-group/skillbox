package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// Approval policy constants.
const (
	ApprovalPolicyAuto   = "auto"   // auto-approve clean, review flagged
	ApprovalPolicyAlways = "always" // always require review
	ApprovalPolicyNone   = "none"   // auto-approve everything (scanner still runs for logging)
)

// ScannerConfig holds per-tenant scanner configuration.
type ScannerConfig struct {
	TenantID       string    `json:"tenant_id"`
	ApprovalPolicy string    `json:"approval_policy"`
	Tier1Enabled   bool      `json:"tier1_enabled"`
	Tier2Enabled   bool      `json:"tier2_enabled"`
	Tier3Enabled   bool      `json:"tier3_enabled"`
	Tier3APIKey    *string   `json:"tier3_api_key,omitempty"`
	Tier3Model     string    `json:"tier3_model"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// GetScannerConfig retrieves the scanner configuration for a tenant.
// If no configuration exists, it returns sensible defaults.
func (s *Store) GetScannerConfig(ctx context.Context, tenantID string) (*ScannerConfig, error) {
	cfg := &ScannerConfig{}
	err := s.conn().QueryRowContext(ctx, `
		SELECT tenant_id, approval_policy, tier1_enabled, tier2_enabled,
		       tier3_enabled, tier3_api_key, tier3_model, updated_at
		FROM sandbox.scanner_config
		WHERE tenant_id = $1
	`, tenantID).Scan(
		&cfg.TenantID, &cfg.ApprovalPolicy,
		&cfg.Tier1Enabled, &cfg.Tier2Enabled,
		&cfg.Tier3Enabled, &cfg.Tier3APIKey,
		&cfg.Tier3Model, &cfg.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		// Return defaults when no config exists for this tenant.
		return &ScannerConfig{
			TenantID:       tenantID,
			ApprovalPolicy: ApprovalPolicyAuto,
			Tier1Enabled:   true,
			Tier2Enabled:   true,
			Tier3Enabled:   false,
			Tier3Model:     "claude-sonnet-4-5-20250514",
		}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get scanner config: %w", err)
	}
	return cfg, nil
}

// UpsertScannerConfig creates or updates the scanner configuration for a tenant.
func (s *Store) UpsertScannerConfig(ctx context.Context, cfg *ScannerConfig) error {
	_, err := s.conn().ExecContext(ctx, `
		INSERT INTO sandbox.scanner_config
			(tenant_id, approval_policy, tier1_enabled, tier2_enabled,
			 tier3_enabled, tier3_api_key, tier3_model, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, now())
		ON CONFLICT (tenant_id) DO UPDATE SET
			approval_policy = EXCLUDED.approval_policy,
			tier1_enabled = EXCLUDED.tier1_enabled,
			tier2_enabled = EXCLUDED.tier2_enabled,
			tier3_enabled = EXCLUDED.tier3_enabled,
			tier3_api_key = EXCLUDED.tier3_api_key,
			tier3_model = EXCLUDED.tier3_model,
			updated_at = now()
	`, cfg.TenantID, cfg.ApprovalPolicy,
		cfg.Tier1Enabled, cfg.Tier2Enabled,
		cfg.Tier3Enabled, cfg.Tier3APIKey, cfg.Tier3Model)
	if err != nil {
		return fmt.Errorf("upsert scanner config: %w", err)
	}
	return nil
}
