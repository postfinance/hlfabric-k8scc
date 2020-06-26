# hlfabric-k8scc demo
This demo is inspired by the [test-network](https://github.com/hyperledger/fabric-samples/tree/master/test-network) of 
the [fabric-samples](https://github.com/hyperledger/fabric-samples).
It demonstrates how you can use the [postfinance/hlfabric-k8scc](https://github.com/postfinance/hlfabric-k8scc/) 
to deploy a fabric network in kubernetes without exposing the docker socket.

## Prerequisites
Before you can run this demo make sure you have installed the Fabric 
[Prerequisites](https://hyperledger-fabric.readthedocs.io/en/latest/prereqs.html) and 
[Samples, Binaries, and Docker Images](https://hyperledger-fabric.readthedocs.io/en/latest/install.html).
Furthermore you need a running Kubernetes cluster (easiest way is to deploy it with [Minikube](https://kubernetes.io/docs/tasks/tools/install-minikube/)) 
and [Kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl/).

## Bring up the network
Similar to the test-network sample you can find the network.sh script to bring up the network in the ```example/demo``` 
directory of the ```hlfabric-k8scc``` repository.
```shell script
# Generate and start the network
./network.sh up

# Stop and destroy the network
./network.sh down

# Recreate the network
./network.sh restart
```

## Interact with the network

### cli peer org1
```shell script
# connect to cli pod
kubectl exec -it $(kubectl get pod -l app=cli.peer0.org1.example.com -o jsonpath="{.items[0].metadata.name}") -- bash

# create and join channel
configtxgen -configPath ./ -profile TwoOrgsChannel -channelID mychannel -outputCreateChannelTx /channels/mychannel/mychannel.tx
peer channel create -c mychannel -f /channels/mychannel/mychannel.tx -o orderer-example-com:7050 --tls --cafile /etc/hyperledger/organizations/ordererOrganizations/example.com/tlsca/tlsca.example.com-cert.pem
mv mychannel.block /channels/mychannel
peer channel join -b /channels/mychannel/mychannel.block
peer channel list

# install chaincode
peer lifecycle chaincode install /chaincodes/fabcar.tar.gz
peer lifecycle chaincode queryinstalled

# approve chaincode
peer lifecycle chaincode approveformyorg -o orderer-example-com:7050 --tls true --cafile /etc/hyperledger/organizations/ordererOrganizations/example.com/tlsca/tlsca.example.com-cert.pem --channelID mychannel --name fabcar --version 1 --sequence 1 --init-required --package-id fabcar_1:5a10300271158be80c65b9500268f9fc0abc1fb6247eae93adf2915d273651f4
peer lifecycle chaincode checkcommitreadiness --channelID mychannel --name fabcar --version 1 --sequence 1 --output json --init-required
```
### cli peer org2
```shell script
# connect to cli pod
kubectl exec -it $(kubectl get pod -l app=cli.peer0.org2.example.com -o jsonpath="{.items[0].metadata.name}") -- bash

# join channel
peer channel join -b /channels/mychannel/mychannel.block
peer channel list

# install chaincode
peer lifecycle chaincode install /chaincodes/fabcar.tar.gz
peer lifecycle chaincode queryinstalled

# approve chaincode
peer lifecycle chaincode approveformyorg -o orderer-example-com:7050 --tls true --cafile /etc/hyperledger/organizations/ordererOrganizations/example.com/tlsca/tlsca.example.com-cert.pem --channelID mychannel --name fabcar --version 1 --sequence 1 --init-required --package-id fabcar_1:5a10300271158be80c65b9500268f9fc0abc1fb6247eae93adf2915d273651f4
peer lifecycle chaincode checkcommitreadiness --channelID mychannel --name fabcar --version 1 --sequence 1 --output json --init-required

# commit chaincode
peer lifecycle chaincode commit -o orderer-example-com:7050 --tls true --cafile /etc/hyperledger/organizations/ordererOrganizations/example.com/tlsca/tlsca.example.com-cert.pem --channelID mychannel --name fabcar --peerAddresses peer0-org2-example-com:7051 --tlsRootCertFiles /etc/hyperledger/organizations/peerOrganizations/org2.example.com/peers/peer0.org2.example.com/tls/ca.crt --peerAddresses peer0-org1-example-com:7051 --tlsRootCertFiles /etc/hyperledger/organizations/peerOrganizations/org1.example.com/peers/peer0.org1.example.com/tls/ca.crt --version 1 --sequence 1 --init-required
peer lifecycle chaincode  querycommitted -C mychannel

# init and invoke chaincode
peer chaincode invoke -o orderer-example-com:7050 --tls true --cafile /etc/hyperledger/organizations/ordererOrganizations/example.com/tlsca/tlsca.example.com-cert.pem -C mychannel -n fabcar --peerAddresses peer0-org2-example-com:7051 --tlsRootCertFiles /etc/hyperledger/organizations/peerOrganizations/org2.example.com/peers/peer0.org2.example.com/tls/ca.crt --peerAddresses peer0-org1-example-com:7051 --tlsRootCertFiles /etc/hyperledger/organizations/peerOrganizations/org1.example.com/peers/peer0.org1.example.com/tls/ca.crt --isInit -c '{"function":"initLedger","Args":[]}'

# query chaincode
peer chaincode query -C mychannel -n fabcar -c '{"Args":["queryAllCars"]}'
```