package main

import (
	"log"
	"strings"

	pb "github.com/hyperledger/fabric-protos-go/peer"
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
	// platforms are defined as uppercase in protobuf
	ccType = strings.ToUpper(ccType)

	dvm := dockercontroller.DockerVM{}
	args, err := dvm.GetArgs(ccType, peerAddress)
	if err != nil {
		log.Printf("dockercontroller.GetArgs returned %q, but we will use a default", err)
		return []string{"chaincode", "-peer.address", peerAddress}
	}

	return args
}

// GetMountDir returns the mount directory for the chaincode depending on the platform.
// This is required as DockerVM.GetArgs assumes a platform dependend setup.
func GetCCMountDir(ccType string) string {
	// platforms are defined as uppercase in protobuf
	ccType = strings.ToUpper(ccType)

	switch ccType {
	case pb.ChaincodeSpec_GOLANG.String():
		// https://github.com/hyperledger/fabric/blob/v2.1.1/core/chaincode/platforms/golang/platform.go#L192
		return "/usr/local/bin"
	case pb.ChaincodeSpec_JAVA.String():
		// https://github.com/hyperledger/fabric/blob/v2.1.1/core/chaincode/platforms/java/platform.go#L125
		return "/root/chaincode-java/chaincode"
	case pb.ChaincodeSpec_NODE.String():
		// https://github.com/hyperledger/fabric/blob/v2.1.1/core/chaincode/platforms/node/platform.go#L170
		return "/usr/local/src"
	default:
		// Fall back to Go dir
		log.Printf("Unknown platform %q for chaincode mount dir, we will use a default", ccType)
		return "/usr/local/bin"
	}
}
