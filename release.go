package main

import (
	"context"
	"log"
	"os"
	"path/filepath"

	cpy "github.com/otiai10/copy"
	"github.com/pkg/errors"
)

// Release copies the META-INF data from the chaincode source to the release directory
// on the peer
func Release(ctx context.Context, cfg Config) error {
	log.Println("Procedure: release")

	if len(os.Args) != 3 {
		return errors.New("release requires exactly two arguments")
	}

	// bld dir from external builder
	sourceDir := os.Args[1]
	outputDir := os.Args[2]

	// Copy statedb from bld dir, if available
	statedbSrc := filepath.Join(sourceDir, "statedb")
	statedbDest := filepath.Join(outputDir, "statedb");
	if _, err := os.Stat(statedbSrc); !os.IsNotExist(err) {
		err = cpy.Copy(statedbSrc, statedbDest)
		if err != nil {
			return errors.Wrap(err, "accessing statedb folder")
		}
	}

	return nil
}
