package command

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/spf13/cobra"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content/file"

	"github.com/bschaatsbergen/kroctl/internal/oci"
)

type PushOptions struct {
	Filenames []string
	Reference string
}

func NewPushCommand(cli *CLI) *cobra.Command {
	opts := PushOptions{}

	cmd := &cobra.Command{
		Use:   "push <reference>",
		Short: "Push ResourceGraphDefinitions to an OCI registry",
		Long: "Push ResourceGraphDefinitions to an OCI registry.\n\n" +
			"Packages and pushes ResourceGraphDefinitions as an OCI artifact\n" +
			"to a specified registry. The RGDs must be valid YAML files.\n\n" +
			"Examples:\n" +
			"  kroctl push localhost:5001/kro-stack-network:v1.0.0 \\\n" +
			"    -f stack.yaml -f subnet.yaml -f vpc.yaml\n\n" +
			"  kroctl push ghcr.io/myorg/kro-stack:latest -f ./rgds/\n",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Reference = args[0]
			return RunPush(cmd.Context(), cli, &opts)
		},
	}

	cmd.Flags().StringSliceVarP(&opts.Filenames, "filenames", "f",
		[]string{}, "RGD files or directories to push (required)")
	_ = cmd.MarkFlagRequired("filenames")

	return cmd
}

func RunPush(ctx context.Context, cli *CLI, opts *PushOptions) error {
	if len(opts.Filenames) == 0 {
		return fmt.Errorf("no files specified, use -f to provide RGD files")
	}

	// Collect all YAML files
	var allFiles []string
	for _, filename := range opts.Filenames {
		info, err := os.Stat(filename)
		if err != nil {
			return fmt.Errorf("failed to access %s: %w", filename, err)
		}

		if info.IsDir() {
			err := filepath.Walk(filename, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				ext := filepath.Ext(path)
				if !info.IsDir() && (ext == ".yaml" || ext == ".yml") {
					allFiles = append(allFiles, path)
				}
				return nil
			})
			if err != nil {
				return fmt.Errorf("failed to walk directory %s: %w", filename, err)
			}
		} else {
			allFiles = append(allFiles, filename)
		}
	}

	if len(allFiles) == 0 {
		return fmt.Errorf("no YAML files found in specified paths")
	}

	cli.Logger().Info("Preparing to push RGD stack",
		"reference", opts.Reference,
		"files", len(allFiles))

	// Create a file store for the artifact
	store, err := file.New("")
	if err != nil {
		return fmt.Errorf("failed to create file store: %w", err)
	}
	defer store.Close()

	// Add files to the store
	layers := make([]v1.Descriptor, 0, len(allFiles))
	for _, file := range allFiles {
		desc, err := store.Add(ctx, filepath.Base(file), oci.LayerMediaType, file)
		if err != nil {
			return fmt.Errorf("failed to add %s to store: %w", file, err)
		}
		cli.Logger().Debug("Added file to artifact",
			"file", filepath.Base(file),
			"digest", desc.Digest.String())
		layers = append(layers, desc)
	}

	packOpts := oras.PackManifestOptions{
		Layers: layers,
	}
	manifestDesc, err := oras.PackManifest(ctx, store, oras.PackManifestVersion1_1, oci.ArtifactType, packOpts)
	if err != nil {
		return fmt.Errorf("failed to pack manifest: %w", err)
	}

	if err := store.Tag(ctx, manifestDesc, opts.Reference); err != nil {
		return fmt.Errorf("failed to tag manifest: %w", err)
	}

	// Set up remote repository with authentication
	repo, err := oci.SetupRepository(opts.Reference)
	if err != nil {
		return err
	}

	if repo.PlainHTTP {
		cli.Logger().Debug("Using plain HTTP for local registry", "host", repo.Reference.Host())
	}

	// Copy from file store to remote registry
	cli.Logger().Info("Pushing artifact to registry", "reference", opts.Reference)
	_, err = oras.Copy(ctx, store, opts.Reference, repo, opts.Reference, oras.DefaultCopyOptions)
	if err != nil {
		return fmt.Errorf("failed to push artifact: %w", err)
	}

	cli.Printf("Successfully pushed %d RGD file(s) to %s\n",
		len(allFiles), opts.Reference)
	cli.Printf("Digest: %s\n", manifestDesc.Digest.String())

	return nil
}
