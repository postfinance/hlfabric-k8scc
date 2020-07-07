# This needs to be executed in peer1

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
