package bbr_integration

import (
	"fmt"

	"github.com/cloudfoundry-incubator/credhub-acceptance-tests/test_helpers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gbytes"
	. "github.com/onsi/gomega/gexec"
	"encoding/json"
	"strings"
)

type findResults struct {
	Credentials []findResult `json:"credentials"`
}

type findResult struct {
	Name string `json:"name"`
	VersionCreatedAt string `json:"version_created_at"`
}

var _ = Describe("Backup and Restore", func() {
	var credentialName string
	var bbrTestPath = "bbr_test"

	BeforeEach(func() {
		var session *Session
		credentialName = fmt.Sprintf("%s/%s", bbrTestPath, test_helpers.GenerateUniqueCredentialName())

		By("authenticating against credhub")

		session = RunCommand("credhub", "api", "--server", config.ApiUrl, "--skip-tls-validation")
		Eventually(session).Should(Exit(0))

		session = RunCommand("credhub", "login", "--skip-tls-validation", "-u", config.ApiUsername, "-p", config.ApiPassword)
		Eventually(session).Should(Exit(0))

		CleanupCredhub(bbrTestPath)
	})

	AfterEach(func() {
		CleanupCredhub(bbrTestPath)
		CleanupArtifacts()
	})

	It("Successfully backs up and restores a Credhub release", func() {
		var session *Session
		var bbrArgs []string

		if config.Bosh.Deployment == "" {
			// director case
			bbrArgs = []string{
				"bbr",
				"director",
				"--host", config.Bosh.Host,
				"--username", config.Bosh.SshUsername,
				"--private-key-path", config.Bosh.SshPrivateKeyPath,
			}
		} else {
			// deployment case mysql
			bbrArgs = []string{
				"bbr",
				"deployment",
				"--target", config.Bosh.Host,
				"--username", config.Bosh.DirectorUsername,
				"--password", config.Bosh.DirectorPassword,
				"--deployment", config.Bosh.Deployment,
			}
		}

		By("adding a test credential")
		session = RunCommand("credhub", "set", "--name", credentialName, "--type", "password", "-w", "originalsecret")
		Eventually(session).Should(Exit(0))

		By("running bbr backup")
		backupArgs := append(bbrArgs, "backup")
		session = RunCommand(backupArgs...)
		Eventually(session).Should(Exit(0))

		By("asserting that the backup archive exists and contains a pg dump file")
		session = RunCommand("sh", "-c", fmt.Sprintf("tar -xvf ./%s*Z/bosh*credhub.tar", config.DirectorHost))
		Eventually(session).Should(Exit(0))
		Eventually(session).Should(Exit(0))

		By("editing the test credential")
		session = RunCommand("credhub", "set", "--name", credentialName, "--type", "password", "-w", "updatedsecret")
		Eventually(session).Should(Exit(0))

		session = RunCommand("credhub", "get", "--name", credentialName)
		Eventually(session).Should(Exit(0))
		Eventually(session.Out).Should(Say("value: updatedsecret"))

		By("running bbr restore")
		restoreArgs := append(bbrArgs, "restore", "--artifact-path", "./%s*Z/")
		session = RunCommand("sh", "-c", strings.Join(restoreArgs, " "))
		Eventually(session).Should(Exit(0))

		By("checking if the test credentials was restored")
		session = RunCommand("credhub", "get", "--name", credentialName)
		Eventually(session).Should(Exit(0))
		Eventually(session.Out).Should(Say("value: originalsecret"))
	})
})

func CleanupCredhub(path string) {
	var session *Session
	var credentials findResults

	By("Cleaning up credhub bbr test passwords")
	namePrefix := fmt.Sprintf("/%s", path)
	session = RunCommand("credhub", "find", "-p", namePrefix, "--output-json")

	errorMessage := string(session.Err.Contents())

	if !strings.Contains(errorMessage, "No credentials exist which match the provided parameters.") {
		Eventually(session).Should(Exit(0))
		results := string(session.Out.Contents())

		err := json.Unmarshal([]byte(results), &credentials)
		Expect(err).To(BeNil())

		for _, credential := range credentials.Credentials {
			session = RunCommand("credhub", "delete", "-n", credential.Name)
			Eventually(session).Should(Exit(0))
		}
	}
}

func CleanupArtifacts() {
	By("Cleaning up bbr test artifacts")
	RunCommand("rm", "-rf", "credhubdb_dump")
	RunCommand("sh", "-c", fmt.Sprintf("rm -rf %s*Z", config.DirectorHost))
}
