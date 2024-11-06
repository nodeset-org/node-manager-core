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

// Information about a validator key stored in the manager
type StoredValidatorKeyInfo struct {
	// The validator pubkey
	Pubkey beacon.ValidatorPubkey

	// Whether the key is stored on disk in any of the keystores
	IsStoredOnDisk bool

	// Whether the key is currently loaded in the Validator Client
	IsLoadedInValidatorClient bool
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

// Gets all the validator pubkeys stored in all of the keystores on disk, and the keys stored on the Validator Client.
func (m *ValidatorManager) GetAllKeys(ctx context.Context, logger *slog.Logger) (map[beacon.ValidatorPubkey]StoredValidatorKeyInfo, error) {
	m.lock.Lock()
	defer m.lock.Unlock()
	keys := map[beacon.ValidatorPubkey]StoredValidatorKeyInfo{}

	// Get the keys stored on disk
	for client, mgr := range m.keystoreManagers {
		pubkeys, err := mgr.GetAllPubkeys()
		if err != nil {
			return nil, fmt.Errorf("error getting validator pubkeys from keystore for %s: %w", string(client), err)
		}
		for _, pubkey := range pubkeys {
			key, exists := keys[pubkey]
			if !exists {
				key = StoredValidatorKeyInfo{Pubkey: pubkey}
			}
			key.IsStoredOnDisk = true
			keys[pubkey] = key
		}
	}

	// Get the keys stored in the Validator Client
	data, err := m.keyMgr.GetLoadedKeys(ctx, logger)
	if err != nil {
		return nil, fmt.Errorf("error getting loaded validator keys from key manager: %w", err)
	}
	for _, d := range data {
		pubkey := d.Pubkey
		key, exists := keys[pubkey]
		if !exists {
			key = StoredValidatorKeyInfo{Pubkey: pubkey}
		}
		key.IsLoadedInValidatorClient = true
		keys[pubkey] = key
	}

	return keys, nil
}

// Stores a validator key into all of the manager's client keystores on disk, but does not upload to the Validator Client
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

// Deletes a validator key from the manager's client keystores and the Validator Client's key manager
func (m *ValidatorManager) DeleteKey(ctx context.Context, logger *slog.Logger, pubkey beacon.ValidatorPubkey) error {
	m.lock.Lock()
	defer m.lock.Unlock()

	// Try deleting the key from all of the keystores, caching errors but not breaking on them
	errors := []string{}
	for _, mgr := range m.keystoreManagers {
		err := mgr.RemoveValidatorKey(pubkey)
		if err != nil {
			errors = append(errors, err.Error())
		}
	}
	if len(errors) > 0 {
		// If there were errors, return them
		return fmt.Errorf("encountered the following errors while trying to delete the key for validator %s:\n%s", pubkey.Hex(), strings.Join(errors, "\n"))
	}

	// Delete the key from the key manager
	data, err := m.keyMgr.DeleteKeys(ctx, logger, []beacon.ValidatorPubkey{pubkey})
	if err != nil {
		return fmt.Errorf("error deleting validator key from key manager: %w", err)
	}
	for _, d := range data {
		switch d.Status {
		case keymanager.DeleteKeystoreStatus_Error:
			return fmt.Errorf("deleting validator key from key manager failed: %s", d.Message)
		default:
			// Ignore everything else
			continue
		}
	}
	return nil
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
