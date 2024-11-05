package keystore

import (
	"github.com/rocket-pool/node-manager-core/beacon"
	eth2types "github.com/wealdtech/go-eth2-types/v2"
)

const (
	DirectEIPVersion string = "EIP-2335"
)

// Validator keystore manager interface
type IKeystoreManager interface {
	// Get the path of the keystore directory managed by this manager
	GetKeystoreDir() string

	// Get all the validator pubkeys stored in the keystore
	GetAllPubkeys() ([]beacon.ValidatorPubkey, error)

	// Encrypt a validator key with the provided password
	EncryptValidatorKey(key *eth2types.BLSPrivateKey, derivationPath string, password string) (beacon.ValidatorKeystore, error)

	// Store a validator keystore on disk
	StoreValidatorKeystore(keystore beacon.ValidatorKeystore, password string) error

	// Store a validator key on disk
	StoreValidatorKey(key *eth2types.BLSPrivateKey, derivationPath string) error

	// Load a validator key from disk corresponding to the provided pubkey
	LoadValidatorKey(pubkey beacon.ValidatorPubkey) (*eth2types.BLSPrivateKey, error)

	// Removes the validator key from disk. If the key didn't exist, returns nil.
	RemoveValidatorKey(pubkey beacon.ValidatorPubkey) error
}
