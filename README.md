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
This software implements the four parts of an external chaincode launcher and builder: `detect`, `build`, `release`, `run`.

### Step `detect`
The step `detect` just checks the `metadata.json` if the defined platform (e.g. `golang`) is available in Hyperledger Fabric and if an appropriate image is configured in `k8scc.yaml`

### Step `build`
The step `build` is responsible for building the chaincode while ensuring compatibility of the chaincode with the internal builder process.

The preparation:
1. Parse `metadata.json` and check if the plattform (e.g. `golang`) is supported
2. Create a temporary directory on the transfer volume 
3. Inside this temporary directory, copy the provided chaincode source and create an empty directory for the build output

Next, a builder pod is created and has the following properties:
- The name is `{{ peer pod name}}-cc-{{ chaincode label }}-{{ short hash }}`
- It has the temporary subdirectories of the transfer PV mounted
- The command is the same as the one used by Hyperledger Fabric on its internal builder

The created pod is watched until it finishes either successfully or not.
Afterwards this procedure is executed:
1. Write stdout and stderr of the builder pod to the peer log
2. Only if the build failed: Remove all garbage (pod + temporary directory) and exit
3. Copy output data from the temporary directory to the output directory on the peer
4. Copy data from the `META-INF` in the source directory to the output directory on the peer
5. Write build information to the output directory, in order to use the same image for the launch as for the build
6. Cleanup pod and remove the temporary directory
