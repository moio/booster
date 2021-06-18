package wharf

import (
	"context"
	"github.com/itchio/headway/state"
	"github.com/itchio/lake/pools/fspool"
	"github.com/itchio/lake/tlc"
	_ "github.com/itchio/wharf/compressors/cbrotli"
	"github.com/itchio/wharf/pwr"
	"github.com/pkg/errors"
	"io"
	"io/ioutil"
)

func CreatePatch(oldPath string, oldFilter tlc.FilterFunc, newPath string, target io.Writer) (err error) {
	// code adapted from the butler project, https://github.com/itchio/butler
	oldSignature := &pwr.SignatureInfo{}

	oldSignature.Container, err = tlc.WalkDir(oldPath, tlc.WalkOpts{Filter: oldFilter})
	if err != nil {
		return errors.Wrapf(err, "walking %v as directory", oldPath)
	}
	oldPool := fspool.New(oldSignature.Container, oldPath)

	oldSignature.Hashes, err = pwr.ComputeSignature(context.Background(), oldSignature.Container, oldPool, &state.Consumer{})
	if err != nil {
		return errors.Wrapf(err, "computing signature of %v", oldPath)
	}

	var newContainer *tlc.Container
	newContainer, err = tlc.WalkDir(newPath, tlc.WalkOpts{Filter: tlc.KeepAllFilter})
	if err != nil {
		return errors.Wrapf(err, "walking %v as directory", newPath)
	}
	newPool := fspool.New(newContainer, newPath)

	dctx := &pwr.DiffContext{
		SourceContainer: newContainer,
		Pool:            newPool,

		TargetContainer: oldSignature.Container,
		TargetSignature: oldSignature.Hashes,

		Consumer:    &state.Consumer{},
		Compression: &pwr.CompressionSettings{
			Algorithm: pwr.CompressionAlgorithm_BROTLI,
			Quality:   3, // standard for brotli
		},
	}

	err = dctx.WritePatch(context.Background(), target, ioutil.Discard)
	if err != nil {
		return errors.Wrap(err, "computing and writing patch")
	}

	return nil
}