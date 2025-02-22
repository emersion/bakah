package main

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/containers/buildah/define"
	"github.com/containers/buildah/imagebuildah"
	"github.com/containers/buildah/pkg/parse"
	"github.com/containers/buildah/pkg/util"
	"github.com/containers/storage"
)

func Build(ctx context.Context, store storage.Store, f *File, dir string, targetNames []string) error {
	var effectiveTargetNames []string
	seen := make(map[string]struct{})
	for _, name := range targetNames {
		if err := walkTarget(&effectiveTargetNames, seen, f, name); err != nil {
			return err
		}
	}

	for _, targetName := range effectiveTargetNames {
		target := f.Target[targetName]

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

		var platforms []struct{ OS, Arch, Variant string }
		for _, value := range target.Platforms {
			os, arch, variant, err := parse.Platform(value)
			if err != nil {
				return err
			}
			platforms = append(platforms, struct{ OS, Arch, Variant string }{os, arch, variant})
		}

		options := define.BuildOptions{
			Args:                    args,
			Annotations:             target.Annotations,
			ContextDirectory:        contextDir,
			Target:                  target.Target,
			AdditionalTags:          target.Tags,
			AdditionalBuildContexts: additionalContexts,
			Platforms:               platforms,
		}
		_, _, err = imagebuildah.BuildDockerfiles(ctx, store, options, containerfile)
		if err != nil {
			return err
		}
	}

	return nil
}

func walkTarget(targets *[]string, seen map[string]struct{}, f *File, name string) error {
	if _, ok := seen[name]; ok {
		return nil
	}
	seen[name] = struct{}{}

	if group, ok := f.Group[name]; ok {
		for _, dep := range group.Targets {
			if err := walkTarget(targets, seen, f, dep); err != nil {
				return err
			}
		}
	} else if _, ok := f.Target[name]; ok {
		*targets = append(*targets, name)
	} else {
		return fmt.Errorf("target %q not found", name)
	}
	return nil
}
