package api_integration_test

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"log"
	"math/big"
	"net/http"
	"os"
	"path"
	"testing"
	"time"

	. "github.com/cloudfoundry-incubator/credhub-acceptance-tests/test_helpers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var (
	config Config
	err    error
)

var _ = Describe("mutual TLS authentication", func() {

	Describe("with a certificate signed by a trusted CA	", func() {
		BeforeEach(func() {
			config, err = LoadConfig()
			Expect(err).NotTo(HaveOccurred())
		})

		It("allows the client to hit an authenticated endpoint", func() {
			generateCertificate()
			postData := `{"name":"mtlstest","type":"password"}`
			result, err := mtlsPost(
				config.ApiUrl+"/api/v1/data",
				postData,
				"server_ca_cert.pem",
				"client.pem",
				"client_key.pem")

			Expect(err).To(BeNil())
			Expect(result).To(MatchRegexp(`"type":"password"`))
		})
	})

	Describe("with an expired certificate", func() {
		BeforeEach(func() {
			config, err = LoadConfig()
			Expect(err).NotTo(HaveOccurred())
		})

		It("prevents the client from hitting an authenticated endpoint", func() {
			postData := `{"name":"mtlstest","type":"password"}`
			result, err := mtlsPost(
				config.ApiUrl+"/api/v1/data",
				postData,
				"server_ca_cert.pem",
				"expired.pem",
				"expired_key.pem")

			fmt.Printf("result: %#v\n", result)
			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(ContainSubstring("unknown certificate"))
			Expect(result).To(BeEmpty())
		})
	})

	Describe("with a certificate signed by an untrusted CA", func() {
		BeforeEach(func() {
			config, err = LoadConfig()
			Expect(err).NotTo(HaveOccurred())
		})

		It("prevents the client from hitting an authenticated endpoint", func() {
			postData := `{"name":"mtlstest","type":"password"}`
			result, err := mtlsPost(
				config.ApiUrl+"/api/v1/data",
				postData,
				"server_ca_cert.pem",
				"invalid.pem",
				"invalid_key.pem")

			// golang doesn't send client certificate if it's signed by the CA
			// that server doesn't trust if the server is configured with
			// server.ssl.client-auth=want (https://tools.ietf.org/html/rfc5246#section-7.4.4)
			// That is why, we are asserting on OAuth authorization failure here.
			Expect(err).To(BeNil())
			Expect(result).To(MatchRegexp(".*Full authentication is required to access this resource"))
		})
	})
})

func TestMTLS(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "mTLS Test Suite")
}

func handleError(err error) {
	if err != nil {
		log.Fatal("Fatal", err)
	}
}

func mtlsPost(url string, postData string, serverCaFilename, clientCertFilename, clientKeyPath string) (string, error) {
	client, err := createMtlsClient(serverCaFilename, clientCertFilename, clientKeyPath)

	resp, err := client.Post(url, "application/json", bytes.NewBuffer([]byte(postData)))
	if err != nil {
		return "", err
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(body), nil
}

func createMtlsClient(serverCaFilename, clientCertFilename, clientKeyFilename string) (*http.Client, error) {
	serverCaPath := path.Join(config.CredentialRoot, serverCaFilename)
	clientCertPath := path.Join(os.Getenv("PWD"), "certs", clientCertFilename)
	clientKeyPath := path.Join(os.Getenv("PWD"), "certs", clientKeyFilename)

	_, err := os.Stat(serverCaPath)
	handleError(err)
	_, err = os.Stat(clientCertPath)
	handleError(err)
	_, err = os.Stat(clientKeyPath)
	handleError(err)

	clientCertificate, err := tls.LoadX509KeyPair(clientCertPath, clientKeyPath)
	handleError(err)

	trustedCAs := x509.NewCertPool()
	serverCA, err := ioutil.ReadFile(serverCaPath)

	ok := trustedCAs.AppendCertsFromPEM([]byte(serverCA))
	if !ok {
		log.Fatal("failed to parse root certificate")
	}

	tlsConf := &tls.Config{
		Certificates: []tls.Certificate{clientCertificate},
		RootCAs:      trustedCAs,
	}

	transport := &http.Transport{TLSClientConfig: tlsConf}
	client := &http.Client{Transport: transport}

	return client, err
}

func generateCertificate() {
	clientCaCertPath := path.Join(config.CredentialRoot, "client_ca_cert.pem")
	clientCaPrivatePath := path.Join(config.CredentialRoot, "client_ca_private.pem")

	clientCaCertPem, err := ioutil.ReadFile(clientCaCertPath)
	Expect(err).NotTo(HaveOccurred())

	block, _ := pem.Decode([]byte(clientCaCertPem))

	clientCaCert, err := x509.ParseCertificate(block.Bytes)
	Expect(err).NotTo(HaveOccurred())

	clientCaKeyPem, err := ioutil.ReadFile(clientCaPrivatePath)
	Expect(err).NotTo(HaveOccurred())

	block, _ = pem.Decode([]byte(clientCaKeyPem))

	clientCaKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	Expect(err).NotTo(HaveOccurred())

	certTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(1658),
		Subject: pkix.Name{
			CommonName: "credhub_test_client",
		},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().AddDate(10, 0, 0),
		SubjectKeyId: []byte{1, 2, 3, 4, 6},
	}
	priv, _ := rsa.GenerateKey(rand.Reader, 2048)
	pub := &priv.PublicKey
	cert, err := x509.CreateCertificate(
		rand.Reader,
		certTemplate,
		clientCaCert,
		pub,
		clientCaKey)
	Expect(err).NotTo(HaveOccurred())

	fmt.Printf("client ca cert: %#v\n", cert)
}
