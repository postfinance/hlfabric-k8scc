# This needs to be executed after cli1.sh on peer2

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
sleep 15 # isInit works asynchronously, maybe there is a better method to wait

# query chaincode
peer chaincode query -C mychannel -n fabcar -c '{"Args":["queryAllCars"]}'
