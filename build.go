package main

import (
	"context"
	"fmt"
	"math"
	"path/filepath"
	"strings"

	"github.com/containers/buildah/define"
	"github.com/containers/buildah/imagebuildah"
	"github.com/containers/buildah/pkg/parse"
	"github.com/containers/buildah/pkg/util"
	"github.com/containers/storage"
	"golang.org/x/sync/semaphore"
)

type pendingTarget struct {
	done     chan struct{}
	id       string
	metadata *BuildMetadata
	err      error
}

func (pt *pendingTarget) Wait() (string, *BuildMetadata, error) {
	<-pt.done
	return pt.id, pt.metadata, pt.err
}

type BuildMetadata struct {
	Digest string `json:"containerimage.digest,omitempty"`
	// TODO: add more fields
}

type BuildOptions struct {
	Store   storage.Store
	File    *File
	Dir     string
	Targets []string
	Layers  bool
	Jobs    int
}

func Build(ctx context.Context, options *BuildOptions) (map[string]*BuildMetadata, error) {
	f := options.File

	var targetNames []string
	seen := make(map[string]struct{})
	for _, name := range options.Targets {
		if err := walkTarget(&targetNames, seen, f, name); err != nil {
			return nil, err
		}
	}

	pendingTargets := make(map[string]*pendingTarget)
	for _, targetName := range targetNames {
		pendingTargets[targetName] = &pendingTarget{
			done: make(chan struct{}),
		}
	}

	jobs := int64(options.Jobs)
	if jobs == 0 {
		jobs = math.MaxInt64
	}
	sem := semaphore.NewWeighted(jobs)

	for targetName, pt := range pendingTargets {
		targetName, pt := targetName, pt // capture
		go func() {
			defer close(pt.done)

			id, metadata, err := buildTarget(ctx, options, sem, pendingTargets, f.Target[targetName])
			pt.id = id
			pt.metadata = metadata
			pt.err = err
		}()
	}

	var buildErr error
	metadata := make(map[string]*BuildMetadata)
	for targetName, pt := range pendingTargets {
		var targetMetadata *BuildMetadata
		_, targetMetadata, buildErr = pt.Wait()
		if buildErr != nil {
			break
		}
		metadata[targetName] = targetMetadata
	}

	return metadata, buildErr
}

func buildTarget(ctx context.Context, options *BuildOptions, sem *semaphore.Weighted, pendingTargets map[string]*pendingTarget, target *Target) (string, *BuildMetadata, error) {
	contextDir, err := filepath.Abs(resolvePath(options.Dir, target.Context))
	if err != nil {
		return "", nil, err
	}

	var containerfile string
	if target.Dockerfile != "" {
		containerfile = resolvePath(contextDir, target.Dockerfile)
	} else {
		var err error
		containerfile, err = util.DiscoverContainerfile(contextDir)
		if err != nil {
			return "", nil, err
		}
	}

	args := make(map[string]string)
	for k, v := range target.Args {
		if v != nil {
			args[k] = *v
		}
	}

	var (
		outputName     string
		additionalTags []string
	)
	if len(target.Tags) > 0 {
		outputName = target.Tags[0]
		additionalTags = target.Tags[1:]
	}

	additionalContexts := make(map[string]*define.AdditionalBuildContext)
	for name, value := range target.Contexts {
		if dep, ok := strings.CutPrefix(value, "target:"); ok {
			depPendingTarget, ok := pendingTargets[dep]
			if !ok {
				panic("unreachable")
			}
			depID, _, err := depPendingTarget.Wait()
			if err != nil {
				return "", nil, err
			}
			additionalContexts[name] = &define.AdditionalBuildContext{
				IsImage: true,
				Value:   depID,
			}
			continue
		}

		buildCtx, err := parse.GetAdditionalBuildContext(value)
		if err != nil {
			return "", nil, err
		}

		// GetAdditionalBuildContext resolves paths relative to the current
		// working directory
		if !buildCtx.IsImage && !buildCtx.IsURL {
			p, err := filepath.Abs(filepath.Join(options.Dir, value))
			if err != nil {
				return "", nil, err
			}
			buildCtx.Value = p
		}

		additionalContexts[name] = &buildCtx
	}

	var platforms []struct{ OS, Arch, Variant string }
	for _, value := range target.Platforms {
		os, arch, variant, err := parse.Platform(value)
		if err != nil {
			return "", nil, err
		}
		platforms = append(platforms, struct{ OS, Arch, Variant string }{os, arch, variant})
	}

	pullPolicy, err := parsePullPolicy(target.Pull)
	if err != nil {
		return "", nil, err
	}

	buildOptions := define.BuildOptions{
		Args:                    args,
		Annotations:             target.Annotations,
		ContextDirectory:        contextDir,
		Target:                  target.Target,
		Output:                  outputName,
		AdditionalTags:          additionalTags,
		AdditionalBuildContexts: additionalContexts,
		Platforms:               platforms,
		NoCache:                 target.NoCache,
		PullPolicy:              pullPolicy,
		Layers:                  options.Layers,
		JobSemaphore:            sem,
	}
	id, ref, err := imagebuildah.BuildDockerfiles(ctx, options.Store, buildOptions, containerfile)
	if err != nil {
		return "", nil, err
	}

	var digest string
	if ref != nil {
		// TODO: ref is nil when define.BuildOptions.Output is empty
		digest = ref.Digest().String()
	}

	metadata := &BuildMetadata{Digest: digest}
	return id, metadata, nil
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
	} else if target, ok := f.Target[name]; ok {
		for _, value := range target.Contexts {
			if dep, ok := strings.CutPrefix(value, "target:"); ok {
				if err := walkTarget(targets, seen, f, dep); err != nil {
					return err
				}
			}
		}
		*targets = append(*targets, name)
	} else {
		return fmt.Errorf("target %q not found", name)
	}
	return nil
}

func parsePullPolicy(value string) (define.PullPolicy, error) {
	switch strings.ToLower(value) {
	case "", "true", "missing", "ifmissing", "notpresent":
		return define.PullIfMissing, nil
	case "always":
		return define.PullAlways, nil
	case "false", "never":
		return define.PullNever, nil
	case "ifnewer", "newer":
		return define.PullIfNewer, nil
	default:
		return 0, fmt.Errorf("unknown pull policy %q", value)
	}
}

func resolvePath(base, target string) string {
	if filepath.IsAbs(target) {
		return target
	}
	return filepath.Join(base, target)
}
