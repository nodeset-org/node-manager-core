package keymanager

import "fmt"

// Key manager client specifically for Lighthouse
type LighthouseKeyManagerClient struct {
	*StandardKeyManagerClient
}

const (
	// Lighthouse JWT key path
	LighthouseJwtTokenPath = "%s/validators/api-token.txt"
)

// Create a new Lighthouse key manager client.
// Since Lighthouse currently hardcodes the location of the JWT token file, you must provide Lighthouse's base keystore directory instead.
func NewLighthouseKeyManagerClient(vcEndpoint string, keystoreDir string, opts *StandardKeyManagerClientOptions) (*LighthouseKeyManagerClient, error) {
	// Create the standard client
	jwtFile := fmt.Sprintf(LighthouseJwtTokenPath, keystoreDir)
	stdClient, err := NewStandardKeyManagerClient(vcEndpoint, jwtFile, opts)
	if err != nil {
		return nil, err
	}
	return &LighthouseKeyManagerClient{
		StandardKeyManagerClient: stdClient,
	}, nil
}
