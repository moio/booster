package cmd

import (
	"context"
	"github.com/containers/image/v5/copy"
	"github.com/containers/image/v5/docker"
	"github.com/containers/image/v5/docker/reference"
	"github.com/containers/image/v5/oci/layout"
	"github.com/containers/image/v5/signature"
	"github.com/containers/image/v5/types"
	"github.com/moio/booster/gzip"
	"github.com/moio/booster/wharf"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"path/filepath"
)

func Apply(oldList string, newList string, patchPath string, tempDir string, destination string) error {
	oldImages, err := readLines(oldList)
	if err != nil {
		return err
	}
	newImages, err := readLines(newList)
	if err != nil {
		return err
	}

	log.Info().Str("list", oldList).Msg("Processing")

	imageTempDir := filepath.Join(tempDir, "images")
	oldFiles, err := downloadAll(oldImages, imageTempDir)
	if err != nil {
		return errors.Wrapf(err, "Error while computing diff")
	}
	gzip.Decompress(oldFiles)

	log.Info().Str("patch", patchPath).Msg("Applying")

	patchTempDir := filepath.Join(tempDir, "patch")
	_, err = wharf.Apply(patchPath, imageTempDir, patchTempDir)
	if err != nil {
		return errors.Wrap(err, "Error while applying patch")
	}

	if err := gzip.RecompressAllIn(tempDir); err != nil {
		return errors.Wrap(err, "Error while recompressing files")
	}

	for _, image := range newImages {
		if err = upload(image, imageTempDir, destination); err != nil {
			return errors.Wrapf(err, "Error while uploading to destination")
		}
	}

	log.Info().Msg("All done!")

	return nil
}

func upload(image string, sourcePath string, destinationRegistry string) error {
	log.Info().Str("image", image).Msg("Uploading")

	policy, err := signature.DefaultPolicy(nil)
	if err != nil {
		return errors.Wrapf(err, "Error creating default policy context while copying the image: %v", image)
	}
	policyContext, err := signature.NewPolicyContext(policy)
	if err != nil {
		return errors.Wrapf(err, "Error creating default policy context while copying the image: %v", image)
	}

	named, err := reference.ParseNormalizedNamed(destinationRegistry + "/" + image)
	if err != nil {
		return errors.Wrapf(err, "Error parsing reference: %v", image)
	}
	destRef, err := docker.NewReference(reference.TagNameOnly(named))
	if err != nil {
		return errors.Wrapf(err, "Error parsing reference: %v", image)
	}

	srcRef, err := layout.NewReference(sourcePath, image)
	if err != nil {
		return errors.Wrapf(err, "Error parsing reference: %v", image)
	}

	_, err = copy.Image(context.Background(), policyContext, destRef, srcRef, &copy.Options{
		// HACK: allow http, should be passed explicitly via commandline switch
		DestinationCtx:                        &types.SystemContext{DockerInsecureSkipTLSVerify: types.NewOptionalBool(true)},
		OptimizeDestinationImageAlreadyExists: true,
	})
	if err != nil {
		return errors.Wrapf(err, "Error copying image: %v", image)
	}

	return nil
}
