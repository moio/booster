package wharf

import (
	"context"
	"github.com/itchio/headway/state"
	"github.com/itchio/lake/pools/fspool"
	"github.com/itchio/lake/tlc"
	"github.com/itchio/savior/filesource"
	_ "github.com/itchio/wharf/compressors/cbrotli"
	_ "github.com/itchio/wharf/decompressors/cbrotli"
	"github.com/itchio/wharf/pwr"
	"github.com/itchio/wharf/pwr/bowl"
	"github.com/itchio/wharf/pwr/patcher"
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

		Consumer: &state.Consumer{},
		Compression: &pwr.CompressionSettings{
			Algorithm: pwr.CompressionAlgorithm_BROTLI,
			Quality:   7, // "plateau" for brotli, see https://blogs.akamai.com/2016/02/understanding-brotlis-potential.html
		},
	}

	err = dctx.WritePatch(context.Background(), target, ioutil.Discard)
	if err != nil {
		return errors.Wrap(err, "computing and writing patch")
	}

	return nil
}

func Apply(patch string, old string) error {
	stagingDir := "staging"

	patchSource, err := filesource.Open(patch)
	if err != nil {
		return errors.WithMessage(err, "opening patch")
	}

	p, err := patcher.New(patchSource, &state.Consumer{})
	if err != nil {
		return errors.WithMessage(err, "creating patcher")
	}

	targetPool := fspool.New(p.GetTargetContainer(), old)

	var bwl bowl.Bowl
	bwl, err = bowl.NewOverlayBowl(bowl.OverlayBowlParams{
		SourceContainer: p.GetSourceContainer(),
		TargetContainer: p.GetTargetContainer(),
		OutputFolder:    old,
		StageFolder:     stagingDir,
	})
	if err != nil {
		return errors.WithMessage(err, "creating overlay bowl")
	}

	err = p.Resume(nil, targetPool, bwl)
	if err != nil {
		return errors.WithMessage(err, "patching")
	}

	err = bwl.Commit()
	if err != nil {
		return errors.WithMessage(err, "committing bowl")
	}

	return nil
}
