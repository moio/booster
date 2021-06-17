package wharf

import (
	"context"
	"github.com/itchio/headway/state"
	"github.com/itchio/lake/pools/fspool"
	"github.com/itchio/lake/tlc"
	"github.com/itchio/wharf/pwr"
	"github.com/pkg/errors"
	"io"
	"io/ioutil"

	_ "github.com/itchio/wharf/compressors/cbrotli"
)

func CreatePatch(path1 string, path2 string, target io.Writer) (err error) {

	// code adapted from the butler project, https://github.com/itchio/butler
	targetSignature := &pwr.SignatureInfo{}

	targetSignature.Container, err = tlc.WalkDir(path1, tlc.WalkOpts{Filter: tlc.KeepAllFilter})
	if err != nil {
		return errors.Wrapf(err, "walking %v as directory", path1)
	}
	targetPool := fspool.New(targetSignature.Container, path1)

	targetSignature.Hashes, err = pwr.ComputeSignature(context.Background(), targetSignature.Container, targetPool, &state.Consumer{})
	if err != nil {
		return errors.Wrapf(err, "computing signature of %v", path1)
	}

	var sourceContainer *tlc.Container
	sourceContainer, err = tlc.WalkDir(path2, tlc.WalkOpts{Filter: tlc.KeepAllFilter})
	if err != nil {
		return errors.Wrapf(err, "walking %v as directory", path2)
	}
	sourcePool := fspool.New(sourceContainer, path2)

	dctx := &pwr.DiffContext{
		SourceContainer: sourceContainer,
		Pool:            sourcePool,

		TargetContainer: targetSignature.Container,
		TargetSignature: targetSignature.Hashes,

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