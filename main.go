package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/containers/buildah/imagebuildah"
	"github.com/containers/storage"
	"github.com/containers/storage/pkg/unshare"
	"github.com/spf13/pflag"
)

func main() {
	if imagebuildah.InitReexec() {
		return
	}

	unshare.MaybeReexecUsingUserNamespace(false)

	var (
		filenames    []string
		noCache      bool
		metadataFile string
		layers       bool
		jobs         int
	)
	pflag.StringArrayVarP(&filenames, "file", "f", nil, "Build definition file")
	pflag.BoolVar(&noCache, "no-cache", false, "Do not use cache when building the image")
	pflag.StringVar(&metadataFile, "metadata-file", "", "Write build result metadata to a file")
	pflag.BoolVar(&layers, "layers", true, "Cache intermediate images during the build process")
	pflag.IntVar(&jobs, "jobs", 1, "How many stages to run in parallel")
	pflag.Parse()

	var filename string
	if len(filenames) == 0 {
		filename = "docker-bake.json"
	} else if len(filenames) == 1 {
		filename = filenames[0]
	} else {
		// TODO: support multiple files
		log.Fatal("Only a single Bake file is supported")
	}

	targetNames := pflag.Args()
	if len(targetNames) == 0 {
		targetNames = []string{"default"}
	}

	var (
		f       *os.File
		dirname string
	)
	if filename == "-" {
		f = os.Stdin
	} else {
		var err error
		f, err = os.Open(filename)
		if err != nil {
			log.Fatalf("failed to open Bake file: %v", err)
		}
		dirname = filepath.Dir(filename)
	}
	defer f.Close()

	bakefile, err := Decode(f)
	if err != nil {
		log.Fatalf("failed to decode Bake file: %v", err)
	}

	if noCache {
		for _, target := range bakefile.Target {
			target.NoCache = true
		}
	}

	store, err := getStore()
	if err != nil {
		log.Fatal(err)
	}
	defer store.Shutdown(false)

	ctx := context.Background()
	options := &BuildOptions{
		Store:   store,
		File:    bakefile,
		Dir:     dirname,
		Targets: targetNames,
		Layers:  layers,
		Jobs:    jobs,
	}
	metadata, err := Build(ctx, options)
	if err != nil {
		log.Fatal(err)
	}

	if metadataFile != "" {
		if err := writeMetadataFile(metadataFile, metadata); err != nil {
			log.Fatal(err)
		}
	}
}

func getStore() (storage.Store, error) {
	storeOptions, err := storage.DefaultStoreOptions()
	if err != nil {
		return nil, fmt.Errorf("failed to query default store options: %v", err)
	}

	store, err := storage.GetStore(storeOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to get store: %v", err)
	}

	return store, nil
}

func writeMetadataFile(filename string, metadata map[string]*BuildMetadata) error {
	f, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create metadata file: %v", err)
	}
	defer f.Close()

	if err := json.NewEncoder(f).Encode(metadata); err != nil {
		return fmt.Errorf("failed to encode metadata file: %v", err)
	}

	if err := f.Close(); err != nil {
		return fmt.Errorf("failed to close metadata file: %v", err)
	}

	return nil
}
