# hlfabric-k8scc
Chaincode launcher and builder for Hyperledger Fabric on Kubernetes 

Status: *In development*

## Features
This project implements an external chaincode launcher and builder for Hyperledger Fabric.
It talks directly to the Kubernetes API and doesn't depend on Docker which enables you to deploy
a Hyperledger Fabric Peer on Kubernetes without exposing the Docker socket and therefore improve security and portability.

The following points have been addressed:
- Chaincode build runs separated from the peer
- Chaincode is launched separated from the peer
- The build and launch of chaincode is compatible with the default/internal system
- The chaincode must be usable without changes
- The used images for build and launch of the chaincode can be the same as in the internal system
- Kubernetes security mechanisms can be enforced (network policies, host isolation, etc.)
- There is no dependency on the Kubernetes CRI implementation (like Docker)
- There is no need for priviledged pods
- stdout and stderr of the chaincode is forwarded to the peer
- The peer notices when a chaincode fails

## Usage
Requirements:
- The peer runs as a pod under a Kubernetes ServiceAccount that can manipulate pods
- The peer uses a `PersistentVolume` provided by a `PersistentVolumeClaim`, which is used to exchange data between the peer, builder and launcher pods

The easiest way to use this project is by using the `postfinance/hlfabric-k8scc` [Docker image](https://hub.docker.com/repository/docker/postfinance/hlfabric-k8scc). It's based on the Hyperledger Fabric Peer image and extended with a default [configuration](./k8scc.yaml).

When you have your peer running on Kubernetes, you need to ensure that the [rbac](./examples/rbac.yaml) are set and that the peer pod runs as a service account matching the rules. The following configuration must be done for the peer:
```yaml
spec:
  template:
    spec:
      serviceAccountName: peer                   # run peer as service account
      containers:
      - image: postfinance/hlfabric-k8scc:latest # use an appropriate image and tag
        - name: K8SCC_CFGFILE
          value: "/opt/k8scc/k8scc.yaml"         # this points to the default configuration file
        volumeMounts:
        - mountPath: /var/lib/k8scc/transfer/    # here we mount our transfer PV
          name: transfer-pv
        ports:
        - containerPort: 7051
        - containerPort: 7052
      volumes:
      - name: transfer-pv
        persistentVolumeClaim:
          claimName: k8scc-transfer-pv           # this is our default claim name for transfer PVs
```

You can have a look in the [example](./example/) directory for a more complete example.
If you want to customize the images or the resources, you need to use an own `k8scc.yaml` configuration file.

And if you have an own `core.yaml`, you need to configure the launcher. Have a look at this [patch](core.yaml.patch).
It is not possible to inject this data structure using environment variables.

## Development
*TODO*
