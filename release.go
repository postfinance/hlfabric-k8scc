package main

import (
	"context"
	"fmt"
	"io/ioutil"
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
	metaInf := filepath.Join(sourceDir, "META-INF")
	if _, err := os.Stat(metaInf); !os.IsNotExist(err) {
		entries, err := ioutil.ReadDir(metaInf)
		if err != nil {
			return errors.Wrap(err, "accessing META-INF")
		}

		for _, entry := range entries {
			err = cpy.Copy(entry.Name(), outputDir)
			if err != nil {
				return errors.Wrap(err, fmt.Sprintf("copy %q from META-INF to output dir", entry.Name()))
			}
		}
	}

	return nil
}
