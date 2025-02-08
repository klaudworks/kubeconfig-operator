package kubeconfig_test

import (
	"context"
	"testing"
	"time"

	"github.com/fgrosse/zaptest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/reddit/achilles-sdk/pkg/fsm/metrics"
	"github.com/reddit/achilles-sdk/pkg/io"
	"github.com/reddit/achilles-sdk/pkg/logging"
	achratelimiter "github.com/reddit/achilles-sdk/pkg/ratelimiter"
	sdktest "github.com/reddit/achilles-sdk/pkg/test"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
	ctrlzap "sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/klaudworks/kubeconfig-operator/internal/controllers/kubeconfig"
	"github.com/klaudworks/kubeconfig-operator/internal/controlplane"
	intscheme "github.com/klaudworks/kubeconfig-operator/internal/scheme"
	"github.com/klaudworks/kubeconfig-operator/internal/test"
)

var (
	ctx     context.Context
	testEnv *sdktest.TestEnv
	c       client.Client
	scheme  *runtime.Scheme
	log     *zap.SugaredLogger
)

func TestKubeconfig(t *testing.T) {
	RegisterFailHandler(Fail)
	ctrllog.SetLogger(ctrlzap.New(ctrlzap.WriteTo(GinkgoWriter), ctrlzap.UseDevMode(true)))
	RunSpecs(t, "Kubeconfig Suite")
}

var _ = BeforeSuite(func() {
	SetDefaultEventuallyTimeout(15 * time.Second)
	SetDefaultEventuallyPollingInterval(200 * time.Millisecond)

	log = zaptest.LoggerWriter(GinkgoWriter).Sugar()
	ctx = logging.NewContext(context.Background(), log)
	rl := achratelimiter.NewDefaultProviderRateLimiter(achratelimiter.DefaultProviderRPS)

	scheme = intscheme.MustNewScheme()

	var err error
	testEnv, err = sdktest.NewEnvTestBuilder(ctx).
		WithCRDDirectoryPaths(
			test.CRDPaths(),
		).
		WithScheme(scheme).
		WithLog(log.Desugar()).
		WithManagerSetupFns(
			func(mgr manager.Manager) error {
				// setup controller being tested
				clientApplicator := &io.ClientApplicator{
					Client:     mgr.GetClient(),
					Applicator: io.NewAPIPatchingApplicator(mgr.GetClient()),
				}

				cpCtx := controlplane.Context{
					Metrics: metrics.MustMakeMetrics(scheme, prometheus.NewRegistry()),
				}

				return kubeconfig.SetupController(ctx, cpCtx, mgr, rl, clientApplicator)
			},
		).
		WithKubeConfigFile("./").
		Start()

	Expect(err).ToNot(HaveOccurred())

	c = testEnv.Client
})

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	err := testEnv.Stop()
	Expect(err).NotTo(HaveOccurred())
})
