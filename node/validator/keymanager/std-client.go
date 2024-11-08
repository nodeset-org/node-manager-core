package keymanager

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"

	"github.com/ethereum/go-ethereum/common"
	"github.com/goccy/go-json"
	"github.com/rocket-pool/node-manager-core/beacon"
)

const (
	KeystoresRoute    string = "/eth/v1/keystores"
	GraffitiRoute     string = "/eth/v1/validator/%s/graffiti"
	FeeRecipientRoute string = "/eth/v1/validator/%s/feerecipient"
)

// Standard implementation for the VC key manager API
type StandardKeyManagerClient struct {
	vcEndpoint string
	client     *http.Client
	jwtFile    string
	jwtToken   string
}

// Options for the StandardKeyManagerClient
type StandardKeyManagerClientOptions struct {
	KeyLength int
}

// Creates a new StandardKeyManagerClient instance, generating the JWT key if it doesn't exist and loading it
func NewStandardKeyManagerClient(vcEndpoint string, jwtFile string, opts *StandardKeyManagerClientOptions) (*StandardKeyManagerClient, error) {
	// Default options
	if opts == nil {
		opts = &StandardKeyManagerClientOptions{
			KeyLength: DefaultKeyLength,
		}
	}

	// Generate or load the JWT key
	token, err := GenerateOrLoadAuthKey(jwtFile, opts.KeyLength)
	if err != nil {
		return nil, err
	}

	// Return the new client
	return &StandardKeyManagerClient{
		vcEndpoint: vcEndpoint,
		client:     &http.Client{},
		jwtFile:    jwtFile,
		jwtToken:   token,
	}, nil
}

// Get the path of the JWT file
func (c *StandardKeyManagerClient) GetJwtFilePath() string {
	return c.jwtFile
}

// Get the JWT token as a hex-encoded string (no 0x prefix)
func (c *StandardKeyManagerClient) GetJwtToken() string {
	return c.jwtToken
}

// Get the list of pubkeys that are loaded in the key manager
func (c *StandardKeyManagerClient) GetLoadedKeys(ctx context.Context, logger *slog.Logger) ([]GetKeystoreData, error) {
	// Submit the request
	code, response, err := submitRequest[[]GetKeystoreData](c, ctx, logger, http.MethodGet, nil, nil, KeystoresRoute)
	if err != nil {
		return nil, fmt.Errorf("error getting loaded keys: %w", err)
	}

	// Handle response based on return code
	switch code {
	case http.StatusOK:
		// Success
		return response.Data, nil

	default:
		return nil, fmt.Errorf("VC responded to request with code %d and message: %s", code, response.Message)
	}
}

// Import a list of keys into the key manager.
// Slashing protection is optional.
func (c *StandardKeyManagerClient) ImportKeys(ctx context.Context, logger *slog.Logger, keystores []beacon.ValidatorKeystore, passwords []string, slashingProtection *beacon.SlashingProtectionData) ([]ImportKeystoreData, error) {
	var bodyNoPasswords ImportKeysBody
	var body ImportKeysBody

	// Marshal the keystores
	keystoreStrings := make([]string, len(keystores))
	for i, keystore := range keystores {
		keystoreBytes, err := json.Marshal(keystore)
		if err != nil {
			return nil, fmt.Errorf("error marshalling keystore [%s]: %w", keystore.Pubkey.HexWithPrefix(), err)
		}
		keystoreStrings[i] = string(keystoreBytes)
	}
	body.Keystores = keystoreStrings
	bodyNoPasswords.Keystores = keystoreStrings

	body.Passwords = passwords
	redactedPasswords := make([]string, len(passwords))
	for i := range passwords {
		redactedPasswords[i] = "REDACTED"
	}
	bodyNoPasswords.Passwords = redactedPasswords

	// Marshal the slashing protection if provided
	if slashingProtection != nil {
		slashingProtectionBytes, err := json.Marshal(slashingProtection)
		if err != nil {
			return nil, fmt.Errorf("error marshalling slashing protection: %w", err)
		}
		body.SlashingProtection = string(slashingProtectionBytes)
		bodyNoPasswords.SlashingProtection = string(slashingProtectionBytes)
	}

	// Marshal the body
	bodyData, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("error marshalling exit data to JSON: %w", err)
	}
	safeDebugLog(logger, "Prepared import keys POST body",
		"body", bodyNoPasswords,
	)

	// Submit the request
	code, response, err := submitRequest[[]ImportKeystoreData](c, ctx, logger, http.MethodPost, bytes.NewBuffer(bodyData), nil, KeystoresRoute)
	if err != nil {
		return nil, fmt.Errorf("error importing keys: %w", err)
	}

	// Handle response based on return code
	switch code {
	case http.StatusOK:
		// Success
		return response.Data, nil

	default:
		return nil, fmt.Errorf("VC responded to request with code %d and message: %s", code, response.Message)
	}
}

// Delete keys from the key manager
func (c *StandardKeyManagerClient) DeleteKeys(ctx context.Context, logger *slog.Logger, pubkeys []beacon.ValidatorPubkey) ([]DeleteKeystoreData, *beacon.SlashingProtectionData, error) {
	var body DeleteKeysBody
	body.Pubkeys = pubkeys

	// Marshal the body
	bodyData, err := json.Marshal(body)
	if err != nil {
		return nil, nil, fmt.Errorf("error marshalling exit data to JSON: %w", err)
	}
	safeDebugLog(logger, "Prepared delete keys POST body",
		"body", body,
	)

	// Submit the request
	code, response, err := submitRequest[[]DeleteKeystoreData](c, ctx, logger, http.MethodDelete, bytes.NewBuffer(bodyData), nil, KeystoresRoute)
	if err != nil {
		return nil, nil, fmt.Errorf("error deleting keys: %w", err)
	}

	// Handle response based on return code
	switch code {
	case http.StatusOK:
		// Success
		return response.Data, &response.SlashingProtection, nil

	default:
		return nil, nil, fmt.Errorf("VC responded to request with code %d and message: %s", code, response.Message)
	}
}

// Get the graffiti for a validator
func (c *StandardKeyManagerClient) GetGraffitiForValidator(ctx context.Context, logger *slog.Logger, pubkey beacon.ValidatorPubkey) (GetGraffitiData, error) {
	// Submit the request
	route := fmt.Sprintf(GraffitiRoute, pubkey.HexWithPrefix())
	code, response, err := submitRequest[GetGraffitiData](c, ctx, logger, http.MethodGet, nil, nil, route)
	if err != nil {
		return GetGraffitiData{}, fmt.Errorf("error getting graffiti for validator [%s]: %w", pubkey.HexWithPrefix(), err)
	}

	// Handle response based on return code
	switch code {
	case http.StatusOK:
		// Success
		return response.Data, nil

	default:
		return GetGraffitiData{}, fmt.Errorf("VC responded to request with code %d and message: %s", code, response.Message)
	}
}

// Set the graffiti for a validator
func (c *StandardKeyManagerClient) SetGraffitiForValidator(ctx context.Context, logger *slog.Logger, pubkey beacon.ValidatorPubkey, graffiti string) error {
	var body SetGraffitiBody
	body.Graffiti = graffiti

	// Marshal the body
	bodyData, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("error marshalling graffiti to JSON: %w", err)
	}
	safeDebugLog(logger, "Prepared set graffiti POST body",
		"body", body,
	)

	// Submit the request
	route := fmt.Sprintf(GraffitiRoute, pubkey.HexWithPrefix())
	code, response, err := submitRequest[any](c, ctx, logger, http.MethodPost, bytes.NewBuffer(bodyData), nil, route)
	if err != nil {
		return fmt.Errorf("error setting graffiti for validator [%s]: %w", pubkey.HexWithPrefix(), err)
	}

	// Handle response based on return code
	switch code {
	case http.StatusAccepted:
		// Success
		return nil

	default:
		return fmt.Errorf("VC responded to request with code %d and message: %s", code, response.Message)
	}
}

// Get the fee recipient for a validator
func (c *StandardKeyManagerClient) GetFeeRecipientForValidator(ctx context.Context, logger *slog.Logger, pubkey beacon.ValidatorPubkey) (GetFeeRecipientData, error) {
	// Submit the request
	route := fmt.Sprintf(FeeRecipientRoute, pubkey.HexWithPrefix())
	code, response, err := submitRequest[GetFeeRecipientData](c, ctx, logger, http.MethodGet, nil, nil, route)
	if err != nil {
		return GetFeeRecipientData{}, fmt.Errorf("error getting fee recipient for validator [%s]: %w", pubkey.HexWithPrefix(), err)
	}

	// Handle response based on return code
	switch code {
	case http.StatusOK:
		// Success
		return response.Data, nil

	default:
		return GetFeeRecipientData{}, fmt.Errorf("VC responded to request with code %d and message: %s", code, response.Message)
	}
}

// Set the fee recipient for a validator
func (c *StandardKeyManagerClient) SetFeeRecipientForValidator(ctx context.Context, logger *slog.Logger, pubkey beacon.ValidatorPubkey, feeRecipient common.Address) error {
	var body SetFeeRecipientBody
	body.EthAddress = feeRecipient

	// Marshal the body
	bodyData, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("error marshalling fee recipient to JSON: %w", err)
	}
	safeDebugLog(logger, "Prepared set fee recipient POST body",
		"body", body,
	)

	// Submit the request
	route := fmt.Sprintf(FeeRecipientRoute, pubkey.HexWithPrefix())
	code, response, err := submitRequest[any](c, ctx, logger, http.MethodPost, bytes.NewBuffer(bodyData), nil, route)
	if err != nil {
		return fmt.Errorf("error setting fee recipient for validator [%s]: %w", pubkey.HexWithPrefix(), err)
	}

	// Handle response based on return code
	switch code {
	case http.StatusAccepted:
		// Success
		return nil

	default:
		return fmt.Errorf("VC responded to request with code %d and message: %s", code, response.Message)
	}
}

// Submit a request to the key manager API
func submitRequest[DataType any](c *StandardKeyManagerClient, ctx context.Context, logger *slog.Logger, method string, body io.Reader, queryParams map[string]string, route string) (int, KeyManagerResponse[DataType], error) {
	var response KeyManagerResponse[DataType]

	// Make the request
	path, err := url.JoinPath(c.vcEndpoint, route)
	if err != nil {
		return 0, response, fmt.Errorf("error joining path [%v]: %w", route, err)
	}
	request, err := http.NewRequestWithContext(ctx, method, path, body)
	if err != nil {
		return 0, response, fmt.Errorf("error generating request to [%s]: %w", path, err)
	}
	query := request.URL.Query()
	for name, value := range queryParams {
		query.Add(name, value)
	}
	request.URL.RawQuery = query.Encode()
	safeDebugLog(logger, "Submitting request to VC",
		"method", method,
		"path", path,
		"query", request.URL.RawQuery,
	)

	// Set the headers
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Authorization", "Bearer "+c.jwtToken)

	// Send the request
	resp, err := c.client.Do(request)
	if err != nil {
		return 0, response, fmt.Errorf("error sending GET keystores request: %w", err)
	}

	// Read the body
	defer resp.Body.Close()
	bytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, response, fmt.Errorf("VC responded to request with code %s but reading the response body failed: %w", resp.Status, err)
	}

	// Handle an empty response
	if len(bytes) == 0 {
		// Debug log
		safeDebugLog(logger, "Received response from NodeSet server",
			"status", resp.Status,
			"response", "<empty>",
		)
		return resp.StatusCode, response, nil
	}

	// Unmarshal the response
	err = json.Unmarshal(bytes, &response)
	if err != nil {
		return 0, response, fmt.Errorf("VC responded to request with code %s and unmarshalling the response failed: [%w]... original body: [%s]", resp.Status, err, string(bytes))
	}
	safeDebugLog(logger, "Received response from NodeSet server",
		"status", resp.Status,
		"response", response,
	)
	return resp.StatusCode, response, nil
}

// Logs a message at the debug level if the logger is not nil
func safeDebugLog(logger *slog.Logger, msg string, args ...any) {
	if logger == nil {
		return
	}
	logger.Debug(msg, args...)
}
