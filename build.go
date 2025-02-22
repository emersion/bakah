package main

import (
	"context"
	"path/filepath"

	"github.com/containers/buildah/define"
	"github.com/containers/buildah/imagebuildah"
	"github.com/containers/buildah/pkg/parse"
	"github.com/containers/buildah/pkg/util"
	"github.com/containers/storage"
)

func Build(ctx context.Context, store storage.Store, f *File, dir string) error {
	for _, target := range f.Target {
		contextDir, err := filepath.Abs(filepath.Join(dir, target.Context))
		if err != nil {
			return err
		}

		var containerfile string
		if target.Dockerfile != "" {
			containerfile = filepath.Join(contextDir, target.Dockerfile)
		} else {
			var err error
			containerfile, err = util.DiscoverContainerfile(contextDir)
			if err != nil {
				return err
			}
		}

		args := make(map[string]string)
		for k, v := range target.Args {
			if v != nil {
				args[k] = *v
			}
		}

		additionalContexts := make(map[string]*define.AdditionalBuildContext)
		for name, value := range target.Contexts {
			buildCtx, err := parse.GetAdditionalBuildContext(value)
			if err != nil {
				return err
			}

			// GetAdditionalBuildContext resolves paths relative to the current
			// working directory
			if !buildCtx.IsImage && !buildCtx.IsURL {
				p, err := filepath.Abs(filepath.Join(dir, value))
				if err != nil {
					return err
				}
				buildCtx.Value = p
			}

			additionalContexts[name] = &buildCtx
		}

		options := define.BuildOptions{
			Args:                    args,
			ContextDirectory:        contextDir,
			Target:                  target.Target,
			AdditionalTags:          target.Tags,
			AdditionalBuildContexts: additionalContexts,
		}
		_, _, err = imagebuildah.BuildDockerfiles(ctx, store, options, containerfile)
		if err != nil {
			return err
		}
	}

	return nil
}
