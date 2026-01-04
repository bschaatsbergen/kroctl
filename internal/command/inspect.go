package command

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"text/tabwriter"
	"time"

	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/spf13/cobra"

	"github.com/bschaatsbergen/kroctl/internal/oci"
)

type InspectOptions struct {
	Reference string
}

func NewInspectCommand(cli *CLI) *cobra.Command {
	opts := InspectOptions{}

	cmd := &cobra.Command{
		Use:   "inspect <reference>",
		Short: "Inspect a ResourceGraphDefinition artifact in an OCI registry",
		Long: "Inspect a ResourceGraphDefinition artifact in an OCI registry.\n\n" +
			"Fetches the manifest from the registry and displays information\n" +
			"about the RGD stack, including all ResourceGraphDefinitions\n" +
			"contained in the artifact.\n\n" +
			"Examples:\n" +
			"  kroctl inspect localhost:5001/kro-stack-network:v1.0.0\n\n" +
			"  kroctl inspect ghcr.io/acme/kro-stack:latest\n",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Reference = args[0]
			return RunInspect(cmd.Context(), cli, &opts)
		},
	}

	return cmd
}

func RunInspect(ctx context.Context, cli *CLI, opts *InspectOptions) error {
	cli.Logger().Info("Inspecting artifact", "reference", opts.Reference)

	// Set up remote repository with authentication
	repo, err := oci.SetupRepository(opts.Reference)
	if err != nil {
		return err
	}

	if repo.PlainHTTP {
		cli.Logger().Debug("Using plain HTTP for local registry", "host", repo.Reference.Host())
	}

	// Fetch the manifest
	manifestDesc, rc, err := repo.FetchReference(ctx, opts.Reference)
	if err != nil {
		return fmt.Errorf("failed to fetch manifest: %w", err)
	}
	defer rc.Close()

	cli.Logger().Debug("Fetched manifest",
		"digest", manifestDesc.Digest.String(),
		"mediaType", manifestDesc.MediaType)

	manifestBytes, err := io.ReadAll(rc)
	if err != nil {
		return fmt.Errorf("failed to read manifest: %w", err)
	}

	var manifest v1.Manifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		return fmt.Errorf("failed to parse manifest: %w", err)
	}

	artifactName := repo.Reference.Repository
	if tag := repo.Reference.Reference; tag != "" {
		artifactName = artifactName + ":" + tag
	}
	registry := repo.Reference.Host()

	cli.Printf("Artifact:  %s\n", artifactName)
	cli.Printf("Registry:  %s\n", registry)
	cli.Printf("Digest:    %s\n", manifestDesc.Digest.String())

	// If the created annotation is present, display it
	if manifest.Config.Annotations != nil {
		if created, ok := manifest.Config.Annotations[v1.AnnotationCreated]; ok {
			if t, err := time.Parse(time.RFC3339, created); err == nil {
				cli.Printf("Created:   %s\n", t.Format(time.RFC3339))
			}
		}
	}

	if len(manifest.Layers) == 0 {
		cli.Printf("\nNo ResourceGraphDefinitions found in artifact\n")
		return nil
	}

	cli.Printf("\nResourceGraphDefinitions:\n")

	w := tabwriter.NewWriter(cli.Writer, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "Name\tDigest\n")

	for _, layer := range manifest.Layers {
		name := "unknown"
		if layer.Annotations != nil {
			if title, ok := layer.Annotations[v1.AnnotationTitle]; ok {
				name = title
			}
		}

		fmt.Fprintf(w, "%s\t%s\n", name, layer.Digest.String())
	}

	w.Flush()

	return nil
}
