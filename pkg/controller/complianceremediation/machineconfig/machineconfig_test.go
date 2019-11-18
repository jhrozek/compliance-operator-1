package machineconfig

import "testing"

func TestMarshal(t *testing.T) {
	mcoSpec :=`
apiVersion: machineconfiguration.openshift.io/v1
kind: MachineConfig
metadata:
  name: worker-2864988432
spec:
  config:
    ignition:
      version: 2.2.0
    storage:
      files:
      - contents:
          source: data:,%20
        filesystem: root
        mode: 384
        path: /root/myfile
`


	_, err := YamlToMachineConfig([]byte(mcoSpec))
	if err != nil {
		t.Errorf("Unexpected error %v", err)
	}
}
