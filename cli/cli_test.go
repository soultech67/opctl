package main

import (
	"io"
	"net/http"
	"os/exec"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var pathToOpctl string

// nodeAvailable reports whether a reachable opctl node (i.e. a working container
// runtime) is present. The node-dependent specs below are skipped when it isn't
// — e.g. in the Docker-less `cli unit` op. They are exercised by the cli
// integration op (cli/.opspec/test/integration), which provides a runtime.
var nodeAvailable bool

var _ = BeforeSuite(func() {
	compiledPath, err := gexec.Build("./", "-buildvcs=false")
	if err != nil {
		panic(err)
	}

	pathToOpctl = filepath.Join(compiledPath, "cli")

	// start node (no-op if one is already running)
	command := exec.Command(pathToOpctl, "node", "create")
	if _, err := gexec.Start(command, io.Discard, io.Discard); err != nil {
		panic(err)
	}

	nodeAvailable = waitForNodeLiveness(10 * time.Second)
})

// waitForNodeLiveness polls the node API liveness endpoint until it responds
// 200 or the timeout elapses.
func waitForNodeLiveness(timeout time.Duration) bool {
	httpClient := &http.Client{Timeout: time.Second}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := httpClient.Get("http://127.0.0.1:42224/api/liveness")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return true
			}
		}
		time.Sleep(250 * time.Millisecond)
	}
	return false
}

var _ = Context("cli", func() {
	BeforeEach(func() {
		if !nodeAvailable {
			Skip("requires a reachable opctl node (container runtime); exercised by the cli integration op")
		}
	})

	Context("--no-color", func() {
		It("should not err", func() {
			/* arrange */
			command := exec.Command(pathToOpctl, "--no-color", "ls")

			/* act */
			session, actualErr := gexec.Start(command, io.Discard, io.Discard)

			/* assert */
			Expect(actualErr).NotTo(HaveOccurred())
			Eventually(session, 10).Should(gexec.Exit(0))
		})
	})

	Context("auth", func() {

		Context("add", func() {

			It("should not err", func() {
				/* arrange */
				providedResources := "resources"
				providedUsername := "username"
				providedPassword := "password"
				command := exec.Command(pathToOpctl, "auth", "add", providedResources, "-u", providedUsername, "-p", providedPassword)

				/* act */
				session, actualErr := gexec.Start(command, GinkgoWriter, GinkgoWriter)

				/* assert */
				Expect(actualErr).NotTo(HaveOccurred())
				Eventually(session, 10).Should(gexec.Exit(0))
			})

		})

	})

	Context("events", func() {
		It("should not err", func() {
			/* arrange */
			command := exec.Command(pathToOpctl, "events")

			/* act */
			session, actualErr := gexec.Start(command, GinkgoWriter, GinkgoWriter)
			session.Interrupt()

			/* assert */
			Expect(actualErr).NotTo(HaveOccurred())
			Eventually(session, 10).Should(gexec.Exit(130))
		})
	})

	Context("ls", func() {
		Context("w/ dirRef", func() {
			It("should not err", func() {
				/* arrange */
				command := exec.Command(pathToOpctl, "ls", "testdata/ls")

				/* act */
				session, actualErr := gexec.Start(command, GinkgoWriter, GinkgoWriter)

				/* assert */
				Expect(actualErr).NotTo(HaveOccurred())
				Eventually(session, 10).Should(gexec.Exit(0))
				Expect(string(session.Out.Contents())).Should(Equal(
					`REF		DESCRIPTION
testdata/ls/op1	A single line description
`))
			})
		})
		Context("w/out dirRef", func() {

			It("should not err", func() {
				/* arrange */
				command := exec.Command(pathToOpctl, "ls")

				/* act */
				session, actualErr := gexec.Start(command, GinkgoWriter, GinkgoWriter)

				/* assert */
				Expect(actualErr).NotTo(HaveOccurred())
				Eventually(session, 10).Should(gexec.Exit(0))
			})
		})
	})

	// disable for now since it will kill the running test container
	XContext("node", Label("Ordered"), func() {

		Context("create", func() {

			It("should not err", func() {
				/* arrange */
				// ensure no node running
				err := exec.Command(pathToOpctl, "node", "delete").Run()
				if err != nil {
					panic(err)
				}

				command := exec.Command(pathToOpctl, "node", "create")

				/* act */
				session, actualErr := gexec.Start(command, GinkgoWriter, GinkgoWriter)
				session.Interrupt()

				/* assert */
				Expect(actualErr).NotTo(HaveOccurred())
				Eventually(session, 10).Should(gexec.Exit(130))
			})

		})

		Context("delete", func() {

			It("should not err", func() {
				/* arrange */
				command := exec.Command(pathToOpctl, "node", "delete")

				/* act */
				session, actualErr := gexec.Start(command, GinkgoWriter, GinkgoWriter)

				/* assert */
				Expect(actualErr).NotTo(HaveOccurred())
				Eventually(session, 10).Should(gexec.Exit(0))
			})

		})

		Context("kill", Label("Serial"), func() {

			It("should not err", func() {
				/* arrange */
				command := exec.Command(pathToOpctl, "node", "kill")

				/* act */
				session, actualErr := gexec.Start(command, GinkgoWriter, GinkgoWriter)

				/* assert */
				Expect(actualErr).NotTo(HaveOccurred())
				Eventually(session, 10).Should(gexec.Exit(0))
			})

		})
	})

	Context("op", func() {

		Context("create", func() {
			Context("w/ path", func() {
				It("should not err", func() {
					/* arrange */
					command := exec.Command(pathToOpctl, "op", "create", "--path", "/tmp", "withPath")

					/* act */
					session, actualErr := gexec.Start(command, GinkgoWriter, GinkgoWriter)

					/* assert */
					Expect(actualErr).NotTo(HaveOccurred())
					Eventually(session, 10).Should(gexec.Exit(0))
				})
			})

			Context("w/out path", func() {
				It("should not err", func() {
					/* arrange */
					command := exec.Command(pathToOpctl, "op", "create", "withoutPath")

					/* act */
					session, actualErr := gexec.Start(command, GinkgoWriter, GinkgoWriter)

					/* assert */
					Expect(actualErr).NotTo(HaveOccurred())
					Eventually(session, 10).Should(gexec.Exit(0))
				})
			})
			Context("w/ description", func() {
				It("should not err", func() {
					/* arrange */
					command := exec.Command(pathToOpctl, "op", "create", "--path", "/tmp", "-d", "dummyOpDescription", "withDescription")

					/* act */
					session, actualErr := gexec.Start(command, GinkgoWriter, GinkgoWriter)

					/* assert */
					Expect(actualErr).NotTo(HaveOccurred())
					Eventually(session, 10).Should(gexec.Exit(0))
				})
			})

			Context("w/out description", func() {
				It("should not err", func() {
					/* arrange */
					command := exec.Command(pathToOpctl, "op", "create", "--path", "/tmp", "withoutDescription")

					/* act */
					session, actualErr := gexec.Start(command, GinkgoWriter, GinkgoWriter)

					/* assert */
					Expect(actualErr).NotTo(HaveOccurred())
					Eventually(session, 10).Should(gexec.Exit(0))
				})
			})
		})

		Context("install", func() {
			It("should not err", func() {
				/* arrange */
				command := exec.Command(
					pathToOpctl,
					"op",
					"install",
					"--path",
					"/tmp/twoArgsCopy",
					"github.com/opspec-pkgs/uuid.v4.generate#1.1.0",
				)

				/* act */
				session, actualErr := gexec.Start(command, GinkgoWriter, GinkgoWriter)

				/* assert */
				Expect(actualErr).NotTo(HaveOccurred())
				Eventually(session, 10).Should(gexec.Exit(0))
			})
		})

		Context("kill", func() {
			It("should not err", func() {
				/* arrange */
				command := exec.Command(pathToOpctl, "op", "kill", "xxx")

				/* act */
				session, actualErr := gexec.Start(command, GinkgoWriter, GinkgoWriter)

				/* assert */
				Expect(actualErr).NotTo(HaveOccurred())
				Eventually(session, 10).Should(gexec.Exit(0))
			})
		})

		Context("validate", func() {

			It("should not err", func() {
				/* arrange */
				command := exec.Command(pathToOpctl, "op", "validate", "./testdata/zeroArgs")

				/* act */
				session, actualErr := gexec.Start(command, GinkgoWriter, GinkgoWriter)

				/* assert */
				Expect(actualErr).NotTo(HaveOccurred())
				Eventually(session, 10).Should(gexec.Exit(0))
			})

		})

	})

	Context("run", func() {
		Context("with two args", func() {
			It("should not err", func() {
				/* arrange */
				command := exec.Command(pathToOpctl, "run", "-a", "arg1=value1", "-a", "arg2=value2", "./testdata/twoArgs")

				/* act */
				session, actualErr := gexec.Start(command, GinkgoWriter, GinkgoWriter)

				/* assert */
				Expect(actualErr).NotTo(HaveOccurred())
				Eventually(session, 10).Should(gexec.Exit(0))
			})
		})

		Context("with zero args", func() {
			It("should not err", func() {
				/* arrange */
				command := exec.Command(pathToOpctl, "run", "./testdata/zeroArgs")

				/* act */
				session, actualErr := gexec.Start(command, GinkgoWriter, GinkgoWriter)

				/* assert */
				Expect(actualErr).NotTo(HaveOccurred())
				Eventually(session, 10).Should(gexec.Exit(0))
			})
		})
	})

	Context("self-update", func() {

		It("should not err", func() {
			/* arrange */
			command := exec.Command(pathToOpctl, "self-update")

			/* act */
			session, actualErr := gexec.Start(command, GinkgoWriter, GinkgoWriter)

			/* assert */
			Expect(actualErr).NotTo(HaveOccurred())
			Eventually(session, 10).Should(gexec.Exit(1))
		})

	})

	Context("ui", func() {
		Context("w/ mountRef", func() {
			It("should not err", func() {
				/* arrange */
				// --no-open: print the URL instead of spawning a browser tab
				command := exec.Command(pathToOpctl, "ui", "--no-open", "../.opspec/build")

				/* act */
				session, actualErr := gexec.Start(command, GinkgoWriter, GinkgoWriter)

				/* assert */
				Expect(actualErr).NotTo(HaveOccurred())
				Eventually(session, 10).Should(gexec.Exit(0))
			})
		})
		Context("w/out mountRef", func() {
			It("should not err", func() {
				/* arrange */
				// --no-open: print the URL instead of spawning a browser tab
				command := exec.Command(pathToOpctl, "ui", "--no-open")

				/* act */
				session, actualErr := gexec.Start(command, GinkgoWriter, GinkgoWriter)

				/* assert */
				Expect(actualErr).NotTo(HaveOccurred())
				Eventually(session, 10).Should(gexec.Exit(0))
			})
		})
	})

})
