package billing

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/aiserve/gpuproxy/internal/config"
	"github.com/aiserve/gpuproxy/internal/database"
	"github.com/aiserve/gpuproxy/internal/models"
	"github.com/google/uuid"
)

type Provider string

const (
	ProviderStripe     Provider = "stripe"
	ProviderAfterDark  Provider = "afterdark"
	ProviderCrypto     Provider = "crypto"
)

type Service struct {
	db         *database.PostgresDB
	config     *config.BillingConfig
	httpClient *http.Client
}

func NewService(db *database.PostgresDB, cfg *config.BillingConfig) *Service {
	return &Service{
		db:         db,
		config:     cfg,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

func (s *Service) CreateTransaction(ctx context.Context, userID uuid.UUID, amount float64, currency string, provider Provider, paymentMethod string) (*models.BillingTransaction, error) {
	tx := &models.BillingTransaction{
		ID:              uuid.New(),
		UserID:          userID,
		Amount:          amount,
		Currency:        currency,
		Status:          "pending",
		PaymentMethod:   paymentMethod,
		PaymentProvider: string(provider),
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}

	query := `
		INSERT INTO billing_transactions (id, user_id, amount, currency, status, payment_method, payment_provider, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id
	`

	err := s.db.Pool.QueryRow(ctx, query, tx.ID, tx.UserID, tx.Amount, tx.Currency, tx.Status, tx.PaymentMethod, tx.PaymentProvider, tx.CreatedAt, tx.UpdatedAt).Scan(&tx.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to create transaction: %w", err)
	}

	return tx, nil
}

func (s *Service) ProcessPayment(ctx context.Context, transaction *models.BillingTransaction, payment *models.PaymentPreference) error {
	provider := Provider(transaction.PaymentProvider)

	switch provider {
	case ProviderStripe:
		return s.processStripePayment(ctx, transaction, payment)
	case ProviderAfterDark:
		return s.processAfterDarkPayment(ctx, transaction, payment)
	case ProviderCrypto:
		return s.processCryptoPayment(ctx, transaction, payment)
	default:
		return fmt.Errorf("unsupported payment provider: %s", provider)
	}
}

func (s *Service) processStripePayment(ctx context.Context, transaction *models.BillingTransaction, payment *models.PaymentPreference) error {
	if s.config.StripeSecretKey == "" {
		return fmt.Errorf("Stripe not configured")
	}

	payload := map[string]interface{}{
		"amount":   int(transaction.Amount * 100),
		"currency": transaction.Currency,
		"payment_method": map[string]string{
			"type": "card",
		},
	}

	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.stripe.com/v1/payment_intents", bytes.NewBuffer(body))
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+s.config.StripeSecretKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("Stripe API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Stripe API error: %s", string(body))
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}

	externalID, _ := result["id"].(string)
	return s.updateTransactionStatus(ctx, transaction.ID, "completed", externalID)
}

func (s *Service) processAfterDarkPayment(ctx context.Context, transaction *models.BillingTransaction, payment *models.PaymentPreference) error {
	if s.config.AfterDarkAPIKey == "" {
		return fmt.Errorf("AfterDark billing not configured")
	}

	payload := map[string]interface{}{
		"amount":         transaction.Amount,
		"currency":       transaction.Currency,
		"payment_method": payment,
	}

	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, "POST", s.config.AfterDarkAPIURL+"/checkout", bytes.NewBuffer(body))
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+s.config.AfterDarkAPIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("AfterDark API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("AfterDark API error: %s", string(body))
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}

	externalID, _ := result["transaction_id"].(string)
	return s.updateTransactionStatus(ctx, transaction.ID, "completed", externalID)
}

func (s *Service) processCryptoPayment(ctx context.Context, transaction *models.BillingTransaction, payment *models.PaymentPreference) error {
	if !s.config.CryptoEnabled {
		return fmt.Errorf("crypto payments not enabled")
	}

	return s.updateTransactionStatus(ctx, transaction.ID, "pending_crypto", fmt.Sprintf("%s:%s", payment.Network, payment.Wallet))
}

func (s *Service) updateTransactionStatus(ctx context.Context, txID uuid.UUID, status, externalID string) error {
	query := `
		UPDATE billing_transactions
		SET status = $1, external_id = $2, updated_at = $3
		WHERE id = $4
	`

	_, err := s.db.Pool.Exec(ctx, query, status, externalID, time.Now(), txID)
	return err
}

func (s *Service) GetTransactionsByUser(ctx context.Context, userID uuid.UUID) ([]models.BillingTransaction, error) {
	query := `
		SELECT id, user_id, amount, currency, status, payment_method, payment_provider, external_id, created_at, updated_at
		FROM billing_transactions
		WHERE user_id = $1
		ORDER BY created_at DESC
	`

	rows, err := s.db.Pool.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var transactions []models.BillingTransaction
	for rows.Next() {
		var tx models.BillingTransaction
		if err := rows.Scan(&tx.ID, &tx.UserID, &tx.Amount, &tx.Currency, &tx.Status, &tx.PaymentMethod, &tx.PaymentProvider, &tx.ExternalID, &tx.CreatedAt, &tx.UpdatedAt); err != nil {
			continue
		}
		transactions = append(transactions, tx)
	}

	return transactions, nil
}
