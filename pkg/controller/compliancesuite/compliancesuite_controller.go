package compliancesuite

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"fmt"
	"github.com/go-logr/logr"
	complianceoperatorv1alpha1 "github.com/openshift/compliance-operator/pkg/apis/complianceoperator/v1alpha1"
	"github.com/openshift/compliance-operator/pkg/utils"
	"io/ioutil"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
	"strings"
)

var log = logf.Log.WithName("controller_compliancesuite")

const (
	configMapRemediationsProcessed = "compliance-remediations/processed"
	configMapCompressed            = "openscap-scan-result/compressed"
	nodeRolePrefix                 = "node-role.kubernetes.io/"
)

// Add creates a new ComplianceSuite Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileComplianceSuite{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("compliancesuite-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource ComplianceSuite
	err = c.Watch(&source.Kind{Type: &complianceoperatorv1alpha1.ComplianceSuite{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// Watch for changes to secondary resource ComplianceScans and requeue the owner ComplianceSuite
	err = c.Watch(&source.Kind{Type: &complianceoperatorv1alpha1.ComplianceScan{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &complianceoperatorv1alpha1.ComplianceSuite{},
	})
	if err != nil {
		return err
	}

	// Watch for changes to secondary resource ComplianceRemediation and requeue the owner ComplianceSuite
	err = c.Watch(&source.Kind{Type: &complianceoperatorv1alpha1.ComplianceRemediation{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &complianceoperatorv1alpha1.ComplianceSuite{},
	})
	if err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcileComplianceSuite implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileComplianceSuite{}

// ReconcileComplianceSuite reconciles a ComplianceSuite object
type ReconcileComplianceSuite struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for a ComplianceSuite object and makes changes based on the state read
// and what is in the ComplianceSuite.Spec
// TODO(user): Modify this Reconcile function to implement your Controller logic.  This example creates
// a Pod as an example
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileComplianceSuite) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling ComplianceSuite")

	// Fetch the ComplianceSuite suite
	suite := &complianceoperatorv1alpha1.ComplianceSuite{}
	err := r.client.Get(context.TODO(), request.NamespacedName, suite)
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
	reqLogger.Info("Retrieved suite", "suite", suite)

	if err := r.reconcileScans(suite, reqLogger); err != nil {
		// TODO: Not all errors are recoverable..
		return reconcile.Result{}, err
	}

	if err := r.reconcileRemediations(request.NamespacedName, reqLogger); err != nil {
		// TODO: Not all errors are recoverable..
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

func (r *ReconcileComplianceSuite) reconcileScans(suite *complianceoperatorv1alpha1.ComplianceSuite, logger logr.Logger) error {
	for _, scanWrap := range suite.Spec.Scans {
		// The scans contain a nodeSelector that ultimately must match a machineConfigPool. The only way we can
		// ensure it does is by checking if the nodeSelector contains a label with the key
		//"node-role.kubernetes.io/XXX". Then the matching Pool would be labeled with
		//"machineconfiguration.openshift.io/role: XXX".
		//See also: https://github.com/openshift/machine-config-operator/blob/master/docs/custom-pools.md
		if getScanRoleLabel(scanWrap.NodeSelector) == "" {
			logger.Info("Not scheduling scan without a role label", "scan", scanWrap.Name)
			continue
		}

		scan := &complianceoperatorv1alpha1.ComplianceScan{}
		err := r.client.Get(context.TODO(), types.NamespacedName{Name: scanWrap.Name, Namespace: suite.Namespace}, scan)
		if err != nil && errors.IsNotFound(err) {
			// If the scan was not found, launch it
			logger.Info("Scan not found, launching..", "scan", scanWrap.Name)
			if err = launchScanForSuite(r, suite, &scanWrap, logger); err != nil {
				return err
			}
			logger.Info("Scan created", "scan", scanWrap.Name)
			// No point in reconciling status yet
			continue
		} else if err != nil {
			return err
		}

		// The scan already exists, let's just make sure its status is reflected
		if err := r.reconcileScanStatus(suite, scan, logger); err != nil {
			return err
		}
	}

	return nil
}

func getScanRoleLabel(nodeSelector map[string]string) string {
	if nodeSelector == nil {
		return ""
	}

	// FIXME: should we protect against multiple labels and return
	// an empty string if there are multiple?
	for k := range nodeSelector {
		if strings.HasPrefix(k, nodeRolePrefix) {
			return strings.TrimPrefix(k, nodeRolePrefix)
		}
	}

	return ""
}

func (r *ReconcileComplianceSuite) reconcileScanStatus(suite *complianceoperatorv1alpha1.ComplianceSuite, scan *complianceoperatorv1alpha1.ComplianceScan, logger logr.Logger) error {
	// See if we already have a ScanStatusWrapper for this name
	for idx, scanStatusWrap := range suite.Status.ScanStatuses {
		if scan.Name == scanStatusWrap.Name {
			logger.Info("About to update scan status", "scan", scan.Name)
			err := r.updateScanStatus(suite, idx, &scanStatusWrap, scan, logger)
			if err != nil {
				logger.Error(err, "Could not update scan status")
				return err
			}
			return nil
		}
	}

	logger.Info("About to add scan status", "scan", scan.Name)
	err := r.addScanStatus(suite, scan, logger)
	if err != nil {
		logger.Error(err, "Could not add scan status")
		return err
	}
	return nil
}

func (r *ReconcileComplianceSuite) updateScanStatus(suite *complianceoperatorv1alpha1.ComplianceSuite, idx int, scanStatusWrap *complianceoperatorv1alpha1.ComplianceScanStatusWrapper, scan *complianceoperatorv1alpha1.ComplianceScan, logger logr.Logger) error {
	// if yes, update it, if the status differs
	if scanStatusWrap.Phase == scan.Status.Phase {
		logger.Info("Not updating scan, the phase is the same", "scan", scanStatusWrap.Name, "phase", scanStatusWrap.Phase)
		return nil
	}

	// FIXME: Should we treat this as non-fatal (as we do now?) Or at least
	// propagate the error to the caller?
	if scan.Status.Phase == complianceoperatorv1alpha1.PhaseDone {
		err := r.reconcileScanRemediations(suite, scan, logger)
		if err != nil {
			logger.Error(err, "Error reconciling remediations")
		}
	}

	modScanStatus := complianceoperatorv1alpha1.ComplianceScanStatusWrapper{
		ComplianceScanStatus: complianceoperatorv1alpha1.ComplianceScanStatus{
			Phase: scan.Status.Phase,
		},
		Name: scan.Name,
	}

	suiteCopy := suite.DeepCopy()
	suiteCopy.Status.ScanStatuses[idx] = modScanStatus
	logger.Info("Updating scan status", "scan", modScanStatus.Name, "phase", modScanStatus.Phase)

	return r.client.Status().Update(context.TODO(), suiteCopy)
}

func (r *ReconcileComplianceSuite) reconcileScanRemediations(suite *complianceoperatorv1alpha1.ComplianceSuite, scan *complianceoperatorv1alpha1.ComplianceScan, logger logr.Logger) error {
	var cMapList v1.ConfigMapList
	var scanRemediations []*complianceoperatorv1alpha1.ComplianceRemediation

	// Look for configMap with this scan label
	listOpts := client.ListOptions{
		LabelSelector: labels.SelectorFromSet(labels.Set{"compliance-scan": scan.Name}),
	}

	if err := r.client.List(context.TODO(), &cMapList, &listOpts); err != nil {
		return err
	}
	logger.Info("Scan has results", "scan", scan.Name)

	for _, cm := range cMapList.Items {
		resultRemediations, err := parseResultRemediations(r, scan.Name, scan.Namespace, &cm, logger)
		if err != nil {
			logger.Error(err, "cannot parse scan result")
			return err
		} else if resultRemediations == nil {
			logger.Info("Either no remediations found in result or result already processed")
			// Already processed
			continue
		}

		// Annotate the configmap, we want to avoid parsing it next time the reconcile
		// loop hits
		err = updateResultsConfigMap(r, &cm)
		if err != nil {
			logger.Error(err, "Cannot annotate the CM")
			return err
		}

		logger.Info("Parsed out remediations for CM", "remediations", resultRemediations, "scan", scan.Name)
		// If there are any remediations, make sure all of them for the scan are
		// exactly the same
		if scanRemediations == nil {
			// This is the first loop or only result
			logger.Info("This is the first remediation list, keeping it")
			scanRemediations = resultRemediations
		} else {
			// All remediation lists in the scan must be equal
			ok := diffRemediationList(scanRemediations, resultRemediations)
			if !ok {
				return fmt.Errorf("the list of remediations differs")
			}
		}
	}

	// At this point either scanRemediations is nil or contains a list
	// of remediations for this scan
	// Create the remediations
	logger.Info("Creating remediation objects", "remediations", scanRemediations)
	if err := createRemediations(r, suite, scan, scanRemediations, logger); err != nil {
		return err
	}

	return nil
}

func parseResultRemediations(r *ReconcileComplianceSuite, scanName string, namespace string, cm *v1.ConfigMap, logger logr.Logger) ([]*complianceoperatorv1alpha1.ComplianceRemediation, error) {
	var scanResult string
	var err error

	_, ok := cm.Annotations[configMapRemediationsProcessed]
	if ok {
		logger.Info("ConfigMap already processed", "map", cm.Name)
		return nil, nil
	}

	scanResult, ok = cm.Data["results"]
	if !ok {
		return nil, fmt.Errorf("no results in configmap %s", cm.Name)
	}

	_, ok = cm.Annotations[configMapCompressed]
	if ok {
		logger.Info("Results are compressed\n")
		scanResult, err = readCompressedData(scanResult)
		if err != nil {
			return nil, err
		}
	}

	return utils.ParseRemediationsFromArf(r.scheme, scanName, namespace, scanResult)
}

func updateResultsConfigMap(r *ReconcileComplianceSuite, cm *v1.ConfigMap) error {
	cmCopy := cm.DeepCopy()

	if cmCopy.Annotations == nil {
		cmCopy.Annotations = make(map[string]string)
	}
	cmCopy.Annotations[configMapRemediationsProcessed] = ""

	return r.client.Update(context.TODO(), cmCopy)
}

func createRemediations(r *ReconcileComplianceSuite, suite *complianceoperatorv1alpha1.ComplianceSuite, scan *complianceoperatorv1alpha1.ComplianceScan, remediations []*complianceoperatorv1alpha1.ComplianceRemediation, logger logr.Logger) error {
	for _, rem := range remediations {
		logger.Info("Creating remediation", "rem", rem.Name)
		if err := controllerutil.SetControllerReference(suite, rem, r.scheme); err != nil {
			log.Error(err, "Failed to set remediation ownership", "rem", rem)
			return err
		}

		if rem.Labels == nil {
			rem.Labels = make(map[string]string)
		}
		rem.Labels["complianceoperator.openshift.io/suite"] = suite.Name
		rem.Labels["complianceoperator.openshift.io/scan"] = scan.Name
		rem.Labels["machineconfiguration.openshift.io/role"] = getScanRoleLabel(scan.Spec.NodeSelector)
		if rem.Labels["machineconfiguration.openshift.io/role"] == "" {
			return fmt.Errorf("scan %s has no role assignment", scan.Name)
		}

		err := r.client.Create(context.TODO(), rem)
		if err != nil {
			log.Error(err, "Failed to create remediation", "rem", rem)
			return err
		}
	}

	return nil
}

func readCompressedData(compressed string) (string, error) {
	decoded, err := base64.StdEncoding.DecodeString(compressed)
	if err != nil {
		return "", err
	}

	compressedReader := bytes.NewReader(decoded)
	gzReader, err := gzip.NewReader(compressedReader)
	if err != nil {
		return "", err
	}
	defer gzReader.Close()

	// FIXME: probably unsafe, see https://haisum.github.io/2017/09/11/golang-ioutil-readall/
	b, err := ioutil.ReadAll(gzReader)
	if err != nil {
		return "", err
	}

	return string(b), nil
}

func (r *ReconcileComplianceSuite) addScanStatus(suite *complianceoperatorv1alpha1.ComplianceSuite, scan *complianceoperatorv1alpha1.ComplianceScan, logger logr.Logger) error {
	// if not, create the scan status with the name and the current state
	newScanStatus := complianceoperatorv1alpha1.ComplianceScanStatusWrapper{
		ComplianceScanStatus: complianceoperatorv1alpha1.ComplianceScanStatus{
			Phase: scan.Status.Phase,
		},
		Name: scan.Name,
	}

	suiteCopy := suite.DeepCopy()
	suiteCopy.Status.ScanStatuses = append(suite.Status.ScanStatuses, newScanStatus)
	logger.Info("Adding scan status", "scan", newScanStatus.Name, "phase", newScanStatus.Phase)
	return r.client.Status().Update(context.TODO(), suiteCopy)
}

func launchScanForSuite(r *ReconcileComplianceSuite, suite *complianceoperatorv1alpha1.ComplianceSuite, scanWrap *complianceoperatorv1alpha1.ComplianceScanSpecWrapper, logger logr.Logger) error {
	scan := newScanForSuite(suite, scanWrap)
	if scan == nil {
		return fmt.Errorf("cannot create ComplianceScan for %s:%s", suite.Name, scanWrap.Name)
	}

	if err := controllerutil.SetControllerReference(suite, scan, r.scheme); err != nil {
		log.Error(err, "Failed to set scan ownership", "scan", scan)
		return err
	}

	logger.Info("About to launch scan", "scan", scan)
	err := r.client.Create(context.TODO(), scan)
	if errors.IsAlreadyExists(err) {
		// Was there a race that created the scan in the meantime?
		return nil
	} else if err != nil {
		return err
	}

	return nil
}

func newScanForSuite(suite *complianceoperatorv1alpha1.ComplianceSuite, scanWrap *complianceoperatorv1alpha1.ComplianceScanSpecWrapper) *complianceoperatorv1alpha1.ComplianceScan {
	return &complianceoperatorv1alpha1.ComplianceScan{
		ObjectMeta: metav1.ObjectMeta{
			Name:      scanWrap.Name,
			Namespace: suite.Namespace,
		},
		Spec: complianceoperatorv1alpha1.ComplianceScanSpec{
			ContentImage: scanWrap.ContentImage,
			Profile:      scanWrap.Profile,
			Rule:         scanWrap.Rule,
			Content:      scanWrap.Content,
			NodeSelector: scanWrap.NodeSelector,
		},
	}
}

func diffRemediationList(oldList, newList []*complianceoperatorv1alpha1.ComplianceRemediation) bool {
	if newList == nil {
		return oldList == nil
	}

	if len(newList) != len(oldList) {
		return false
	}

	// Let's implement the rest later..
	return true
}

func (r *ReconcileComplianceSuite) reconcileRemediations(namespacedName types.NamespacedName, logger logr.Logger) error {
	// Get the suite again, it might have been changed earlier during the scan status change
	suite := &complianceoperatorv1alpha1.ComplianceSuite{}
	err := r.client.Get(context.TODO(), namespacedName, suite)
	if err != nil {
		return err
	}

	// Get all the remediations
	var remList complianceoperatorv1alpha1.ComplianceRemediationList
	listOpts := client.ListOptions{
		LabelSelector: labels.SelectorFromSet(labels.Set{"complianceoperator.openshift.io/suite": suite.Name}),
	}

	if err := r.client.List(context.TODO(), &remList, &listOpts); err != nil {
		return err
	}

	// Construct the list of the statuses
	remOverview := make([]complianceoperatorv1alpha1.ComplianceRemediationNameStatus, len(remList.Items))
	for idx, rem := range remList.Items {
		remOverview[idx].ScanName = rem.Labels["complianceoperator.openshift.io/scan"]
		remOverview[idx].RemediationName = rem.Name
		remOverview[idx].Type = rem.Spec.Type
		remOverview[idx].Apply = rem.Spec.Apply
	}

	// Update the suite status
	suiteCopy := suite.DeepCopy()
	suiteCopy.Status.RemediationOverview = remOverview
	logger.Info("Remediations", "remOverview", remOverview)
	return r.client.Status().Update(context.TODO(), suiteCopy)
}
