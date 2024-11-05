package keymanager

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/rocket-pool/node-manager-core/beacon"
)

// =====================
// === Request Types ===
// =====================

// Body for an import keys request
type ImportKeysBody struct {
	// The keys to import
	Keystores []string `json:"keystores"`

	// The passwords to decrypt the keys
	Passwords []string `json:"passwords"`

	// The slashing protection DB
	SlashingProtection string `json:"slashing_protection,omitempty"`
}

// Body for a delete keys request
type DeleteKeysBody struct {
	// The keys to delete
	Pubkeys []beacon.ValidatorPubkey `json:"pubkeys"`
}

// Body for setting the graffiti of a validator
type SetGraffitiBody struct {
	// The new graffiti
	Graffiti string `json:"graffiti"`
}

// Body for setting the fee recipient of a validator
type SetFeeRecipientBody struct {
	// The new fee recipient
	EthAddress common.Address `json:"ethaddress"`
}

// ======================
// === Response Types ===
// ======================

// A response from the key manager
type KeyManagerResponse[DataType any] struct {
	// The response data
	Data DataType `json:"data"`

	// A message if the response is an error
	Message string `json:"message"`
}

// Data returned when a keystore loaded in the key manager
type GetKeystoreData struct {
	// The pubkey
	Pubkey beacon.ValidatorPubkey `json:"validating_pubkey"`

	// The derivation path
	DerivationPath string `json:"derivation_path"`

	// Read-only status
	ReadOnly bool `json:"readonly"`
}

// Status of a keystore import
type ImportKeystoreStatus string

const (
	// Keystore successfully decrypted and imported to keymanager permanent storage
	ImportKeystoreStatus_Imported ImportKeystoreStatus = "imported"

	// Keystore's pubkey is already known to the keymanager
	ImportKeystoreStatus_Duplicate ImportKeystoreStatus = "duplicate"

	// Any other status different to the above: decrypting error, I/O errors, etc.
	ImportKeystoreStatus_Error ImportKeystoreStatus = "error"
)

// Data returned when importing a keystore
type ImportKeystoreData struct {
	// Import status
	Status ImportKeystoreStatus `json:"status"`

	// Error message if status is error
	Message string `json:"message,omitempty"`
}

// Status of a keystore deletion
type DeleteKeystoreStatus string

const (
	// Key was active and removed
	DeleteKeystoreStatus_Deleted DeleteKeystoreStatus = "deleted"

	// Slashing protection data returned but key was not active
	DeleteKeystoreStatus_NotActive DeleteKeystoreStatus = "not_active"

	// Key was not found to be removed, and no slashing data can be returned
	DeleteKeystoreStatus_NotFound DeleteKeystoreStatus = "not_found"

	// Unexpected condition meant the key could not be removed (the key was actually found,
	// but we couldn't stop using it) - this would be a sign that making it active elsewhere
	// would almost certainly cause you headaches / slashing conditions etc.
	DeleteKeystoreStatus_Error DeleteKeystoreStatus = "error"
)

// Data returned when deleting a keystore
type DeleteKeystoreData struct {
	// Delete status
	Status DeleteKeystoreStatus `json:"status"`

	// Error message if status is error
	Message string `json:"message,omitempty"`
}

// Data for the graffiti of a validator
type GetGraffitiData struct {
	// The pubkey
	Pubkey beacon.ValidatorPubkey `json:"pubkey"`

	// The graffiti
	Graffiti string `json:"graffiti"`
}

// Data for the fee recipient of a validator
type GetFeeRecipientData struct {
	// The pubkey
	Pubkey beacon.ValidatorPubkey `json:"pubkey"`

	// The fee recipient
	EthAddress common.Address `json:"ethaddress"`
}
