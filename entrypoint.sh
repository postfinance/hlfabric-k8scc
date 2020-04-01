#!/bin/sh

# Chec if core.yaml has still its original content
md5sum -c /opt/k8scc/core.yaml.md5sum

if  [ $? -eq 0 ]; then 
	patch /etc/hyperledger/fabric/core.yaml /opt/k8scc/core.yaml.patch
fi

echo "Running: $@"
exec "$@"
