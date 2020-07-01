package main

import (
	"bufio"
	"context"
	"crypto/sha1" // #nosec G505
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"gopkg.in/yaml.v2"

	"github.com/pkg/errors"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
)

const (
	namespaceFile = "/var/run/secrets/kubernetes.io/serviceaccount/namespace"
)

// Procedure implements a Hyperledger Fabric externalbuilders command
type Procedure func(ctx context.Context, cfg Config) error

func main() {
	// Select procedure
	procedures := map[string]Procedure{
		"detect":  Detect,
		"build":   Build,
		"release": Release,
		"run":     Run,
	}

	proc := getProcedureFromArg(procedures)
	if proc == nil {
		log.Fatalln("Please pass one of the following values as first argument" +
			"or set it as the name of the executable: detect, build, release, run")
	}

	// Read configuration
	cfgFile := os.Getenv("K8SCC_CFGFILE")
	if cfgFile == "" {
		cfgFile = "k8scc.yaml"
	}

	cfgData, err := ioutil.ReadFile(cfgFile)
	if err != nil {
		log.Fatalf("Loading configuration file %s: %s", cfgFile, err)
	}

	cfg := Config{}
	err = yaml.Unmarshal(cfgData, &cfg)
	if err != nil {
		log.Fatalf("Parsing configuration: %s", err)
	}

	// Read namespace
	namespace, err := ioutil.ReadFile(namespaceFile)
	if err != nil {
		log.Fatalf("Reading namespace file %s: %s", namespaceFile, err)
	}
	cfg.Namespace = string(namespace)

	// Handle SIGTERM and SIGINT in order to collect garbage.
	// We cancel the request/procedure using a context, so we can cancel it.
	// This will trigger a cleanup in the according functions and we will terminate.
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		sig := <-sigs
		log.Printf("Received %s, stopping launcher", sig)
		cancel()
	}()

	// Run procedure
	err = proc(ctx, cfg)
	if err != nil {
		log.Fatalln(err)
	}
}

func getProcedureFromArg(procs map[string]Procedure) Procedure {
	for argi := 0; argi < len(os.Args) && argi < 2; argi++ {
		function := filepath.Base(os.Args[argi])
		proc, ok := procs[function]
		if ok {
			return proc
		}
	}

	return nil
}

// Config defines the configuration for the Kubernetes chaincode builder and launcher
type Config struct {
	Images         map[string]string `yaml:"images"` // map[technology]image
	TransferVolume struct {
		Path  string `yaml:"path"`
		Claim string `yaml:"claim"`
	} `yaml:"transfer_volume"`

	Builder struct {
		Resources struct {
			LimitMemory string `yaml:"memory_limit"`
			LimitCPU    string `yaml:"cpu_limit"`
		} `yaml:"resources"`
	} `yaml:"builder"`

	Launcher struct {
		Resources struct {
			LimitMemory string `yaml:"memory_limit"`
			LimitCPU    string `yaml:"cpu_limit"`
		} `yaml:"resources"`
	} `yaml:"launcher"`

	// Internal configurations
	Namespace string `yaml:"-"`
}

// BuildInformation is used to serialize build data for consumption by the launcher
type BuildInformation struct {
	Image    string
	Platform string
}

// ChaincodeMetadata is based on
// https://github.com/hyperledger/fabric/blob/v2.0.1/core/chaincode/persistence/chaincode_package.go#L226
type ChaincodeMetadata struct {
	Type       string `json:"type"` // golang, java, node
	Path       string `json:"path"`
	Label      string `json:"label"`
	MetadataID string
}

// ChaincodeRunConfig is based on
// https://github.com/hyperledger/fabric/blob/v2.1.1/core/container/externalbuilder/externalbuilder.go#L335
type ChaincodeRunConfig struct {
	CCID        string `json:"chaincode_id"`
	PeerAddress string `json:"peer_address"`
	ClientCert  string `json:"client_cert"` // PEM encoded client certificate
	ClientKey   string `json:"client_key"`  // PEM encoded client key
	RootCert    string `json:"root_cert"`   // PEM encoded peer chaincode certificate
	MSPID       string `json:"mspid"`

	// Custom fields
	ShortName string
	Image     string
	Platform  string
}

func streamPodLogs(ctx context.Context, pod *apiv1.Pod) error {
	// Setup kubernetes client
	clientset, err := getKubernetesClientset()
	if err != nil {
		return errors.Wrap(err, "getting kubernetes clientset")
	}

	req := clientset.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, &apiv1.PodLogOptions{Follow: true})
	logs, err := req.Stream(ctx)
	if err != nil {
		return errors.Wrap(err, "opening log stream")
	}
	defer logs.Close()

	log.Printf("Start log of pod %s", pod.Name)

	s := bufio.NewScanner(logs)
	for s.Scan() {
		log.Printf("%s: %s", pod.Name, s.Text())
	}

	if err := s.Err(); err != nil {
		log.Println(err)
		log.Printf("%s error: %s", pod.Name, err)
	}

	log.Printf("End log of pod %s", pod.Name)

	return nil
}

func cleanupPodSilent(pod *apiv1.Pod) {
	err := cleanupPod(pod)
	log.Println(err)
}

func cleanupPod(pod *apiv1.Pod) error {
	clientset, err := getKubernetesClientset()
	if err != nil {
		return errors.Wrap(err, "getting kubernetes clientset")
	}

	ctx := context.Background()
	err = clientset.CoreV1().Pods(pod.Namespace).Delete(ctx, pod.Name, metav1.DeleteOptions{})
	return err
}

func watchPodUntilCompletion(ctx context.Context, pod *apiv1.Pod) (bool, error) {
	// Setup kubernetes client
	clientset, err := getKubernetesClientset()
	if err != nil {
		return false, errors.Wrap(err, "getting kubernetes clientset")
	}

	/* Create log attacher
	var attachOnce sync.Once
	attachLogs := func() {
		go func() {
			err := streamPodLogs(pod)
			if err != nil {
				log.Printf("While streaming pod logs: %q", err)
			}
		}()
	}*/

	// Create informer
	factory := informers.NewSharedInformerFactoryWithOptions(clientset, 0, informers.WithNamespace(pod.Namespace))
	informer := factory.Core().V1().Pods().Informer()
	c := make(chan struct{})
	defer close(c)

	podSuccessfull := make(chan bool)
	defer close(podSuccessfull)
	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: func(oldPod, newPod interface{}) {
			p := newPod.(*apiv1.Pod)
			if p.Name == pod.Name {
				log.Printf("Received update on pod %s, phase %s", p.Name, p.Status.Phase)
				// TODO: Can we miss an update, so not getting logs?

				switch p.Status.Phase {
				case apiv1.PodSucceeded:
					podSuccessfull <- true
				case apiv1.PodFailed, apiv1.PodUnknown:
					podSuccessfull <- false
				case apiv1.PodPending, apiv1.PodRunning:
					// Do nothing as this state is good
				default:
					podSuccessfull <- false // Unknown phase
				}
			}
		},
		DeleteFunc: func(oldPod interface{}) {
			p := oldPod.(*apiv1.Pod)
			if p.Name == pod.Name {
				log.Printf("Pod %s, phase %s got deleted", p.Name, p.Status.Phase)
				podSuccessfull <- false
			}
		},
	})
	go informer.Run(c)

	// Wait for result of informer and stop it afterwards.
	res := <-podSuccessfull
	c <- struct{}{}

	// Stream logs
	// TODO: This should be done as soon as the pod is running or has an result
	err = streamPodLogs(ctx, pod)
	if err != nil {
		log.Printf("While streaming pod logs: %q", err)
	}

	return res, nil
}

func getMetadata(metadataDir string) (*ChaincodeMetadata, error) {
	metadataFile := filepath.Join(metadataDir, "metadata.json")
	metadataData, err := ioutil.ReadFile(metadataFile)
	if err != nil {
		return nil, errors.Wrap(err, "Reading metadata.json")
	}

	metadata := ChaincodeMetadata{}
	err = json.Unmarshal(metadataData, &metadata)
	if err != nil {
		return nil, errors.Wrap(err, "Unmarshaling metadata.json")
	}

	// Create hash in order to track this CC
	h := sha1.New() // #nosec G401
	_, err = h.Write(metadataData)
	if err != nil {
		return nil, errors.Wrap(err, "hashing metadata")
	}

	metadata.MetadataID = fmt.Sprintf("%x", h.Sum(nil))[0:8]

	return &metadata, nil
}

// BoolRef returns the reference to a boolean
func BoolRef(b bool) *bool {
	return &b
}

func getKubernetesClientset() (*kubernetes.Clientset, error) {
	// Setup kubernetes client
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, errors.Wrap(err, "getting kubernetes in-cluster config")
	}

	clientset, err := kubernetes.NewForConfig(config)
	return clientset, errors.Wrap(err, "creating kubernetes client")
}
