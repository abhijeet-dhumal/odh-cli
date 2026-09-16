package trainer_test

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/opendatahub-io/odh-cli/pkg/lint/check"
	resultpkg "github.com/opendatahub-io/odh-cli/pkg/lint/check/result"
	"github.com/opendatahub-io/odh-cli/pkg/lint/check/testutil"
	"github.com/opendatahub-io/odh-cli/pkg/lint/checks/workloads/trainer"
	"github.com/opendatahub-io/odh-cli/pkg/resources"

	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
)

//nolint:gochecknoglobals
var listKinds = map[schema.GroupVersionResource]string{
	resources.TrainJob.GVR():           resources.TrainJob.ListKind(),
	resources.DataScienceCluster.GVR(): resources.DataScienceCluster.ListKind(),
}

func managedTrainerDSC() *unstructured.Unstructured {
	return testutil.NewDSC(map[string]string{"trainer": "Managed"})
}

func newTrainJob(name, namespace string, spec map[string]any) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": resources.TrainJob.APIVersion(),
			"kind":       resources.TrainJob.Kind,
			"metadata": map[string]any{
				"name":      name,
				"namespace": namespace,
			},
			"spec": spec,
		},
	}
}

func TestPodTemplateOverridesCheck_Metadata(t *testing.T) {
	g := NewWithT(t)

	chk := trainer.NewPodTemplateOverridesCheck()

	g.Expect(chk.ID()).To(Equal("workloads.trainer.podtemplateoverrides"))
	g.Expect(chk.Name()).To(Equal("Workloads :: Trainer :: PodTemplateOverrides (3.6+)"))
	g.Expect(chk.Group()).To(Equal(check.GroupWorkload))
	g.Expect(chk.CheckKind()).To(Equal("trainer"))
	g.Expect(chk.Description()).ToNot(BeEmpty())
}

func TestPodTemplateOverridesCheck_CanApply_TargetBelow36(t *testing.T) {
	g := NewWithT(t)

	target := testutil.NewTarget(t, testutil.TargetConfig{
		ListKinds:      listKinds,
		Objects:        []*unstructured.Unstructured{managedTrainerDSC()},
		CurrentVersion: "3.5.0",
		TargetVersion:  "3.5.0",
	})

	canApply, err := trainer.NewPodTemplateOverridesCheck().CanApply(t.Context(), target)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(canApply).To(BeFalse())
}

func TestPodTemplateOverridesCheck_CanApply_CurrentAlready36(t *testing.T) {
	g := NewWithT(t)

	target := testutil.NewTarget(t, testutil.TargetConfig{
		ListKinds:      listKinds,
		Objects:        []*unstructured.Unstructured{managedTrainerDSC()},
		CurrentVersion: "3.6.0",
		TargetVersion:  "3.6.0",
	})

	canApply, err := trainer.NewPodTemplateOverridesCheck().CanApply(t.Context(), target)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(canApply).To(BeFalse())
}

func TestPodTemplateOverridesCheck_CanApply_TrainerRemoved(t *testing.T) {
	g := NewWithT(t)

	target := testutil.NewTarget(t, testutil.TargetConfig{
		ListKinds: listKinds,
		Objects: []*unstructured.Unstructured{
			testutil.NewDSC(map[string]string{"trainer": "Removed"}),
		},
		CurrentVersion: "3.5.0",
		TargetVersion:  "3.6.0",
	})

	canApply, err := trainer.NewPodTemplateOverridesCheck().CanApply(t.Context(), target)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(canApply).To(BeFalse())
}

func TestPodTemplateOverridesCheck_CanApply_Upgrade35To36_Managed(t *testing.T) {
	g := NewWithT(t)

	target := testutil.NewTarget(t, testutil.TargetConfig{
		ListKinds:      listKinds,
		Objects:        []*unstructured.Unstructured{managedTrainerDSC()},
		CurrentVersion: "3.5.0",
		TargetVersion:  "3.6.0",
	})

	canApply, err := trainer.NewPodTemplateOverridesCheck().CanApply(t.Context(), target)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(canApply).To(BeTrue())
}

func TestPodTemplateOverridesCheck_NoTrainJobs(t *testing.T) {
	g := NewWithT(t)
	ctx := t.Context()

	target := testutil.NewTarget(t, testutil.TargetConfig{
		ListKinds:      listKinds,
		Objects:        []*unstructured.Unstructured{managedTrainerDSC()},
		CurrentVersion: "3.5.0",
		TargetVersion:  "3.6.0",
	})

	result, err := trainer.NewPodTemplateOverridesCheck().Validate(ctx, target)

	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(result.Status.Conditions).To(HaveLen(1))
	g.Expect(result.Status.Conditions[0].Condition).To(MatchFields(IgnoreExtras, Fields{
		"Type":    Equal(trainer.ConditionTypePodTemplateOverridesCompatible),
		"Status":  Equal(metav1.ConditionTrue),
		"Reason":  Equal(check.ReasonVersionCompatible),
		"Message": ContainSubstring("No TrainJob(s) with podTemplateOverrides found"),
	}))
	g.Expect(result.Annotations).To(HaveKeyWithValue(check.AnnotationImpactedWorkloadCount, "0"))
	g.Expect(result.ImpactedObjects).To(BeEmpty())
}

func TestPodTemplateOverridesCheck_TrainJobWithoutOverrides(t *testing.T) {
	g := NewWithT(t)
	ctx := t.Context()

	job := newTrainJob("plain-job", "test-ns", map[string]any{
		"runtimeRef": map[string]any{
			"name": "torch-distributed",
		},
	})

	target := testutil.NewTarget(t, testutil.TargetConfig{
		ListKinds:      listKinds,
		Objects:        []*unstructured.Unstructured{managedTrainerDSC(), job},
		CurrentVersion: "3.5.0",
		TargetVersion:  "3.6.0",
	})

	result, err := trainer.NewPodTemplateOverridesCheck().Validate(ctx, target)

	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(result.Status.Conditions).To(HaveLen(1))
	g.Expect(result.Status.Conditions[0].Condition).To(MatchFields(IgnoreExtras, Fields{
		"Type":   Equal(trainer.ConditionTypePodTemplateOverridesCompatible),
		"Status": Equal(metav1.ConditionTrue),
		"Reason": Equal(check.ReasonVersionCompatible),
	}))
	g.Expect(result.Annotations).To(HaveKeyWithValue(check.AnnotationImpactedWorkloadCount, "0"))
	g.Expect(result.ImpactedObjects).To(BeEmpty())
}

func TestPodTemplateOverridesCheck_TrainJobWithEmptyOverrides(t *testing.T) {
	g := NewWithT(t)
	ctx := t.Context()

	job := newTrainJob("empty-overrides", "test-ns", map[string]any{
		"podTemplateOverrides": []any{},
	})

	target := testutil.NewTarget(t, testutil.TargetConfig{
		ListKinds:      listKinds,
		Objects:        []*unstructured.Unstructured{managedTrainerDSC(), job},
		CurrentVersion: "3.5.0",
		TargetVersion:  "3.6.0",
	})

	result, err := trainer.NewPodTemplateOverridesCheck().Validate(ctx, target)

	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(result.Status.Conditions).To(HaveLen(1))
	g.Expect(result.Status.Conditions[0].Condition).To(MatchFields(IgnoreExtras, Fields{
		"Type":   Equal(trainer.ConditionTypePodTemplateOverridesCompatible),
		"Status": Equal(metav1.ConditionTrue),
		"Reason": Equal(check.ReasonVersionCompatible),
	}))
	g.Expect(result.Annotations).To(HaveKeyWithValue(check.AnnotationImpactedWorkloadCount, "0"))
}

func TestPodTemplateOverridesCheck_TrainJobWithOverrides(t *testing.T) {
	g := NewWithT(t)
	ctx := t.Context()

	job := newTrainJob("legacy-job", "test-ns", map[string]any{
		"podTemplateOverrides": []any{
			map[string]any{
				"targetJobs": []any{"node"},
				"spec": map[string]any{
					"serviceAccountName": "training-sa",
				},
			},
		},
	})

	target := testutil.NewTarget(t, testutil.TargetConfig{
		ListKinds:      listKinds,
		Objects:        []*unstructured.Unstructured{managedTrainerDSC(), job},
		CurrentVersion: "3.5.0",
		TargetVersion:  "3.6.0",
	})

	result, err := trainer.NewPodTemplateOverridesCheck().Validate(ctx, target)

	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(result.Status.Conditions).To(HaveLen(1))
	g.Expect(result.Status.Conditions[0].Condition).To(MatchFields(IgnoreExtras, Fields{
		"Type":   Equal(trainer.ConditionTypePodTemplateOverridesCompatible),
		"Status": Equal(metav1.ConditionFalse),
		"Reason": Equal(check.ReasonWorkloadsImpacted),
		"Message": And(
			ContainSubstring("Found 1 TrainJob(s) that use podTemplateOverrides"),
			ContainSubstring("We recommend waiting for these jobs to complete before upgrading when possible"),
			ContainSubstring("recreate TrainJobs using runtimePatches"),
			ContainSubstring("https://access.redhat.com/articles/7146204"),
		),
	}))
	g.Expect(result.Status.Conditions[0].Impact).To(Equal(resultpkg.ImpactAdvisory))
	g.Expect(result.Annotations).To(HaveKeyWithValue(check.AnnotationImpactedWorkloadCount, "1"))
	g.Expect(result.ImpactedObjects).To(HaveLen(1))
	g.Expect(result.ImpactedObjects[0].Name).To(Equal("legacy-job"))
	g.Expect(result.ImpactedObjects[0].Namespace).To(Equal("test-ns"))
}

func TestPodTemplateOverridesCheck_MixedTrainJobs(t *testing.T) {
	g := NewWithT(t)
	ctx := t.Context()

	legacyJob := newTrainJob("legacy-job", "ns1", map[string]any{
		"podTemplateOverrides": []any{
			map[string]any{
				"targetJobs": []any{"node"},
			},
		},
	})
	plainJob := newTrainJob("plain-job", "ns1", map[string]any{
		"runtimeRef": map[string]any{"name": "torch-distributed"},
	})

	target := testutil.NewTarget(t, testutil.TargetConfig{
		ListKinds:      listKinds,
		Objects:        []*unstructured.Unstructured{managedTrainerDSC(), legacyJob, plainJob},
		CurrentVersion: "3.5.0",
		TargetVersion:  "3.6.0",
	})

	result, err := trainer.NewPodTemplateOverridesCheck().Validate(ctx, target)

	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(result.Status.Conditions).To(HaveLen(1))
	g.Expect(result.Status.Conditions[0].Condition).To(MatchFields(IgnoreExtras, Fields{
		"Type":    Equal(trainer.ConditionTypePodTemplateOverridesCompatible),
		"Status":  Equal(metav1.ConditionFalse),
		"Message": ContainSubstring("Found 1 TrainJob(s) that use podTemplateOverrides"),
	}))
	g.Expect(result.Annotations).To(HaveKeyWithValue(check.AnnotationImpactedWorkloadCount, "1"))
	g.Expect(result.ImpactedObjects).To(HaveLen(1))
	g.Expect(result.ImpactedObjects[0].Name).To(Equal("legacy-job"))
}

//nolint:gochecknoglobals // Shared immutable test specifications.
var (
	stringNumProcPerNodeSpec = map[string]any{
		"trainer": map[string]any{"numProcPerNode": "auto"},
	}
	numericNumProcPerNodeSpec = map[string]any{
		"trainer": map[string]any{"numProcPerNode": float64(2)},
	}
	nilNumProcPerNodeSpec = map[string]any{
		"trainer": map[string]any{"numProcPerNode": nil},
	}
	missingNumProcPerNodeSpec = map[string]any{}
)

func TestNumProcPerNodeCheck_Metadata(t *testing.T) {
	g := NewWithT(t)

	chk := trainer.NewNumProcPerNodeCheck()

	g.Expect(chk.ID()).To(Equal("workloads.trainer.numprocpernode"))
	g.Expect(chk.Name()).To(Equal("Workloads :: Trainer :: numProcPerNode (3.6+)"))
	g.Expect(chk.Group()).To(Equal(check.GroupWorkload))
	g.Expect(chk.CheckKind()).To(Equal("trainer"))
	g.Expect(chk.Description()).ToNot(BeEmpty())
}

func TestNumProcPerNodeCheck_CanApply(t *testing.T) {
	tests := []struct {
		name           string
		currentVersion string
		targetVersion  string
		managed        string
		want           bool
	}{
		{name: "upgrade to 3.6 with managed Trainer", currentVersion: "3.5.0", targetVersion: "3.6.0", managed: "Managed", want: true},
		{name: "target below 3.6", currentVersion: "3.5.0", targetVersion: "3.5.0", managed: "Managed", want: false},
		{name: "current already 3.6", currentVersion: "3.6.0", targetVersion: "3.6.0", managed: "Managed", want: false},
		{name: "Trainer removed", currentVersion: "3.5.0", targetVersion: "3.6.0", managed: "Removed", want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			target := testutil.NewTarget(t, testutil.TargetConfig{
				ListKinds:      listKinds,
				Objects:        []*unstructured.Unstructured{testutil.NewDSC(map[string]string{"trainer": tc.managed})},
				CurrentVersion: tc.currentVersion,
				TargetVersion:  tc.targetVersion,
			})

			canApply, err := trainer.NewNumProcPerNodeCheck().CanApply(t.Context(), target)
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(canApply).To(Equal(tc.want))
		})
	}
}

func TestNumProcPerNodeCheck_StringValueBlocks(t *testing.T) {
	g := NewWithT(t)
	target := testutil.NewTarget(t, testutil.TargetConfig{
		ListKinds: listKinds,
		Objects: []*unstructured.Unstructured{
			managedTrainerDSC(),
			newTrainJob("legacy-job", "test-ns", stringNumProcPerNodeSpec),
		},
		CurrentVersion: "3.5.0",
		TargetVersion:  "3.6.0",
	})

	diagnostic, err := trainer.NewNumProcPerNodeCheck().Validate(t.Context(), target)

	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(diagnostic.Status.Conditions[0].Condition).To(MatchFields(IgnoreExtras, Fields{
		"Status":  Equal(metav1.ConditionFalse),
		"Reason":  Equal(check.ReasonWorkloadsImpacted),
		"Message": And(ContainSubstring("Found 1 TrainJob(s)"), ContainSubstring("Delete these TrainJobs")),
	}))
	g.Expect(diagnostic.Status.Conditions[0].Impact).To(Equal(resultpkg.ImpactBlocking))
	g.Expect(diagnostic.ImpactedObjects).To(HaveLen(1))
}

func TestNumProcPerNodeCheck_NonStringValuesDoNotMatch(t *testing.T) {
	g := NewWithT(t)
	target := testutil.NewTarget(t, testutil.TargetConfig{
		ListKinds: listKinds,
		Objects: []*unstructured.Unstructured{
			managedTrainerDSC(),
			newTrainJob("numeric-job", "test-ns", numericNumProcPerNodeSpec),
			newTrainJob("nil-job", "test-ns", nilNumProcPerNodeSpec),
			newTrainJob("missing-job", "test-ns", missingNumProcPerNodeSpec),
		},
		CurrentVersion: "3.5.0",
		TargetVersion:  "3.6.0",
	})

	diagnostic, err := trainer.NewNumProcPerNodeCheck().Validate(t.Context(), target)

	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(diagnostic.Status.Conditions[0].Status).To(Equal(metav1.ConditionTrue))
	g.Expect(diagnostic.ImpactedObjects).To(BeEmpty())
}
