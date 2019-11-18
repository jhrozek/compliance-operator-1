package complianceremediation

import (
	"context"
	"fmt"
	complianceoperatorv1alpha1 "github.com/openshift/compliance-operator/pkg/apis/complianceoperator/v1alpha1"
	"github.com/openshift/compliance-operator/pkg/controller/complianceremediation/machineconfig"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var log = logf.Log.WithName("controller_complianceremediation")

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new ComplianceRemediation Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	mcw, err := machineconfig.NewMachineConfigDynWrapper()
	if err != nil {
		log.Error(err, "Cannot create MachineConfigWrapper")
	}

	return &ReconcileComplianceRemediation{client: mgr.GetClient(),
										   scheme: mgr.GetScheme(),
										   mcWrapper: mcw}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("complianceremediation-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource ComplianceRemediation
	err = c.Watch(&source.Kind{Type: &complianceoperatorv1alpha1.ComplianceRemediation{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// Watch for changes to secondary resource Pods and requeue the owner ComplianceRemediation
	err = c.Watch(&source.Kind{Type: &corev1.Pod{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &complianceoperatorv1alpha1.ComplianceRemediation{},
	})
	if err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcileComplianceRemediation implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileComplianceRemediation{}

// ReconcileComplianceRemediation reconciles a ComplianceRemediation object
type ReconcileComplianceRemediation struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
	mcWrapper *machineconfig.MachineConfigDynWrapper
}

// Reconcile reads that state of the cluster for a ComplianceRemediation object and makes changes based on the state read
// and what is in the ComplianceRemediation.Spec
// a Pod as an example
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileComplianceRemediation) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)


	if r.mcWrapper == nil {
		reqLogger.Error(fmt.Errorf("Missing machineConfigWrapper"), "Required field missing")
		return reconcile.Result{}, nil
	}

	// Fetch the ComplianceRemediation instance
	remediationInstance := &complianceoperatorv1alpha1.ComplianceRemediation{}
	err := r.client.Get(context.TODO(), request.NamespacedName, remediationInstance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}
	reqLogger.Info("Reconciling ComplianceRemediation",
		"ComplianceRemediation name", remediationInstance.Name)

	// TODO: convert the name into $priority-$name?

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

	_, err = machineconfig.YamlToMachineConfig([]byte(mcoSpec))
	if err != nil {
		reqLogger.Error(err, "Cannot convert")
	}

	exists, err := r.mcWrapper.ExistsByName(remediationInstance.Name)
	if err != nil {
		return reconcile.Result{}, err
	}
	if exists {
		reqLogger.Info("This MachineConfig object already exists, doing nothing")
		return reconcile.Result{}, nil
	}

	contents, err := getRemediationContents(remediationInstance.Spec.Annotation)
	if err != nil {
		return reconcile.Result{}, err
	}

	newMc, err := mergeContentsToMachineConfig(contents)
	if err != nil {
		return reconcile.Result{}, err
	}

	err = createMachineConfig(newMc)
	if err != nil {
		return reconcile.Result{}, err
	}

	// Return and don't requeue
	return reconcile.Result{}, nil
}

func getRemediationContents(label string) ([]string, error) {
	contents := make([]string, 0)
	//err = r.client.List(context.TODO(), types.NamespacedName{Name: remediationInstance.Name}, mcInstance)
	return contents, nil
}

func mergeContentsToMachineConfig(contents []string) (*machineconfig.MachineConfig, error) {
	return &machineconfig.MachineConfig{}, nil
}

func createMachineConfig(config *machineconfig.MachineConfig) error {
	return nil
}
