package machineconfig

import (
	"fmt"
	igntypes "github.com/coreos/ignition/config/v2_2/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"os"
		"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
)

// MachineConfig defines the configuration for a machine
type MachineConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec MachineConfigSpec `json:"spec"`
}

// MachineConfigSpec is the spec for MachineConfig
type MachineConfigSpec struct {
	// OSImageURL specifies the remote location that will be used to
	// fetch the OS.
	OSImageURL string `json:"osImageURL"`
	// Config is a Ignition Config object.
	Config igntypes.Config `json:"config"`

	KernelArguments []string `json:"kernelArguments"`

	FIPS bool `json:"fips"`
}

// MachineConfigList is a list of MachineConfig resources
type MachineConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []MachineConfig `json:"items"`
}

var (
	mcsGVR = schema.GroupVersionResource{
		Group:    "machineconfiguration.openshift.io",
		Version:  "v1",
		Resource: "machineconfigs",
	}
	mcsGVK = schema.GroupVersionKind{
		Group:    "machineconfiguration.openshift.io",
		Version:  "v1",
		Kind: "MachineConfig",
	}
)

type MachineConfigDynWrapper struct {
	dynClient dynamic.Interface
}

func (mcw *MachineConfigDynWrapper) ExistsByName(name string) (bool, error) {
	unstructuredObject, err := mcw.dynClient.Resource(mcsGVR).Get(name, metav1.GetOptions{})
	if err != nil {
		return false, err
	}

	if unstructuredObject != nil {
		return true, nil
	}
	return false, nil
}

func (mcw *MachineConfigDynWrapper) List() {
	mcs, _ := mcw.dynClient.Resource(mcsGVR).List(metav1.ListOptions{})
	for _, mc := range mcs.Items {
		fmt.Println("BEGIN")
		fmt.Printf("MachineConfig: %v\n\n", mc)
		fmt.Println("END")
	}
}

func NewMachineConfigDynWrapper() (*MachineConfigDynWrapper, error) {
	var config *rest.Config
	var err error

	wrapper := MachineConfigDynWrapper{}

	if os.Getenv("KUBERNETES_PORT") != "" {
		config, err = rest.InClusterConfig()
	} else {
		kubeconfig := os.Getenv("KUBECONFIG")
		if kubeconfig == "" {
			return nil, fmt.Errorf("running outside of a cluster and no KUBECONFIG provided")
		}
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
	}
	if err != nil {
		return nil, err
	}

	wrapper.dynClient, err = dynamic.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return &wrapper, nil
}

var (
	mcfgScheme = runtime.NewScheme()
	mcfgCodecs = serializer.NewCodecFactory(mcfgScheme)
	GroupName = "machineconfiguration.openshift.io"
	GroupVersion  = schema.GroupVersion{Group: GroupName, Version: "v1"}
)

func YamlToMachineConfig(in []byte) (*MachineConfig, error) {
	m, err := runtime.Decode(mcfgCodecs.UniversalDecoder(GroupVersion), in)
	if err != nil {
		return nil, err
	}

	fmt.Printf("%#v\n", m)
	return &MachineConfig{}, nil
}
