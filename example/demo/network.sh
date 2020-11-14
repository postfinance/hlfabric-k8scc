#!/bin/bash

# Print the usage message
function printHelp() {
  echo "Usage: "
  echo "  network.sh <Mode>"
  echo "    <Mode>"
  echo "      - 'up' - bring up fabric orderer and peer nodes. No channel is created"
  echo "      - 'down' - clear the network with docker-compose down"
  echo "      - 'restart' - restart the network"
  echo
  echo "    Flags:"
  echo "    -o <overlay> -  overlay to use - default=local"
  echo
  echo "  network.sh -h (print this message)"
  echo
  echo " Examples:"
  echo "  network.sh up"
  echo "  network.sh down"
}

function networkUp() {
  kubectl create configmap config --from-file=configs/configtx.yaml
  kubectl create configmap cryptogen --from-file=configs/crypto-config-orderer.yaml --from-file=configs/crypto-config-org1.yaml --from-file=configs/crypto-config-org2.yaml
  kubectl create configmap core --from-file=configs/core.yaml
  kubectl apply -f k8s/bootstrap
  kubectl wait --for=condition=complete job/setup
  kubectl apply -k k8s/components/overlays/$OVERLAY
  kubectl wait --for=condition=ready pod/$(kubectl get pod -l app=cli.peer0.org1.example.com -o jsonpath="{.items[0].metadata.name}")
  kubectl cp chaincodes/fabcar.tar.gz $(kubectl get pod -l app=cli.peer0.org1.example.com -o jsonpath="{.items[0].metadata.name}"):/chaincodes
  kubectl cp chaincodes/marbles02.tar.gz $(kubectl get pod -l app=cli.peer0.org1.example.com -o jsonpath="{.items[0].metadata.name}"):/chaincodes
  kubectl wait --for=condition=ready pod/$(kubectl get pod -l app=orderer.example.com -o jsonpath="{.items[0].metadata.name}")
}

function networkDown() {
  kubectl delete -k k8s/components/overlays/$OVERLAY
  kubectl delete -f k8s/bootstrap
  kubectl delete configmap config
  kubectl delete configmap cryptogen
  kubectl delete configmap core
}

OVERLAY="local"

if [[ $# -lt 1 ]]; then
  printHelp
  exit 0
else
  MODE=$1
  shift
fi

# parse flags

while [[ $# -ge 1 ]] ; do
  key="$1"
  case $key in
  -h )
    printHelp
    exit 0
    ;;
  -o )
    OVERLAY="$2"
    shift
    ;;
  * )
    echo
    echo "Unknown flag: $key"
    echo
    printHelp
    exit 1
    ;;
  esac
  shift
done

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
