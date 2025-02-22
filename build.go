package main

import (
	"context"
	"path/filepath"

	"github.com/containers/buildah/define"
	"github.com/containers/buildah/imagebuildah"
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

		options := define.BuildOptions{
			Args:             args,
			ContextDirectory: contextDir,
			Target:           target.Target,
			AdditionalTags:   target.Tags,
		}
		_, _, err = imagebuildah.BuildDockerfiles(ctx, store, options, containerfile)
		if err != nil {
			return err
		}
	}

	return nil
}
