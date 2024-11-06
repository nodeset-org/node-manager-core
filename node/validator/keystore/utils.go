package keystore

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/rocket-pool/node-manager-core/beacon"
	eth2types "github.com/wealdtech/go-eth2-types/v2"
	eth2ks "github.com/wealdtech/go-eth2-wallet-encryptor-keystorev4"
)

// Create an encrypted EIP-2335 keystore from a BLS private key.
func EncryptValidatorKey(key *eth2types.BLSPrivateKey, derivationPath string, password string) (beacon.ValidatorKeystore, error) {
	encryptor := eth2ks.New()

	// Get validator pubkey
	pubkey := beacon.ValidatorPubkey(key.PublicKey().Marshal())

	// Encrypt key
	encryptedKey, err := encryptor.Encrypt(key.Marshal(), password)
	if err != nil {
		return beacon.ValidatorKeystore{}, fmt.Errorf("error encrypting validator key: %w", err)
	}

	// Create key store
	keyStore := beacon.ValidatorKeystore{
		Crypto:  encryptedKey,
		Version: encryptor.Version(),
		UUID:    uuid.New(),
		Path:    derivationPath,
		Pubkey:  pubkey,
	}

	return keyStore, nil
}
