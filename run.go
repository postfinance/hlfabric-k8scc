package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"

	cpy "github.com/otiai10/copy"
	"github.com/pkg/errors"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Run implements the chaincode launcher on Kubernetes whose function is implemented after
// https://github.com/hyperledger/fabric/blob/v2.1.1/integration/externalbuilders/golang/bin/run
func Run(ctx context.Context, cfg Config) error {
	log.Println("Procedure: run")

	if len(os.Args) != 3 {
		return errors.New("run requires exactly two arguments")
	}

	outputDir := os.Args[1]
	metadataDir := os.Args[2]

	// Read run configuration
	runConfig, err := getChaincodeRunConfig(metadataDir, outputDir)
	if err != nil {
		return errors.Wrap(err, "getting run config for chaincode")
	}

	// Create transfer dir
	copyOpts := cpy.Options{AddPermission: os.ModePerm}

	prefix, _ := os.Hostname()
	transferdir, err := ioutil.TempDir(cfg.TransferVolume.Path, prefix)
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("creating directory %s on transfer volume", cfg.TransferVolume.Path))
	}
	defer os.RemoveAll(transferdir)

	// Setup transfer
	transferOutput := filepath.Join(transferdir, "output")
	transferArtifacts := filepath.Join(transferdir, "artifacts")

	// Copy outputDir to transfer PV
	err = cpy.Copy(outputDir, transferOutput, copyOpts)
	if err != nil {
		return errors.Wrap(err, "copy output dir to transfer dir")
	}

	// Create artifacts dir on transfer PV
	err = os.Mkdir(transferArtifacts, os.ModePerm) // Apply full permissions, but this is before umask
	if err != nil {
		return errors.Wrap(err, "create artifacts dir in the transfer dir")
	}
	err = os.Chmod(transferArtifacts, os.ModePerm)
	if err != nil {
		return errors.Wrap(err, "chmod on artifacts dir in the transfer dir")
	}

	// Create artifacts
	err = createArtifacts(runConfig, transferArtifacts)
	if err != nil {
		return errors.Wrap(err, "creating artifacts")
	}

	// Create chaincode pod
	pod, err := createChaincodePod(ctx, cfg, runConfig, filepath.Base(transferdir))
	if err != nil {
		return errors.Wrap(err, "creating chaincode pod")
	}
	defer cleanupPodSilent(pod) // Cleanup pod on finish

	// Watch chaincode Pod for completion or failure
	podSucceeded, err := watchPodUntilCompletion(ctx, pod)
	if err != nil {
		return errors.Wrap(err, "watching chaincode pod")
	}

	if !podSucceeded {
		return fmt.Errorf("chaincode %s in Pod %s failed", runConfig.CCID, pod.Name)
	}

	return nil
}

func createArtifacts(c *ChaincodeRunConfig, dir string) error {
	clientCertPath := filepath.Join(dir, "client.crt")
	clientKeyPath := filepath.Join(dir, "client.key")
	clientCertFile := filepath.Join(dir, "client_pem.crt")
	clientKeyFile := filepath.Join(dir, "client_pem.key")
	peerCertFile := filepath.Join(dir, "root.crt")

	// Create cert files
	err := ioutil.WriteFile(clientCertFile, []byte(c.ClientCert), os.ModePerm)
	if err != nil {
		return errors.Wrap(err, "writing client cert pem file")
	}

	err = ioutil.WriteFile(clientKeyFile, []byte(c.ClientKey), os.ModePerm)
	if err != nil {
		return errors.Wrap(err, "writing client key pem file")
	}

	err = ioutil.WriteFile(peerCertFile, []byte(c.RootCert), os.ModePerm)
	if err != nil {
		return errors.Wrap(err, "writing peer cert file")
	}

	// Create weird cert files (used by node platform)
	// https://github.com/hyperledger/fabric/blob/v2.1.1/core/container/dockercontroller/dockercontroller.go#L319
	err = ioutil.WriteFile(clientCertPath, []byte(base64.StdEncoding.EncodeToString([]byte(c.ClientCert))), os.ModePerm)
	if err != nil {
		return errors.Wrap(err, "writing client cert file")
	}

	err = ioutil.WriteFile(clientKeyPath, []byte(base64.StdEncoding.EncodeToString([]byte(c.ClientKey))), os.ModePerm)
	if err != nil {
		return errors.Wrap(err, "writing client key file")
	}

	// Change permissions
	err = os.Chmod(clientCertFile, os.ModePerm)
	if err != nil {
		return errors.Wrap(err, "changing client cert pem file permissions")
	}

	err = os.Chmod(clientKeyFile, os.ModePerm)
	if err != nil {
		return errors.Wrap(err, "changing client key pem file permissions")
	}

	err = os.Chmod(clientCertPath, os.ModePerm)
	if err != nil {
		return errors.Wrap(err, "changing client key file permissions")
	}

	err = os.Chmod(clientKeyPath, os.ModePerm)
	if err != nil {
		return errors.Wrap(err, "changing client key file permissions")
	}

	err = os.Chmod(peerCertFile, os.ModePerm)
	if err != nil {
		return errors.Wrap(err, "changing peer cert file permissions")
	}

	return nil
}

func getChaincodeRunConfig(metadataDir string, outputDir string) (*ChaincodeRunConfig, error) {
	// Read chaincode.json
	metadataFile := filepath.Join(metadataDir, "chaincode.json")
	metadataData, err := ioutil.ReadFile(metadataFile)
	if err != nil {
		return nil, errors.Wrap(err, "Reading chaincode.json")
	}

	metadata := ChaincodeRunConfig{}
	err = json.Unmarshal(metadataData, &metadata)
	if err != nil {
		return nil, errors.Wrap(err, "Unmarshaling chaincode.json")
	}

	// Create shortname
	parts := strings.SplitN(metadata.CCID, ":", 2)
	if len(parts) != 2 {
		return nil, errors.New("Cannot parse chaincode name")
	}

	name := strings.ReplaceAll(parts[0], "_", "-")
	hash := parts[1]
	if len(hash) < 8 {
		return nil, errors.New("Hash of chaincode ID too short")
	}

	metadata.ShortName = fmt.Sprintf("%s-%s", name, hash[0:8])

	// Read BuildInformation
	buildInfoFile := filepath.Join(outputDir, "k8scc_buildinfo.json")
	buildInfoData, err := ioutil.ReadFile(buildInfoFile)
	if err != nil {
		return nil, errors.Wrap(err, "Reading k8scc_buildinfo.json")
	}

	buildInformation := BuildInformation{}
	err = json.Unmarshal(buildInfoData, &buildInformation)
	if err != nil {
		return nil, errors.Wrap(err, "Unmarshaling k8scc_buildinfo.json")
	}

	if buildInformation.Image == "" {
		return nil, errors.New("No image found in buildinfo")
	}

	metadata.Image = buildInformation.Image
	metadata.Platform = buildInformation.Platform

	return &metadata, nil
}

func createChaincodePod(ctx context.Context,
	cfg Config, runConfig *ChaincodeRunConfig, transferPVPrefix string) (*apiv1.Pod, error) {
	// Setup kubernetes client
	clientset, err := getKubernetesClientset()
	if err != nil {
		return nil, errors.Wrap(err, "getting kubernetes clientset")
	}

	// Get peer Pod
	myself, _ := os.Hostname()
	myselfPod, err := clientset.CoreV1().Pods(cfg.Namespace).Get(ctx, myself, metav1.GetOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "getting myself Pod")
	}

	// Set resources
	limits := apiv1.ResourceList{}
	if limit := cfg.Launcher.Resources.LimitMemory; limit != "" {
		limits["memory"] = resource.MustParse(limit)
	}
	if limit := cfg.Launcher.Resources.LimitCPU; limit != "" {
		limits["cpu"] = resource.MustParse(limit)
	}

	// Configuration
	hasTLS := "true"
	if runConfig.ClientCert == "" {
		hasTLS = "false"
	}

	// Pod
	podname := fmt.Sprintf("%s-cc-%s", myself, runConfig.ShortName)
	pod := &apiv1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: podname,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion:         "v1",
					Kind:               "Pod",
					Name:               myselfPod.Name,
					UID:                myselfPod.UID,
					BlockOwnerDeletion: BoolRef(true),
				},
			},
			Labels: map[string]string{
				"externalcc-type": "launcher",
			},
		},
		Spec: apiv1.PodSpec{
			Containers: []apiv1.Container{
				{
					Name:            "chaincode",
					Image:           runConfig.Image,
					ImagePullPolicy: apiv1.PullIfNotPresent,
					Env: []apiv1.EnvVar{
						{
							Name:  "CORE_CHAINCODE_ID_NAME",
							Value: runConfig.CCID,
						},
						{
							Name:  "CORE_PEER_LOCALMSPID",
							Value: runConfig.MSPID,
						},
						{
							Name:  "CORE_TLS_CLIENT_CERT_PATH",
							Value: "/chaincode/artifacts/client.crt",
						},
						{
							Name:  "CORE_TLS_CLIENT_KEY_PATH",
							Value: "/chaincode/artifacts/client.key",
						},
						{
							Name:  "CORE_TLS_CLIENT_CERT_FILE",
							Value: "/chaincode/artifacts/client_pem.crt",
						},
						{
							Name:  "CORE_TLS_CLIENT_KEY_FILE",
							Value: "/chaincode/artifacts/client_pem.key",
						},
						{
							Name:  "CORE_PEER_TLS_ROOTCERT_FILE",
							Value: "/chaincode/artifacts/root.crt",
						},
						{
							Name:  "CORE_PEER_TLS_ENABLED",
							Value: hasTLS,
						},
					},
					WorkingDir: GetCCMountDir(runConfig.Platform), // Set the CWD to the path where the chaincode is
					Command:    GetRunArgs(runConfig.Platform, runConfig.PeerAddress),
					Resources:  apiv1.ResourceRequirements{Limits: limits},
					VolumeMounts: []apiv1.VolumeMount{
						{
							Name:      "transfer-pv",
							MountPath: "/chaincode/artifacts/",
							SubPath:   transferPVPrefix + "/artifacts/",
							ReadOnly:  true,
						},
						{
							Name:      "transfer-pv",
							MountPath: GetCCMountDir(runConfig.Platform),
							SubPath:   transferPVPrefix + "/output/",
							ReadOnly:  true,
						},
					},
				},
			},
			EnableServiceLinks: BoolRef(false),
			RestartPolicy:      apiv1.RestartPolicyAlways,
			Volumes: []apiv1.Volume{
				{
					Name: "transfer-pv",
					VolumeSource: apiv1.VolumeSource{
						PersistentVolumeClaim: &apiv1.PersistentVolumeClaimVolumeSource{
							ClaimName: cfg.TransferVolume.Claim,
						},
					},
				},
			},
		},
	}

	return clientset.CoreV1().Pods(cfg.Namespace).Create(ctx, pod, metav1.CreateOptions{})
}
