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

	var filenames []string
	pflag.StringArrayVar(&filenames, "file", nil, "Build definition file")
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

	// TODO: grab targets from args

	f, err := os.Open(filename)
	if err != nil {
		log.Fatalf("failed to open Bake file: %v", err)
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
	if err := Build(ctx, store, bakefile, filepath.Dir(filename)); err != nil {
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
