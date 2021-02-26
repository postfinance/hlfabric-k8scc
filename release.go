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

	sourceDir := os.Args[1]
	outputDir := os.Args[2]

	// Copy META-INF, if available
	metaInf := filepath.Join(sourceDir, "statedb")
	if _, err := os.Stat(metaInf); !os.IsNotExist(err) {
		err = cpy.Copy(metaInf, outputDir)
		if err != nil {
			return errors.Wrap(err, "accessing statedb folder")
		}
	}

	return nil
}
