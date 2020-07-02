#!/bin/bash

#export PATH=${PWD}/../bin:$PATH
export FABRIC_CFG_PATH=${PWD}/configs

# Print the usage message
function printHelp() {
  echo "Usage: "
  echo "  network.sh <Mode>"
  echo "    <Mode>"
  echo "      - 'up' - bring up fabric orderer and peer nodes. No channel is created"
  echo "      - 'down' - clear the network with docker-compose down"
  echo "      - 'restart' - restart the network"
  echo
  echo "  network.sh -h (print this message)"
  echo
  echo " Examples:"
  echo "  network.sh up"
  echo "  network.sh down"
}

# Versions of fabric known not to work with the network
BLACKLISTED_VERSIONS="^1\.0\. ^1\.1\. ^1\.2\. ^1\.3\. ^1\.4\."

# Do some basic sanity checking to make sure that the appropriate versions of fabric
# binaries/images are available. In the future, additional checking for the presence
# of go or other items could be added.
function checkPrereqs() {
  ## Check if your have cloned the peer binaries and configuration files.
  peer version >/dev/null 2>&1

  if [[ $? -ne 0 || ! -d "./configs" ]]; then
    echo "ERROR! Peer binary and configuration files not found.."
    echo
    echo "Follow the instructions in the Fabric docs to install the Fabric Binaries:"
    echo "https://hyperledger-fabric.readthedocs.io/en/latest/install.html"
    exit 1
  fi
  # use the fabric tools container to see if the samples and binaries match your
  # docker images
  LOCAL_VERSION=$(peer version | sed -ne 's/ Version: //p')
  DOCKER_IMAGE_VERSION=$(docker run --rm hyperledger/fabric-tools:$IMAGETAG peer version | sed -ne 's/ Version: //p' | head -1)

  echo "LOCAL_VERSION=$LOCAL_VERSION"
  echo "DOCKER_IMAGE_VERSION=$DOCKER_IMAGE_VERSION"

  if [ "$LOCAL_VERSION" != "$DOCKER_IMAGE_VERSION" ]; then
    echo "=================== WARNING ==================="
    echo "  Local fabric binaries and docker images are  "
    echo "  out of  sync. This may cause problems.       "
    echo "==============================================="
  fi

  for UNSUPPORTED_VERSION in $BLACKLISTED_VERSIONS; do
    echo "$LOCAL_VERSION" | grep -q $UNSUPPORTED_VERSION
    if [ $? -eq 0 ]; then
      echo "ERROR! Local Fabric binary version of $LOCAL_VERSION does not match the versions supported by the test network."
      exit 1
    fi

    echo "$DOCKER_IMAGE_VERSION" | grep -q $UNSUPPORTED_VERSION
    if [ $? -eq 0 ]; then
      echo "ERROR! Fabric Docker image version of $DOCKER_IMAGE_VERSION does not match the versions supported by the test network."
      exit 1
    fi
  done
}

function networkUp() {
  checkPrereqs
  kubectl create configmap config --from-file=configs/configtx.yaml
  kubectl create configmap cryptogen --from-file=configs/crypto-config-orderer.yaml --from-file=configs/crypto-config-org1.yaml --from-file=configs/crypto-config-org2.yaml
  kubectl create configmap core --from-file=configs/core.yaml
  kubectl apply -f k8s/bootstrap
  kubectl wait --for=condition=complete job/setup
  kubectl apply -f k8s/components
  kubectl wait --for=condition=ready pod/$(kubectl get pod -l app=cli.peer0.org1.example.com -o jsonpath="{.items[0].metadata.name}")
  kubectl cp chaincodes/fabcar.tar.gz $(kubectl get pod -l app=cli.peer0.org1.example.com -o jsonpath="{.items[0].metadata.name}"):/chaincodes
  kubectl cp chaincodes/marbles02.tar.gz $(kubectl get pod -l app=cli.peer0.org1.example.com -o jsonpath="{.items[0].metadata.name}"):/chaincodes
}

function networkDown() {
  kubectl delete -f k8s/components
  kubectl delete -f k8s/bootstrap
  kubectl delete configmap config
  kubectl delete configmap cryptogen
  kubectl delete configmap core
}

if [[ $# -lt 1 ]]; then
  printHelp
  exit 0
else
  MODE=$1
  shift
fi

# Determine mode of operation and printing out what we asked for
if [ "$MODE" == "up" ]; then
  echo "Starting nodes with CLI timeout of '${MAX_RETRY}' tries and CLI delay of '${CLI_DELAY}' seconds and using database '${DATABASE}' ${CRYPTO_MODE}"
  echo
elif [ "$MODE" == "down" ]; then
  echo "Stopping network"
  echo
elif [ "$MODE" == "restart" ]; then
  echo "Restarting network"
  echo
else
  printHelp
  exit 1
fi

if [ "${MODE}" == "up" ]; then
  networkUp
elif [ "${MODE}" == "down" ]; then
  networkDown
elif [ "${MODE}" == "restart" ]; then
  networkDown
  networkUp
else
  printHelp
  exit 1
fi
