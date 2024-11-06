package keystore

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/goccy/go-json"
	"github.com/google/uuid"
	"github.com/rocket-pool/node-manager-core/beacon"
	"github.com/rocket-pool/node-manager-core/utils"
	eth2types "github.com/wealdtech/go-eth2-types/v2"
	eth2ks "github.com/wealdtech/go-eth2-wallet-encryptor-keystorev4"
)

// Prysm's keystore format
type PrysmKeystore struct {
	Crypto  map[string]any `json:"crypto"`
	Name    string         `json:"name,omitempty"` // Technically not part of the spec but Prysm needs it
	Version uint           `json:"version"`
	UUID    uuid.UUID      `json:"uuid"`
	Path    string         `json:"path"`
	Pubkey  string         `json:"pubkey"` // This has to support being blank for backwards compatibility
}

// Prysm keystore manager
type PrysmKeystoreManager struct {
	as                       *prysmAccountStore
	encryptor                *eth2ks.Encryptor
	keystoreDir              string
	walletDir                string
	accountsDir              string
	keystoreFileName         string
	configFileName           string
	keystorePasswordFileName string
}

type prysmAccountStore struct {
	PrivateKeys [][]byte `json:"private_keys"`
	PublicKeys  [][]byte `json:"public_keys"`
}

// Prysm direct wallet config
type prysmWalletConfig struct {
	DirectEIPVersion string `json:"direct_eip_version"`
}

// Create new prysm keystore manager
func NewPrysmKeystoreManager(keystorePath string) *PrysmKeystoreManager {
	return &PrysmKeystoreManager{
		encryptor:                eth2ks.New(),
		keystoreDir:              filepath.Join(keystorePath, "prysm-non-hd"),
		walletDir:                "direct",
		accountsDir:              "accounts",
		keystoreFileName:         "all-accounts.keystore.json",
		configFileName:           "keymanageropts.json",
		keystorePasswordFileName: "secret",
	}
}

// Get the keystore directory
func (ks *PrysmKeystoreManager) GetKeystoreDir() string {
	return ks.keystoreDir
}

// Get all the validator pubkeys stored in the keystore
func (ks *PrysmKeystoreManager) GetAllPubkeys() ([]beacon.ValidatorPubkey, error) {
	// Initialize the account store
	if err := ks.initialize(); err != nil {
		return nil, err
	}

	// Get all the pubkeys
	var pubkeys []beacon.ValidatorPubkey
	for _, pubkey := range ks.as.PublicKeys {
		pubkey := beacon.ValidatorPubkey(pubkey)
		pubkeys = append(pubkeys, pubkey)
	}
	return pubkeys, nil
}

// Store a validator keystore on disk
func (ks *PrysmKeystoreManager) StoreValidatorKeystore(keystore beacon.ValidatorKeystore, password string) error {
	// Initialize the account store
	if err := ks.initialize(); err != nil {
		return err
	}

	// Cancel if validator key already exists in account store
	for ki := 0; ki < len(ks.as.PrivateKeys); ki++ {
		if bytes.Equal(keystore.Pubkey[:], ks.as.PublicKeys[ki]) {
			return nil
		}
	}

	// Add validator key to account store
	privateKeyBytes, err := ks.encryptor.Decrypt(keystore.Crypto, password)
	if err != nil {
		return fmt.Errorf("error decrypting validator key: %w", err)
	}
	key, err := eth2types.BLSPrivateKeyFromBytes(privateKeyBytes)
	if err != nil {
		return fmt.Errorf("error recreating private key for validator %s: %w", keystore.Pubkey.HexWithPrefix(), err)
	}

	return ks.StoreValidatorKey(key, keystore.Path)
}

// Store a validator key
func (ks *PrysmKeystoreManager) StoreValidatorKey(key *eth2types.BLSPrivateKey, derivationPath string) error {
	// Initialize the account store
	if err := ks.initialize(); err != nil {
		return err
	}

	// Cancel if validator key already exists in account store
	for ki := 0; ki < len(ks.as.PrivateKeys); ki++ {
		if bytes.Equal(key.Marshal(), ks.as.PrivateKeys[ki]) || bytes.Equal(key.PublicKey().Marshal(), ks.as.PublicKeys[ki]) {
			return nil
		}
	}

	// Add validator key to account store
	ks.as.PrivateKeys = append(ks.as.PrivateKeys, key.Marshal())
	ks.as.PublicKeys = append(ks.as.PublicKeys, key.PublicKey().Marshal())
	return ks.saveKeystore()
}

// Load a private key
func (ks *PrysmKeystoreManager) LoadValidatorKey(pubkey beacon.ValidatorPubkey) (*eth2types.BLSPrivateKey, error) {
	// Initialize the account store
	err := ks.initialize()
	if err != nil {
		return nil, err
	}

	// Find the validator key in the account store
	for ki := 0; ki < len(ks.as.PrivateKeys); ki++ {
		if bytes.Equal(pubkey[:], ks.as.PublicKeys[ki]) {
			decryptedKey := ks.as.PrivateKeys[ki]
			privateKey, err := eth2types.BLSPrivateKeyFromBytes(decryptedKey)
			if err != nil {
				return nil, fmt.Errorf("error recreating private key for validator %s: %w", pubkey.HexWithPrefix(), err)
			}

			// Verify the private key matches the public key
			reconstructedPubkey := beacon.ValidatorPubkey(privateKey.PublicKey().Marshal())
			if reconstructedPubkey != pubkey {
				return nil, fmt.Errorf("prysm's keystore has a key that claims to be for validator %s but it's for validator %s", pubkey.HexWithPrefix(), reconstructedPubkey.HexWithPrefix())
			}

			return privateKey, nil
		}
	}

	// Return nothing if the private key wasn't found
	return nil, nil
}

// Removes the validator key from disk. If the key didn't exist, returns nil.
func (ks *PrysmKeystoreManager) RemoveValidatorKey(pubkey beacon.ValidatorPubkey) error {
	// Initialize the account store
	err := ks.initialize()
	if err != nil {
		return err
	}

	// Remove the key
	newAs := &prysmAccountStore{}
	for ki := 0; ki < len(ks.as.PrivateKeys); ki++ {
		if bytes.Equal(pubkey[:], ks.as.PublicKeys[ki]) {
			continue
		}
		newAs.PrivateKeys = append(newAs.PrivateKeys, ks.as.PrivateKeys[ki])
		newAs.PublicKeys = append(newAs.PublicKeys, ks.as.PublicKeys[ki])
	}

	// Save the keystore
	ks.as = newAs
	return ks.saveKeystore()
}

// Initialize the account store
func (ks *PrysmKeystoreManager) initialize() error {
	// Cancel if already initialized
	if ks.as != nil {
		return nil
	}

	// Create the random keystore password if it doesn't exist
	var password string
	passwordFilePath := ks.getPasswordFilePath()
	_, err := os.Stat(passwordFilePath)
	if os.IsNotExist(err) {
		// Create a new password
		password, err = utils.GenerateRandomPassword()
		if err != nil {
			return fmt.Errorf("error generating random password: %w", err)
		}

		// Encode it
		passwordBytes := []byte(password)

		// Write it
		err := os.MkdirAll(filepath.Dir(passwordFilePath), DirMode)
		if err != nil {
			return fmt.Errorf("error creating account password directory: %w", err)
		}
		err = os.WriteFile(passwordFilePath, passwordBytes, FileMode)
		if err != nil {
			return fmt.Errorf("error writing account password file: %w", err)
		}
	}

	// Get the random keystore password
	passwordBytes, err := os.ReadFile(passwordFilePath)
	if err != nil {
		return fmt.Errorf("error opening account password file: %w", err)
	}
	password = string(passwordBytes)

	// Read keystore file; initialize empty account store if it doesn't exist
	keystorePath := ks.getKeystoreFilePath()
	ksBytes, err := os.ReadFile(keystorePath)
	if err != nil {
		ks.as = &prysmAccountStore{}
		return nil
	}

	// Decode keystore
	var keystore PrysmKeystore
	if err = json.Unmarshal(ksBytes, &keystore); err != nil {
		return fmt.Errorf("error decoding validator keystore: %w", err)
	}

	// Decrypt account store
	asBytes, err := ks.encryptor.Decrypt(keystore.Crypto, password)
	if err != nil {
		return fmt.Errorf("error decrypting validator account store: %w", err)
	}

	// Decode account store
	as := &prysmAccountStore{}
	if err = json.Unmarshal(asBytes, as); err != nil {
		return fmt.Errorf("error decoding validator account store: %w", err)
	}
	if len(as.PrivateKeys) != len(as.PublicKeys) {
		return errors.New("validator account store private and public key counts do not match")
	}

	// Set account store & return
	ks.as = as
	return nil
}

// Save the keystore to disk
func (ks *PrysmKeystoreManager) saveKeystore() error {
	// Encode account store
	asBytes, err := json.Marshal(ks.as)
	if err != nil {
		return fmt.Errorf("error encoding validator account store: %w", err)
	}

	// Get the keystore account password
	passwordFilePath := ks.getPasswordFilePath()
	passwordBytes, err := os.ReadFile(passwordFilePath)
	if err != nil {
		return fmt.Errorf("error reading account password file: %w", err)
	}
	password := string(passwordBytes)

	// Encrypt account store
	asEncrypted, err := ks.encryptor.Encrypt(asBytes, password)
	if err != nil {
		return fmt.Errorf("error encrypting validator account store: %w", err)
	}

	// Create new keystore
	keystore := PrysmKeystore{
		Crypto:  asEncrypted,
		Name:    ks.encryptor.Name(),
		Version: ks.encryptor.Version(),
		UUID:    uuid.New(),
	}

	// Encode key store
	ksBytes, err := json.Marshal(keystore)
	if err != nil {
		return fmt.Errorf("error encoding validator keystore: %w", err)
	}

	// Get file paths
	keystoreFilePath := ks.getKeystoreFilePath()
	configFilePath := ks.getConfigFilePath()

	// Create keystore dir
	if err := os.MkdirAll(filepath.Dir(keystoreFilePath), DirMode); err != nil {
		return fmt.Errorf("error creating keystore folder: %w", err)
	}

	// Write keystore to disk
	if err := os.WriteFile(keystoreFilePath, ksBytes, FileMode); err != nil {
		return fmt.Errorf("error writing keystore to disk: %w", err)
	}

	// Return if wallet config file exists
	if _, err := os.Stat(configFilePath); !os.IsNotExist(err) {
		return nil
	}

	// Create & encode wallet config
	configBytes, err := json.Marshal(prysmWalletConfig{
		DirectEIPVersion: DirectEIPVersion,
	})
	if err != nil {
		return fmt.Errorf("error encoding wallet config: %w", err)
	}

	// Write wallet config to disk
	if err := os.WriteFile(configFilePath, configBytes, FileMode); err != nil {
		return fmt.Errorf("error writing wallet config to disk: %w", err)
	}
	return nil
}

// Get the password file path for the keystore
func (ks *PrysmKeystoreManager) getPasswordFilePath() string {
	return filepath.Join(ks.keystoreDir, ks.walletDir, ks.accountsDir, ks.keystorePasswordFileName)
}

// Get the keystore file path
func (ks *PrysmKeystoreManager) getKeystoreFilePath() string {
	return filepath.Join(ks.keystoreDir, ks.walletDir, ks.accountsDir, ks.keystoreFileName)
}

// Get the config file path for the keystore
func (ks *PrysmKeystoreManager) getConfigFilePath() string {
	return filepath.Join(ks.keystoreDir, ks.walletDir, ks.configFileName)
}
