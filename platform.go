package main

import (
	"github.com/hyperledger/fabric/core/chaincode/platforms"
	"strings"
)

// GetPlatform returns the chaincode platform as defined by HyperLedger Fabric Peer
func GetPlatform(ccType string) platforms.Platform {
	for _, plt := range platforms.SupportedPlatforms {
		if plt.Name() == strings.ToUpper(ccType) {
			return plt
		}
	}

	return nil
}
