package cmd

import (
	"bufio"
	"context"
	"github.com/containers/image/v5/copy"
	"github.com/containers/image/v5/docker"
	"github.com/containers/image/v5/docker/reference"
	"github.com/containers/image/v5/manifest"
	"github.com/containers/image/v5/oci/layout"
	"github.com/containers/image/v5/signature"
	"github.com/moio/booster/gzip"
	"github.com/moio/booster/wharf"
	"github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"os"
	"path"
	"path/filepath"
)

// Diff downloads two sets of images in tempDir and then
// creates a wharf diff between them in patchPath
func Diff(oldList string, newList string, tempDir string, patchPath string) error {
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

	uncompressedOldFiles := gzip.Decompress(oldFiles, imageTempDir)

	log.Info().Str("list", newList).Msg("Processing")
	newFiles, err := downloadAll(newImages, imageTempDir)
	if err != nil {
		return errors.Wrapf(err, "Error while computing diff")
	}
	uncompressedNewFiles := gzip.Decompress(newFiles, imageTempDir)

	allUncompressedFiles := wharf.MergeAcceptLists(uncompressedOldFiles, uncompressedNewFiles)
	// add compulsory files from the OCI format
	allUncompressedFiles["oci-layout"] = true
	allUncompressedFiles["index.json"] = true

	log.Info().Str("name", patchPath).Msg("Creating patch")

	f, err := os.Create(patchPath)
	if err != nil {
		return errors.Wrap(err, "Error while opening patch file")
	}
	oldFilter := wharf.NewAcceptListFilter(imageTempDir, uncompressedOldFiles)
	newFilter := wharf.NewAcceptListFilter(imageTempDir, allUncompressedFiles)
	err = wharf.CreatePatch(imageTempDir, oldFilter.Filter, imageTempDir, newFilter.Filter, wharf.PreventClosing(f))
	if err != nil {
		log.Err(err).Msg("Error during patch creation")
	}
	if err := f.Close(); err != nil {
		return errors.Wrap(err, "Error while closing patch file")
	}

	return nil
}

// readLines reads a file and returns a list of strings, one per line
func readLines(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, errors.Wrapf(err, "Error while reading file %v", path)
	}

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	if err := file.Close(); err != nil {
		return nil, errors.Wrapf(err, "Error while closing file %v", path)
	}

	return lines, nil
}

// downloadAll downloads all images into dir
func downloadAll(images []string, dir string) (map[string]bool, error) {
	fileSet := map[string]bool{}
	for _, image := range images {
		files, err := download(image, dir)
		if err != nil {
			return nil, err
		}
		for _, file := range files {
			rel, err := filepath.Rel(dir, file)
			if err != nil {
				return nil, errors.Wrapf(err, "error while downloading image %v", image)
			}
			fileSet[rel] = true
		}
	}
	return fileSet, nil
}

// download downloads an image into dir
// returns a map to files that have been downloaded
func download(image string, dir string) ([]string, error) {
	log.Info().Str("image", image).Msg("Downloading")

	policy, err := signature.DefaultPolicy(nil)
	if err != nil {
		return nil, errors.Wrapf(err, "Error creating default policy context while copying the image: %v", image)
	}
	policyContext, err := signature.NewPolicyContext(policy)
	if err != nil {
		return nil, errors.Wrapf(err, "Error creating default policy context while copying the image: %v", image)
	}

	// build reference to OCI directory export
	destRef, err := layout.NewReference(dir, image)
	if err != nil {
		return nil, errors.Wrapf(err, "Error parsing reference: %v", image)
	}

	// build reference to a container in a Docker registry
	named, err := reference.ParseNormalizedNamed(image)
	if err != nil {
		return nil, errors.Wrapf(err, "Error parsing reference: %v", image)
	}
	srcRef, err := docker.NewReference(reference.TagNameOnly(named))
	if err != nil {
		return nil, errors.Wrapf(err, "Error parsing reference: %v", image)
	}

	manifestBytes, err := copy.Image(context.Background(), policyContext, destRef, srcRef, &copy.Options{OptimizeDestinationImageAlreadyExists: true})
	if err != nil {
		return nil, errors.Wrapf(err, "Error copying image: %v", image)
	}

	d, err := manifest.Digest(manifestBytes)
	if err != nil {
		return nil, errors.Wrapf(err, "Error computing digest for image: %v", image)
	}
	m, err := manifest.FromBlob(manifestBytes, manifest.GuessMIMEType(manifestBytes))
	if err != nil {
		return nil, errors.Wrapf(err, "Error parsing manifest of image: %v", image)
	}

	return manifestFiles(m, d, dir), nil
}

// manifestFiles returns the set of files corresponding to an image manifest
// as per the "Open Container Image Layout Specification"
// see https://github.com/opencontainers/image-spec/blob/v1.0.1/image-layout.md#content
func manifestFiles(manifest manifest.Manifest, manifestDigest digest.Digest, basePath string) []string {
	result := []string{}
	// manifest file
	manifestPath := path.Join(basePath, "blobs", manifestDigest.Algorithm().String(), manifestDigest.Encoded())
	result = append(result, manifestPath)

	// configinfo file
	configDigest := manifest.ConfigInfo().Digest
	configPath := path.Join(basePath, "blobs", configDigest.Algorithm().String(), configDigest.Encoded())
	result = append(result, configPath)

	// layer files
	for _, layerInfo := range manifest.LayerInfos() {
		layerDigest := layerInfo.Digest
		layerPath := path.Join(basePath, "blobs", layerDigest.Algorithm().String(), layerDigest.Encoded())
		result = append(result, layerPath)
	}
	return result
}
