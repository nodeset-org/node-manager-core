package validator

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"github.com/rocket-pool/node-manager-core/beacon"
	"github.com/rocket-pool/node-manager-core/config"
	"github.com/rocket-pool/node-manager-core/node/validator/keymanager"
	"github.com/rocket-pool/node-manager-core/node/validator/keystore"
	"github.com/rocket-pool/node-manager-core/utils"
	types "github.com/wealdtech/go-eth2-types/v2"
)

type ValidatorManager struct {
	keystoreManagers map[config.BeaconNode]keystore.IKeystoreManager
	keyMgr           keymanager.IKeyManagerClient
	lock             *sync.Mutex
}

// Creates a new manager for validator keys and keystores.
// If you have multiple Validator Clients, you should create a new ValidatorManager for each one.
// validatorPath is the path to the base directory for all individual Validator Client resources, such as keystores and slashing databases.
// keyManager is the client for the Validator Client's key manager API.
// You will need to create it using the appropriate client for your Validator Client.
func NewValidatorManager(validatorPath string, keyManager keymanager.IKeyManagerClient) *ValidatorManager {
	mgr := &ValidatorManager{
		keystoreManagers: map[config.BeaconNode]keystore.IKeystoreManager{
			config.BeaconNode_Lighthouse: keystore.NewLighthouseKeystoreManager(validatorPath),
			config.BeaconNode_Lodestar:   keystore.NewLodestarKeystoreManager(validatorPath),
			config.BeaconNode_Nimbus:     keystore.NewNimbusKeystoreManager(validatorPath),
			config.BeaconNode_Prysm:      keystore.NewPrysmKeystoreManager(validatorPath),
			config.BeaconNode_Teku:       keystore.NewTekuKeystoreManager(validatorPath),
		},
		keyMgr: keyManager,
		lock:   &sync.Mutex{},
	}
	return mgr
}

// Stores a validator key into all of the manager's client keystores on disk, but does not upload to the
func (m *ValidatorManager) StoreKey(key *types.BLSPrivateKey, derivationPath string) error {
	m.lock.Lock()
	defer m.lock.Unlock()
	return m.storeKeyImpl(key, derivationPath)
}

// Stores a validator key into all of the manager's client keystores and uploads it to the VC's key manager
func (m *ValidatorManager) StoreAndUploadKey(ctx context.Context, logger *slog.Logger, key *types.BLSPrivateKey, derivationPath string, slashingProtection *beacon.SlashingProtectionData) error {
	m.lock.Lock()
	defer m.lock.Unlock()

	// Store the key in all of the keystores
	err := m.storeKeyImpl(key, derivationPath)
	if err != nil {
		return fmt.Errorf("error storing validator key in keystores: %w", err)
	}

	// Generate a keystore and password
	password, err := utils.GenerateRandomPassword()
	if err != nil {
		return fmt.Errorf("error generating random password: %w", err)
	}
	ks, err := keystore.EncryptValidatorKey(key, derivationPath, password)
	if err != nil {
		return fmt.Errorf("error encrypting validator key: %w", err)
	}

	// Upload the key to the key manager
	data, err := m.keyMgr.ImportKeys(ctx, logger, []beacon.ValidatorKeystore{ks}, []string{password}, slashingProtection)
	if err != nil {
		return fmt.Errorf("error uploading validator key to key manager: %w", err)
	}
	for _, d := range data {
		switch d.Status {
		case keymanager.ImportKeystoreStatus_Error:
			return fmt.Errorf("uploading validator key to key manager failed: %s", d.Message)
		default:
			// Ignore duplicate keys, since this function doesn't check for duplicates
			continue
		}
	}

	return nil
}

// Loads a validator key from the manager's client keystores
func (m *ValidatorManager) LoadKey(pubkey beacon.ValidatorPubkey) (*types.BLSPrivateKey, error) {
	m.lock.Lock()
	defer m.lock.Unlock()

	errors := []string{}
	// Try loading the key from all of the keystores, caching errors but not breaking on them
	for _, mgr := range m.keystoreManagers {
		key, err := mgr.LoadValidatorKey(pubkey)
		if err != nil {
			errors = append(errors, err.Error())
		}
		if key != nil {
			return key, nil
		}
	}

	if len(errors) > 0 {
		// If there were errors, return them
		return nil, fmt.Errorf("encountered the following errors while trying to load the key for validator %s:\n%s", pubkey.Hex(), strings.Join(errors, "\n"))
	} else {
		// If there were no errors, the key just didn't exist
		return nil, fmt.Errorf("couldn't find the key for validator %s in any of the validator manager's keystores", pubkey.Hex())
	}
}

// Implementation for storing a key in the manager's client keystores on disk
func (m *ValidatorManager) storeKeyImpl(key *types.BLSPrivateKey, derivationPath string) error {
	// Store the key in all of the keystores
	for name, mgr := range m.keystoreManagers {
		err := mgr.StoreValidatorKey(key, derivationPath)
		if err != nil {
			pubkey := beacon.ValidatorPubkey(key.PublicKey().Marshal())
			return fmt.Errorf("error saving validator key %s (path %s) to the %s keystore: %w", pubkey.HexWithPrefix(), derivationPath, name, err)
		}
	}
	return nil
}
