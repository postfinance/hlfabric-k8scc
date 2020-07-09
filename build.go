package main

import (
	"context"
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

// Build builds a chaincode on Kubernetes
func Build(ctx context.Context, cfg Config) error {
	log.Println("Procedure: build")

	if len(os.Args) != 4 {
		return errors.New("build requires exactly three arguments")
	}

	sourceDir := os.Args[1]
	metadataDir := os.Args[2]
	outputDir := os.Args[3]

	// Get metadata
	metadata, err := getMetadata(metadataDir)
	if err != nil {
		return errors.Wrap(err, "getting metadata for chaincode")
	}

	// Create transfer directory
	copyOpts := cpy.Options{AddPermission: os.ModePerm}

	prefix, _ := os.Hostname()
	transferdir, err := ioutil.TempDir(cfg.TransferVolume.Path, prefix)
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("creating directory %s on transfer volume", cfg.TransferVolume.Path))
	}
	defer os.RemoveAll(transferdir) // Cleanup transfer directory when this process ends

	// Setup transfer
	transferSrc := filepath.Join(transferdir, "src")
	transferSrcMeta := filepath.Join(sourceDir, "META-INF")
	transferBld := filepath.Join(transferdir, "bld")
	buildInfoFile := filepath.Join(outputDir, "k8scc_buildinfo.json")

	// Copy source
	err = cpy.Copy(sourceDir, transferSrc, copyOpts)
	if err != nil {
		return errors.Wrap(err, "copy source dir in the transfer dir")
	}

	// Create output directory
	err = os.Mkdir(transferBld, os.ModePerm)
	if err != nil {
		return errors.Wrap(err, "create output dir in the transfer dir")
	}
	err = os.Chmod(transferBld, os.ModePerm)
	if err != nil {
		return errors.Wrap(err, "chmod on output dir in the transfer dir")
	}

	// Create builder Pod
	pod, err := createBuilderPod(ctx, cfg, metadata, filepath.Base(transferdir))
	if err != nil {
		return errors.Wrap(err, "creating builder pod")
	}
	defer cleanupPodSilent(pod)

	// Watch builder Pod for completion or failure
	podSucceeded, err := watchPodUntilCompletion(ctx, pod)
	if err != nil {
		return errors.Wrap(err, "watching builder pod")
	}

	if !podSucceeded {
		return fmt.Errorf("build of Chaincode %s in Pod %s failed", metadata.Label, pod.Name)
	}

	// Copy data from transfer pv to original output destination
	err = cpy.Copy(transferBld, outputDir)
	if err != nil {
		return errors.Wrap(err, "copy build artifacts from transfer")
	}

	// Copy META-INF, if available
	if _, err := os.Stat(transferSrcMeta); !os.IsNotExist(err) {
		err = cpy.Copy(transferSrcMeta, outputDir)
		if err != nil {
			return errors.Wrap(err, "copy META-INF to output dir")
		}
	}

	// Create build information
	buildInformation := BuildInformation{
		Image:    pod.Spec.Containers[0].Image,
		Platform: metadata.Type,
	}

	bi, err := json.Marshal(buildInformation)
	if err != nil {
		return errors.Wrap(err, "marshaling BuildInformation")
	}

	err = ioutil.WriteFile(buildInfoFile, bi, os.ModePerm)
	if err != nil {
		return errors.Wrap(err, "writing BuildInformation")
	}

	err = os.Chmod(buildInfoFile, os.ModePerm)
	if err != nil {
		return errors.Wrap(err, "changing permissions of BuildInformation")
	}

	return nil
}

func createBuilderPod(ctx context.Context,
	cfg Config, metadata *ChaincodeMetadata, transferPVPrefix string) (*apiv1.Pod, error) {
	// Setup kubernetes client
	clientset, err := getKubernetesClientset()
	if err != nil {
		return nil, errors.Wrap(err, "getting kubernetes clientset")
	}

	// Get builder image
	image, ok := cfg.Images[metadata.Type]
	if !ok {
		return nil, fmt.Errorf("no builder image available for %q", metadata.Type)
	}

	// Get platform informations from hyperledger
	plt := GetPlatform(metadata.Type)
	if plt == nil {
		return nil, fmt.Errorf("platform %q not supported by Hyperledger Fabric", metadata.Type)
	}

	buildOpts, err := plt.DockerBuildOptions(metadata.Path)
	if err != nil {
		return nil, errors.Wrap(err, "getting build options for platform")
	}

	envvars := []apiv1.EnvVar{}
	for _, env := range buildOpts.Env {
		s := strings.SplitN(env, "=", 2)
		envvars = append(envvars, apiv1.EnvVar{
			Name:  s[0],
			Value: s[1],
		})
	}

	// Get peer Pod
	myself, _ := os.Hostname()
	myselfPod, err := clientset.CoreV1().Pods(cfg.Namespace).Get(ctx, myself, metav1.GetOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "getting myself Pod")
	}

	// Set resources
	limits := apiv1.ResourceList{}
	if limit := cfg.Builder.Resources.LimitMemory; limit != "" {
		limits["memory"] = resource.MustParse(limit)
	}
	if limit := cfg.Builder.Resources.LimitCPU; limit != "" {
		limits["cpu"] = resource.MustParse(limit)
	}

	// Pod
	podname := fmt.Sprintf("%s-ccbuild-%s", myself, metadata.MetadataID)
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
				"externalcc-type": "builder",
			},
		},
		Spec: apiv1.PodSpec{
			Containers: []apiv1.Container{
				{
					Name:            "builder",
					Image:           image,
					ImagePullPolicy: apiv1.PullIfNotPresent,
					Command: []string{
						"/bin/sh", "-c", buildOpts.Cmd,
					},
					Env:       envvars,
					Resources: apiv1.ResourceRequirements{Limits: limits},
					VolumeMounts: []apiv1.VolumeMount{
						{
							Name:      "transfer-pv",
							MountPath: "/chaincode/input/",
							SubPath:   transferPVPrefix + "/src/",
							ReadOnly:  true,
						},
						{
							Name:      "transfer-pv",
							MountPath: "/chaincode/output/",
							SubPath:   transferPVPrefix + "/bld/",
							ReadOnly:  false,
						},
					},
				},
			},
			EnableServiceLinks: BoolRef(false),
			RestartPolicy:      apiv1.RestartPolicyNever,
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
