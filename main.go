package main

import (
	"context"
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
		filenames []string
		layers    bool
		jobs      int
	)
	pflag.StringArrayVarP(&filenames, "file", "f", nil, "Build definition file")
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
	if err := Build(ctx, options); err != nil {
		log.Fatal(err)
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
