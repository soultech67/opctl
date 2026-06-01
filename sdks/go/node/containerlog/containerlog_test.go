package containerlog

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/opctl/opctl/sdks/go/model"
)

var _ = Context("Resolve", func() {
	allEnv := []string{EnvEnabled, EnvMaxSizeMB, EnvMaxBackups, EnvMaxAgeDays, EnvCompress}
	clearEnv := func() {
		for _, k := range allEnv {
			os.Unsetenv(k)
		}
	}
	BeforeEach(clearEnv)
	AfterEach(clearEnv)

	name := "my-worker"
	opPath := "/ops/foo"

	Context("nil log, no env", func() {
		It("is enabled with default-location paths and default knobs", func() {
			/* act */
			cfg := Resolve(nil, "/data", opPath, &name)

			/* assert */
			Expect(cfg.Enabled).To(BeTrue())
			expectedDir := DefaultDir("/data", opPath, &name)
			Expect(cfg.StdOutPath).To(Equal(filepath.Join(expectedDir, "stdout.log")))
			Expect(cfg.StdErrPath).To(Equal(filepath.Join(expectedDir, "stderr.log")))
			Expect(cfg.MaxSizeMB).To(Equal(DefaultMaxSizeMB))
			Expect(cfg.MaxBackups).To(Equal(DefaultMaxBackups))
			Expect(cfg.MaxAgeDays).To(Equal(DefaultMaxAgeDays))
			Expect(cfg.Compress).To(Equal(DefaultCompress))
		})
	})

	Context("DefaultDir", func() {
		It("is <dataDir>/logs/containers/<name>_<opHash>, stable per (name,op) and op-disambiguated", func() {
			d1 := DefaultDir("/data", opPath, &name)
			Expect(d1).To(HavePrefix(filepath.Join("/data", "logs", "containers", "my-worker_")))
			Expect(DefaultDir("/data", opPath, &name)).To(Equal(d1))        // stable across calls
			Expect(DefaultDir("/data", "/ops/bar", &name)).NotTo(Equal(d1)) // different op -> different dir
		})
		It("falls back to 'container' for a nil/empty name and sanitizes unsafe chars", func() {
			Expect(DefaultDir("/data", opPath, nil)).To(HavePrefix(filepath.Join("/data", "logs", "containers", "container_")))
			n := "a/b c"
			Expect(DefaultDir("/data", opPath, &n)).To(HavePrefix(filepath.Join("/data", "logs", "containers", "a_b_c_")))
		})
	})

	Context("log.Dir set", func() {
		It("uses the custom dir with stable name-prefixed files", func() {
			cfg := Resolve(&model.ContainerLog{Dir: "/host/logs"}, "/data", opPath, &name)
			Expect(cfg.StdOutPath).To(Equal(filepath.Join("/host/logs", "my-worker.stdout.log")))
			Expect(cfg.StdErrPath).To(Equal(filepath.Join("/host/logs", "my-worker.stderr.log")))
		})
	})

	Context("disabled", func() {
		It("via spec", func() {
			disabled := false
			Expect(Resolve(&model.ContainerLog{Enabled: &disabled}, "/data", opPath, &name).Enabled).To(BeFalse())
		})
		It("via env", func() {
			os.Setenv(EnvEnabled, "off")
			Expect(Resolve(nil, "/data", opPath, &name).Enabled).To(BeFalse())
		})
		It("when there is no log.dir and no data dir", func() {
			Expect(Resolve(nil, "", opPath, &name).Enabled).To(BeFalse())
			Expect(Resolve(&model.ContainerLog{}, "", opPath, &name).Enabled).To(BeFalse())
		})
	})

	Context("precedence: spec beats env beats default", func() {
		It("resolves each knob by precedence", func() {
			os.Setenv(EnvMaxSizeMB, "99")
			Expect(Resolve(nil, "/data", opPath, &name).MaxSizeMB).To(Equal(99)) // env beats default
			specSize := 7
			Expect(Resolve(&model.ContainerLog{MaxSizeMB: &specSize}, "/data", opPath, &name).MaxSizeMB).To(Equal(7)) // spec beats env
		})
	})

	Context("negative knob", func() {
		It("clamps to the default", func() {
			neg := -5
			Expect(Resolve(&model.ContainerLog{MaxSizeMB: &neg}, "/data", opPath, &name).MaxSizeMB).To(Equal(DefaultMaxSizeMB))
		})
	})
})
