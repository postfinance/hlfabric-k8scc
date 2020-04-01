# hlfabric-k8scc
Chaincode launcher and builder for Hyperledger Fabric on Kubernetes 

Status: *In development*

## Features
This project implements an external chaincode launcher and builder for Hyperledger Fabric.
It talks directly to the Kubernetes API and doesn't depend on Docker which enables you to deploy
a Hyperledger Fabric Peer on Kubernetes without exposing the Docker socket and therefore improve security and portability.

The following points are addressed:
- Chaincode build runs separated from the peer
- Chaincode is launched separated from the peer
- The build and launch of chaincode is compatible with the default/internal system
- The chaincode must be usable without changes
- The used images for build and launch of the chaincode can be the same as in the internal system
- Kubernetes security mechanisms can be enforced (NetworkPolicies, Host isolation, etc.)
- There is no dependency on the Kubernetes CRI implementation (like Docker)
- There is no need for priviledged Pods

## Usage
Requirements:
- The peer runs as a Pod under a Kubernetes ServiceAccount that can manipulate Pods
- The peer uses a PersistentVolume provided by a claim, which is used to exchange data between the peer and the builder and launcher Pods

*TODO*

## Development
*TODO*
