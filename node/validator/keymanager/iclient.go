package keymanager

import (
	"context"
	"errors"
	"log/slog"

	"github.com/ethereum/go-ethereum/common"
	"github.com/rocket-pool/node-manager-core/beacon"
)

var (
	// Error returned when a method is not supported
	ErrNotSupported error = errors.New("not supported")
)

// IKeyManagerClient is an interface for a key manager client
type IKeyManagerClient interface {
	// Get the path to the JWT file
	GetJwtFilePath() string

	// Get the JWT token as a hex-encoded string (no 0x prefix)
	GetJwtToken() string

	// Get the list of pubkeys that are loaded in the key manager
	GetLoadedKeys(ctx context.Context, logger *slog.Logger) ([]GetKeystoreData, error)

	// Import a list of keys into the key manager
	ImportKeys(ctx context.Context, logger *slog.Logger, keystores []beacon.ValidatorKeystore, passwords []string, slashingProtection *beacon.SlashingProtectionData) ([]ImportKeystoreData, error)

	// Delete keys from the key manager
	DeleteKeys(ctx context.Context, logger *slog.Logger, pubkeys []beacon.ValidatorPubkey) ([]DeleteKeystoreData, error)

	// Get the graffiti for a validator
	GetGraffitiForValidator(ctx context.Context, logger *slog.Logger, pubkey beacon.ValidatorPubkey) (GetGraffitiData, error)

	// Set the graffiti for a validator
	SetGraffitiForValidator(ctx context.Context, logger *slog.Logger, pubkey beacon.ValidatorPubkey, graffiti string) error

	// Get the fee recipient for a validator
	GetFeeRecipientForValidator(ctx context.Context, logger *slog.Logger, pubkey beacon.ValidatorPubkey) (GetFeeRecipientData, error)

	// Set the fee recipient for a validator
	SetFeeRecipientForValidator(ctx context.Context, logger *slog.Logger, pubkey beacon.ValidatorPubkey, feeRecipient common.Address) error
}
