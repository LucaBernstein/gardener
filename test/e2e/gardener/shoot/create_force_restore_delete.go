// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package shoot

import (
	"os"
	"os/exec"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	. "github.com/gardener/gardener/test/e2e"
	. "github.com/gardener/gardener/test/e2e/gardener"
	"github.com/gardener/gardener/test/e2e/gardener/seed"
	shootmigration "github.com/gardener/gardener/test/utils/shoots/migration"
)

// This test exercises the "bad-case control-plane migration" / disaster-recovery flow: a Shoot's control
// plane is restored in place on a Seed that lost its etcd data, reusing the existing backup bucket via the
// `force-restore` operation annotation (no backup copy, unlike regular control-plane migration).
//
// The disaster is simulated by destroying and recreating the local2 KIND node. The backup bucket is a
// host-path mount (dev/local-backupbuckets), so its data survives `make kind2-down` and is re-mounted on
// `make kind2-up`, matching a real Seed recovered in place. The local2 gardenlet is deployed via the
// single-node2-force-restore overlay (GARDENLET_PROFILE_SUFFIX=force-restore), which enables the
// ShootState controller (shootState.concurrentSyncs=2) so there is current state to restore from.
var _ = Describe("Shoot Tests", Label("Shoot", "control-plane-force-restore"), func() {
	test := func(s *ShootContext) {
		// Schedule the shoot directly on local2 - the seed whose node will be recovered.
		s.Shoot.Spec.SeedName = new(getSeedName(true))

		ItShouldCreateShoot(s)
		ItShouldWaitForShootToBeReconciledAndHealthy(s)
		ItShouldGetResponsibleSeed(s)
		seed.ItShouldInitializeSeedClient(&s.SeedContext)

		var secretsBeforeRestore map[string]corev1.Secret

		It("Populate comparison elements before restore", func(ctx SpecContext) {
			Eventually(ctx, func() error {
				var err error
				secretsBeforeRestore, err = shootmigration.GetPersistedSecrets(ctx, s.SeedClientSet.Client(), s.Shoot.Status.TechnicalID)
				return err
			}).Should(Succeed())
		}, SpecTimeout(time.Minute))

		// Pause shoot reconciliation on the seed BEFORE the gardenlet comes back up, mirroring the documented
		// operator procedure (docs/operations/disaster_recovery_seed.md). This is essential: the shoot's
		// status.lastOperation lives in the garden cluster and survives the seed etcd loss as 'Reconcile
		// Succeeded'. Without the emergency stop, the recovered gardenlet would compute an ordinary Reconcile
		// (not a Restore - the force-restore annotation is not set yet) and reconcile against the empty etcd,
		// regenerating secrets. The Seed object is a garden-cluster resource, so it can be annotated while the
		// seed node is down.
		seed.ItShouldAnnotateSeed(&s.SeedContext, map[string]string{
			v1beta1constants.AnnotationEmergencyStopShootReconciliations: "true",
		})

		It("Simulate a Seed disaster by destroying and recreating the local2 node", func(ctx SpecContext) {
			// Destroying the KIND node wipes the seed's etcd. The backup bucket lives on a host-path mount
			// (dev/local-backupbuckets), so it survives the node teardown and is re-mounted on kind2-up.
			// GARDENLET_PROFILE_SUFFIX selects the force-restore gardenlet overlay (ShootState controller on).
			By("Destroy the local2 node (wipes the seed etcd)")
			runMake(ctx, nil, "kind2-down")

			By("Recreate the local2 node (re-mounts the surviving backup bucket)")
			runMake(ctx, nil, "kind2-up")

			By("Redeploy the local2 gardenlet")
			runMake(ctx, []string{"GARDENLET_PROFILE_SUFFIX=force-restore"}, "seed2-up")
		}, SpecTimeout(20*time.Minute))

		// Wait for the recovered seed to become ready again before triggering the restore.
		// ItShouldWaitForSeedToBeReady polls s.SeedContext.Seed (set to local2 by ItShouldGetResponsibleSeed)
		// via health.CheckSeed, with a 10-minute timeout.
		seed.ItShouldWaitForSeedToBeReady(&s.SeedContext)

		// Trigger the force-restore operation on the shoot. The gardener-apiserver bumps the generation.
		ItShouldAnnotateShoot(s, map[string]string{
			v1beta1constants.GardenerOperation: v1beta1constants.GardenerOperationForceRestore,
		})

		// Lift the emergency stop so the gardenlet can pick up the restore operation.
		It("Remove the emergency-stop annotation from Seed local2", func(ctx SpecContext) {
			Eventually(ctx, func() error {
				if err := s.GardenClient.Get(ctx, client.ObjectKeyFromObject(s.SeedContext.Seed), s.SeedContext.Seed); err != nil {
					return err
				}
				patch := client.MergeFrom(s.SeedContext.Seed.DeepCopy())
				delete(s.SeedContext.Seed.Annotations, v1beta1constants.AnnotationEmergencyStopShootReconciliations)
				return s.GardenClient.Patch(ctx, s.SeedContext.Seed, patch)
			}).Should(Succeed())
		}, SpecTimeout(time.Minute))

		ItShouldWaitForShootToBeReconciledAndHealthy(s)

		// The gardenlet removes the force-restore annotation once etcd is ready.
		It("Verify that the force-restore annotation was removed from the Shoot", func(ctx SpecContext) {
			Eventually(ctx, func(g Gomega) {
				g.Expect(s.GardenKomega.Get(s.Shoot)()).To(Succeed())
				g.Expect(s.Shoot.Annotations).NotTo(HaveKey(v1beta1constants.GardenerOperation))
			}).WithPolling(10 * time.Second).Should(Succeed())
		}, SpecTimeout(time.Minute))

		// Secrets must not have been regenerated - same names, same data - since the restore reuses the
		// persisted ETCD backup and ShootState.
		It("Verify that all secrets have been restored without regeneration", func(ctx SpecContext) {
			var secretsAfterRestore map[string]corev1.Secret
			Eventually(ctx, func() error {
				var err error
				secretsAfterRestore, err = shootmigration.GetPersistedSecrets(ctx, s.SeedClientSet.Client(), s.Shoot.Status.TechnicalID)
				return err
			}).Should(Succeed())
			Expect(shootmigration.ComparePersistedSecrets(secretsBeforeRestore, secretsAfterRestore)).To(Succeed())
		}, SpecTimeout(time.Minute))

		ItShouldDeleteShoot(s)
		ItShouldWaitForShootToBeDeleted(s)
	}

	Context("Shoot with workers", Ordered, PriorityLonger, func() {
		test(NewTestContext().ForShoot(DefaultShoot("e2e-frestore")))
	})
})

// runMake runs a `make` target from the repository root (four levels up from this test package),
// streaming its output to the Ginkgo writer. extraEnv entries (KEY=VALUE) are appended to the process
// environment so dev-setup scripts that read them (e.g. GARDENLET_PROFILE_SUFFIX in scenario.sh) pick
// them up. It drives the out-of-band Seed outage that a single test process cannot otherwise
// orchestrate (see the gardenadm restore e2e for the same `make` shell-out pattern).
func runMake(ctx SpecContext, extraEnv []string, target string) {
	GinkgoHelper()

	cmd := exec.CommandContext(ctx, "make", target) // #nosec G204 -- Used for e2e tests only.
	cmd.Dir = filepath.Join("..", "..", "..", "..")
	cmd.Env = append(os.Environ(), extraEnv...)
	cmd.Stdout = gexec.NewPrefixedWriter("[make "+target+" out] ", GinkgoWriter)
	cmd.Stderr = gexec.NewPrefixedWriter("[make "+target+" err] ", GinkgoWriter)
	Expect(cmd.Run()).To(Succeed())
}
