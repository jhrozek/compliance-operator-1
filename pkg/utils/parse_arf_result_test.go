package utils

import (
	igntypes "github.com/coreos/ignition/config/v2_2/types"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	complianceoperatorv1alpha1 "github.com/openshift/compliance-operator/pkg/apis/complianceoperator/v1alpha1"
	mcfgv1 "github.com/openshift/compliance-operator/pkg/apis/machineconfiguration/v1"
	"io/ioutil"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
)

var _ = Describe("ARF parser", func() {
	var (
		arf             []byte
		schema          *runtime.Scheme
		resultsFilename string
		remList         []*complianceoperatorv1alpha1.ComplianceRemediation
		err             error
	)

	BeforeEach(func() {
		mcInstance := &mcfgv1.MachineConfig{}
		schema = scheme.Scheme
		schema.AddKnownTypes(mcfgv1.SchemeGroupVersion, mcInstance)
		resultsFilename = "results.arf.xml"
	})

	JustBeforeEach(func() {
		arf, err = ioutil.ReadFile(resultsFilename)
		Expect(err).NotTo(HaveOccurred())
		remList, err = ParseRemediationsFromArf(schema, "testScan", "testNamespace", string(arf))
	})

	Describe("Load the ARF", func() {
		Context("Valid ARF with one remediation", func() {
			It("Should parse the ARF without errors", func() {
				Expect(err).NotTo(HaveOccurred())
			})
			It("Should return exactly one remediation", func() {
				Expect(remList).To(HaveLen(1))
			})
		})

		Context("First remediation type", func() {
			var (
				rem     *complianceoperatorv1alpha1.ComplianceRemediation
				expName string
			)

			BeforeEach(func() {
				rem = remList[0]
				expName = "testScan-no-empty-passwords"
			})

			It("Should have the expected name", func() {
				Expect(rem.Name).To(Equal(expName))
			})
			It("Should be a MC", func() {
				Expect(rem.Spec.Type).To(Equal(complianceoperatorv1alpha1.McRemediation))
			})

			Context("MC files", func() {
				var (
					mcFiles []igntypes.File
				)

				BeforeEach(func() {
					mcFiles = rem.Spec.MachineConfigContents.Spec.Config.Storage.Files
				})

				It("Should define two files", func() {
					Expect(mcFiles).To(HaveLen(2))
				})
				It("Should define the expected files", func() {
					Expect(mcFiles[0].Path).To(Equal("/etc/pam.d/password-auth"))
					Expect(mcFiles[1].Path).To(Equal("/etc/pam.d/system-auth"))
				})
			})
			// FIXME: maybe define Equal methods on the type and use go-cmp/cmp ?
		})
	})
})
