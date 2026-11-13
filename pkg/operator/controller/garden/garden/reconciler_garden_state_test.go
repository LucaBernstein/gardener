// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package garden

import (
	"context"
	"fmt"
	"testing"

	"github.com/go-logr/logr"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	fakekubernetes "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	"github.com/gardener/gardener/pkg/operator/bootstrappers"
	operatorclient "github.com/gardener/gardener/pkg/operator/client"
)

func TestSyncGardenState(t *testing.T) {
	for _, failList := range []bool{false, true} {
		t.Run(fmt.Sprintf("extension listing fails=%t", failList), func(t *testing.T) {
			g := NewWithT(t)
			ctx := context.Background()
			extension := &operatorv1alpha1.Extension{ObjectMeta: metav1.ObjectMeta{Name: "provider-local"}}
			existing, err := bootstrappers.EncodeGardenStateSecret(&operatorv1alpha1.Garden{ObjectMeta: metav1.ObjectMeta{Name: "previous-garden"}}, nil, nil, nil, nil)
			g.Expect(err).NotTo(HaveOccurred())
			c := fakeclient.NewClientBuilder().WithScheme(operatorclient.RuntimeScheme).WithObjects(existing, extension).
				WithInterceptorFuncs(interceptor.Funcs{List: func(ctx context.Context, c client.WithWatch, list client.ObjectList, opts ...client.ListOption) error {
					if _, ok := list.(*operatorv1alpha1.ExtensionList); ok && failList {
						return fmt.Errorf("extension list failed")
					}
					return c.List(ctx, list, opts...)
				}}).Build()
			r := &Reconciler{
				RuntimeClientSet: fakekubernetes.NewClientSetBuilder().WithClient(c).Build(),
				GardenNamespace:  v1beta1constants.GardenNamespace,
			}
			err = r.syncGardenState(ctx, logr.Discard(), &operatorv1alpha1.Garden{ObjectMeta: metav1.ObjectMeta{Name: "garden"}})
			secret := &corev1.Secret{}
			g.Expect(c.Get(ctx, client.ObjectKeyFromObject(existing), secret)).To(Succeed())
			if failList {
				g.Expect(err).To(MatchError("failed listing operator Extensions to persist: extension list failed"))
				g.Expect(secret.Data).To(Equal(existing.Data))
				return
			}
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(secret.Data).To(HaveLen(1))
			g.Expect(secret.Data).To(HaveKey("state.br"))
			state, err := bootstrappers.DecodeGardenStateSecret(secret)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(state.Garden.Name).To(Equal("garden"))
			g.Expect(state.OperatorExtensions).To(Equal([]operatorv1alpha1.GardenStateExtension{{Name: extension.Name}}))
		})
	}
}
