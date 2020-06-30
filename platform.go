package main

import (
	"log"
	"strings"

	"github.com/hyperledger/fabric/core/chaincode/platforms"
	"github.com/hyperledger/fabric/core/container/dockercontroller"
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

// GetRunArgs returns the chaincode run arguments as defined by HyperLedger Fabric Peer
func GetRunArgs(ccType, peerAddress string) []string {
	dvm := dockercontroller.DockerVM{}
	args, err := dvm.GetArgs(ccType, peerAddress)
	if err != nil {
		log.Println("dockercontroller.GetArgs returned %q, but we will use a default", err)
		return []string{"chaincode", "-peer.address", peerAddress}
	}

	return args
}
