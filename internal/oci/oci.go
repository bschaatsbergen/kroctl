package oci

import (
	"fmt"
	"strings"

	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
	"oras.land/oras-go/v2/registry/remote/credentials"
)

const (
	// ArtifactType identifies the OCI artifact as a kro RGD stack
	ArtifactType = "application/vnd.kro.rgd.stack.v1"
	// LayerMediaType identifies individual RGD YAML files
	LayerMediaType = "application/vnd.kro.rgd.content.v1.yaml"
)

// SetupRepository creates and configures a remote repository with authentication
// and plain HTTP support for localhost registries.
func SetupRepository(reference string) (*remote.Repository, error) {
	repo, err := remote.NewRepository(reference)
	if err != nil {
		return nil, fmt.Errorf("invalid reference %s: %w", reference, err)
	}

	// Enable plain HTTP for localhost and local registries
	// TODO: remove this hack when we have proper TLS support
	host := repo.Reference.Host()
	if strings.HasPrefix(host, "localhost:") ||
		strings.HasPrefix(host, "127.0.0.1:") ||
		strings.HasPrefix(host, "::1:") {
		repo.PlainHTTP = true
	}

	// Configure authentication
	// TODO: uses Docker credentials for now.. support more methods later
	credStore, err := credentials.NewStoreFromDocker(credentials.StoreOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to create credential store: %w", err)
	}
	repo.Client = &auth.Client{
		Credential: credentials.Credential(credStore),
	}

	return repo, nil
}
