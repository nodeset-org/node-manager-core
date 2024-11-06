package keymanager

import (
	"errors"

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
	GetLoadedKeys() ([]GetKeystoreData, error)

	// Import a list of keys into the key manager
	ImportKeys(keystores []beacon.ValidatorKeystore, passwords []string) ([]ImportKeystoreData, error)

	// Delete keys from the key manager
	DeleteKeys(pubkeys []beacon.ValidatorPubkey) ([]DeleteKeystoreData, error)

	// Get the graffiti for a validator
	GetGraffitiForValidator(pubkey beacon.ValidatorPubkey) (GetGraffitiData, error)

	// Set the graffiti for a validator
	SetGraffitiForValidator(pubkey beacon.ValidatorPubkey, graffiti string) error

	// Get the fee recipient for a validator
	GetFeeRecipientForValidator(pubkey beacon.ValidatorPubkey) (GetFeeRecipientData, error)

	// Set the fee recipient for a validator
	SetFeeRecipientForValidator(pubkey beacon.ValidatorPubkey, feeRecipient common.Address) error
}
