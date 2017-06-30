package integration_test

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"

	. "github.com/cloudfoundry-incubator/credhub-acceptance-tests/test_helpers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
	"gopkg.in/yaml.v2"
	"strings"
)

var _ = Describe("Certificates Test", func() {
	Describe("setting a certificate", func() {
		It("should be able to set a certificate", func() {
			name := GenerateUniqueCredentialName()
			session := RunCommand("set", "-n", name, "-t", "certificate", "--certificate-string=iamacertificate", "--private-string=iamakey", "--root-string=someca")
			stdOut := string(session.Out.Contents())

			Eventually(session).Should(Exit(0))

			Expect(stdOut).To(ContainSubstring(`name: /` + name))
			Expect(stdOut).To(ContainSubstring(`type: certificate`))
			Expect(stdOut).To(ContainSubstring(`ca: someca`))
			Expect(stdOut).To(ContainSubstring(`certificate: iamacertificate`))
			Expect(stdOut).To(ContainSubstring(`private_key: iamakey`))
		})

		It("should require a certificate type", func() {
			session := RunCommand("set", "-n", GenerateUniqueCredentialName(), "-t", "certificate")
			Eventually(session).Should(Exit(1))
			Expect(session.Err.Contents()).To(MatchRegexp(".*At least one certificate attribute must be set. Please validate your input and retry your request."))
		})

		FIt("should allow you to set a certificate with a named CA", func() {
			caName := GenerateUniqueCredentialName()
			certName := GenerateUniqueCredentialName()
			session := RunCommand("generate", "-n", caName, "-t", "certificate", "--is-ca", "-c", "commonName")
			Eventually(session).Should(Exit(0))
			stdOut := string(session.Out.Contents())
			type certificate struct {
				Certificate string `yaml:"certificate,omitempty"`
			}
			cert := certificate{}
			err := yaml.Unmarshal([]byte(stdOut), &cert)
			Expect(err).To(BeNil())

			session = RunCommand("set", "-n", certName, "-t", "certificate", "--certificate-string=iamacertificate", "--private-string=iamakeytoo", "--ca-name", caName)
			Eventually(session).Should(Exit(0))
			stdOut = string(session.Out.Contents())

			Expect(stdOut).To(ContainSubstring(`name: /` + certName))
			Expect(stdOut).To(ContainSubstring(`type: certificate`))
			Expect(stdOut).To(ContainSubstring(`ca: ` + cert.Certificate + `woof`))
			Expect(stdOut).To(ContainSubstring(`certificate: iamacertificate`))
			Expect(stdOut).To(ContainSubstring(`private_key: iamakeytoo`))
		})
	})

	Describe("CAs and Certificates", func() {
		Describe("certificate chains", func() {
			It("should build the chain with an intermediate CA", func() {
				rootCaName := GenerateUniqueCredentialName()
				intermediateCaName := GenerateUniqueCredentialName()
				leafCertificateName := GenerateUniqueCredentialName()

				session := RunCommand("generate", "-n", rootCaName, "-t", "certificate", "-c", rootCaName, "--is-ca", "--self-sign")
				cert := CertFromPem(string(session.Out.Contents()), false)
				Expect(cert.Subject.CommonName).To(Equal(rootCaName))
				Expect(cert.Issuer.CommonName).To(Equal(rootCaName))
				Expect(cert.IsCA).To(Equal(true))
				Expect(len(cert.SubjectKeyId)).ToNot(Equal(0))

				session = RunCommand("generate", "-n", intermediateCaName, "-t", "certificate", "-c", intermediateCaName, "--is-ca", "--ca", rootCaName)
				cert = CertFromPem(string(session.Out.Contents()), false)
				Expect(cert.Subject.CommonName).To(Equal(intermediateCaName))
				Expect(cert.Issuer.CommonName).To(Equal(rootCaName))
				Expect(cert.IsCA).To(Equal(true))

				session = RunCommand("generate", "-n", leafCertificateName, "-t", "certificate", "-c", leafCertificateName, "--ca", intermediateCaName)
				cert = CertFromPem(string(session.Out.Contents()), false)
				Expect(cert.Subject.CommonName).To(Equal(leafCertificateName))
				Expect(cert.Issuer.CommonName).To(Equal(intermediateCaName))
				Expect(cert.IsCA).To(Equal(false))
			})
		})

		It("should generate a ca when using the --is-ca flag", func() {
			certificateId := GenerateUniqueCredentialName()
			certificateAuthorityId := GenerateUniqueCredentialName()

			By("generating the CA", func() {
				session := RunCommand("generate", "-n", certificateAuthorityId, "-t", "certificate", "--common-name", certificateAuthorityId, "--is-ca")
				stdOut := string(session.Out.Contents())

				Eventually(session).Should(Exit(0))

				Expect(stdOut).To(ContainSubstring(`type: certificate`))
				Expect(stdOut).To(MatchRegexp(`certificate: |\s+-----BEGIN CERTIFICATE-----`))
				Expect(stdOut).To(MatchRegexp(`private_key: |\s+-----BEGIN RSA PRIVATE KEY-----`))
				cert := CertFromPem(stdOut, false)
				Expect(cert.Subject.CommonName).To(Equal(certificateAuthorityId))
				Expect(cert.Issuer.CommonName).To(Equal(certificateAuthorityId)) // self-signed
				Expect(cert.IsCA).To(Equal(true))
			})

			By("getting the CA", func() {
				session := RunCommand("get", "-n", certificateAuthorityId)
				stdOut := string(session.Out.Contents())
				Eventually(session).Should(Exit(0))
				cert := CertFromPem(stdOut, false)
				Expect(cert.Subject.CommonName).To(Equal(certificateAuthorityId))
				Expect(cert.Issuer.CommonName).To(Equal(certificateAuthorityId)) // self-signed
				Expect(cert.IsCA).To(Equal(true))
			})

			By("generating and signing the certificate", func() {
				session := RunCommand("generate", "-n", certificateId, "-t", "certificate", "--common-name", certificateId, "--ca", certificateAuthorityId, "-e", "code_signing", "-g", "digital_signature", "-a", "example.com", "-k", "3072", "-d", "90")
				stdOut := string(session.Out.Contents())

				Eventually(session).Should(Exit(0))

				Expect(stdOut).To(ContainSubstring(`type: certificate`))
				Expect(stdOut).To(MatchRegexp(`certificate: |\s+-----BEGIN CERTIFICATE-----`))
				Expect(stdOut).To(MatchRegexp(`private_key: |\s+-----BEGIN RSA PRIVATE KEY-----`))
				cert := CertFromPem(stdOut, false)
				ca := CertFromPem(stdOut, true)

				Expect(cert.AuthorityKeyId).To(Equal(ca.SubjectKeyId))

				Expect(cert.Subject.CommonName).To(Equal(certificateId))
				Expect(cert.Issuer.CommonName).To(Equal(certificateAuthorityId))
				Expect(ca.CheckSignature(cert.SignatureAlgorithm, cert.RawTBSCertificate, cert.Signature)).To(BeNil()) // signed by ca
				Expect(cert.ExtKeyUsage).To(Equal([]x509.ExtKeyUsage{x509.ExtKeyUsageCodeSigning}))
				Expect(cert.KeyUsage).To(Equal(x509.KeyUsageDigitalSignature))
				Expect(cert.IsCA).To(Equal(false))
				Expect(cert.NotAfter.Sub(cert.NotBefore).Hours()).To(Equal(90 * 24.0))
				Expect(cert.PublicKey.(*rsa.PublicKey).N.BitLen()).To(Equal(3072))
				Expect(cert.DNSNames).To(Equal([]string{"example.com"}))
			})

			By("getting the certificate", func() {
				session := RunCommand("get", "-n", certificateId)
				Eventually(session).Should(Exit(0))
			})

			By("regenerating the certificate", func() {
				session := RunCommand("regenerate", "-n", certificateId)
				Eventually(session).Should(Exit(0))
				stdOut := string(session.Out.Contents())
				cert := CertFromPem(stdOut, false)
				ca := CertFromPem(stdOut, true)
				Expect(cert.Subject.CommonName).To(Equal(certificateId))
				Expect(cert.Issuer.CommonName).To(Equal(certificateAuthorityId))
				Expect(ca.CheckSignature(cert.SignatureAlgorithm, cert.RawTBSCertificate, cert.Signature)).To(BeNil()) // signed by ca
				Expect(cert.ExtKeyUsage).To(Equal([]x509.ExtKeyUsage{x509.ExtKeyUsageCodeSigning}))
				Expect(cert.KeyUsage).To(Equal(x509.KeyUsageDigitalSignature))
				Expect(cert.IsCA).To(Equal(false))
				Expect(cert.NotAfter.Sub(cert.NotBefore).Hours()).To(Equal(90 * 24.0))
				Expect(cert.PublicKey.(*rsa.PublicKey).N.BitLen()).To(Equal(3072))
				Expect(cert.DNSNames).To(Equal([]string{"example.com"}))
			})
		})

		It("should be able to generate a self-signed certificate", func() {
			certificateId := GenerateUniqueCredentialName()
			initialCertificate := ""
			initialPrivateKey := ""

			By("generating the certificate", func() {
				session := RunCommand("generate", "-n", certificateId, "-t", "certificate", "--common-name", certificateId, "--self-sign", "-e", "email_protection", "-g", "digital_signature", "-a", "example.com", "-k", "3072", "-d", "90")
				stdOut := string(session.Out.Contents())

				Eventually(session).Should(Exit(0))

				Expect(stdOut).To(ContainSubstring(`type: certificate`))
				Expect(stdOut).To(MatchRegexp(`certificate: |\s+-----BEGIN CERTIFICATE-----`))
				Expect(stdOut).To(MatchRegexp(`private_key: |\s+-----BEGIN RSA PRIVATE KEY-----`))

				initialCertificate = stdOut[strings.Index(stdOut, "-----BEGIN CERTIFICATE-----"):strings.Index(stdOut, "-----END CERTIFICATE-----")]
				initialPrivateKey = stdOut[strings.Index(stdOut, "-----BEGIN RSA PRIVATE KEY-----"):strings.Index(stdOut, "-----END RSA PRIVATE KEY-----")]

				cert := CertFromPem(stdOut, false)
				Expect(cert.Subject.CommonName).To(Equal(certificateId))
				Expect(cert.Issuer.CommonName).To(Equal(certificateId))                                                  // self-signed
				Expect(cert.CheckSignature(cert.SignatureAlgorithm, cert.RawTBSCertificate, cert.Signature)).To(BeNil()) // signed by self
				Expect(cert.IsCA).To(Equal(false))
				Expect(cert.ExtKeyUsage).To(Equal([]x509.ExtKeyUsage{x509.ExtKeyUsageEmailProtection}))
				Expect(cert.KeyUsage).To(Equal(x509.KeyUsageDigitalSignature))
				Expect(cert.NotAfter.Sub(cert.NotBefore).Hours()).To(Equal(90 * 24.0))
				Expect(cert.PublicKey.(*rsa.PublicKey).N.BitLen()).To(Equal(3072))
				Expect(cert.DNSNames).To(Equal([]string{"example.com"}))
			})

			By("getting the certificate", func() {
				session := RunCommand("get", "-n", certificateId)
				stdOut := string(session.Out.Contents())
				Eventually(session).Should(Exit(0))
				Expect(stdOut).To(MatchRegexp(`certificate: |\s+-----BEGIN CERTIFICATE-----`))
			})

			By("regenerating the certificate", func() {
				session := RunCommand("regenerate", "-n", certificateId)
				Eventually(session).Should(Exit(0))

				stdOut := string(session.Out.Contents())
				cert := CertFromPem(stdOut, false)
				Expect(cert.Subject.CommonName).To(Equal(certificateId))
				Expect(cert.Issuer.CommonName).To(Equal(certificateId))                                                  // self-signed
				Expect(cert.CheckSignature(cert.SignatureAlgorithm, cert.RawTBSCertificate, cert.Signature)).To(BeNil()) // signed by self
				Expect(cert.IsCA).To(Equal(false))
				Expect(cert.ExtKeyUsage).To(Equal([]x509.ExtKeyUsage{x509.ExtKeyUsageEmailProtection}))
				Expect(cert.KeyUsage).To(Equal(x509.KeyUsageDigitalSignature))
				Expect(cert.NotAfter.Sub(cert.NotBefore).Hours()).To(Equal(90 * 24.0))
				Expect(cert.PublicKey.(*rsa.PublicKey).N.BitLen()).To(Equal(3072))
				Expect(cert.DNSNames).To(Equal([]string{"example.com"}))

				Expect(stdOut).NotTo(ContainSubstring(initialCertificate))
				Expect(stdOut).NotTo(ContainSubstring(initialPrivateKey))
			})
		})

		It("should error gracefully when supplying an invalid extended key usage name", func() {
			certificateAuthorityId := GenerateUniqueCredentialName()
			certificateId := certificateAuthorityId + "1"
			RunCommand("generate", "-n", certificateAuthorityId, "-t certificate", "--common-name", certificateAuthorityId, "--is-ca")
			session := RunCommand("generate", "-n", certificateId, "-t", "certificate", "--common-name", certificateId, "--ca", certificateAuthorityId, "-e", "code_sinning")
			stdErr := string(session.Err.Contents())

			Eventually(session).Should(Exit(1))
			Expect(stdErr).To(MatchRegexp(`The provided extended key usage 'code_sinning' is not supported. Valid values include client_auth, server_auth, code_signing, email_protection and timestamping.`))
		})

		It("should error gracefully when supplying an invalid key usage name", func() {
			certificateAuthorityId := GenerateUniqueCredentialName()
			certificateId := certificateAuthorityId + "1"
			RunCommand("generate", "-n", certificateAuthorityId, "-t certificate", "--common-name", certificateAuthorityId, "--is-ca")
			session := RunCommand("generate", "-n", certificateId, "-t", "certificate", "--common-name", certificateId, "--ca", certificateAuthorityId, "-g", "digital_sinnature")
			stdErr := string(session.Err.Contents())

			Eventually(session).Should(Exit(1))
			Expect(stdErr).To(MatchRegexp(`The provided key usage 'digital_sinnature' is not supported. Valid values include digital_signature, non_repudiation, key_encipherment, data_encipherment, key_agreement, key_cert_sign, crl_sign, encipher_only and decipher_only.`))
		})

		It("should handle secrets whose names have lots of special characters", func() {
			madDogCAId := "dan:test/ing?danothertbe$in&the[stuff]=that@shouldn!"

			By("setting a value with lots of special characters", func() {
				session := RunCommand("generate", "-t", "certificate", "-n", madDogCAId, "--common-name", GenerateUniqueCredentialName(), "--is-ca")
				Eventually(session).Should(Exit(0))
			})

			By("retrieving the value that was set", func() {
				session := RunCommand("get", "-n", madDogCAId)
				Eventually(session).Should(Exit(0))
			})
		})
	})
})

// https://golang.org/pkg/crypto/x509/#Certificate
// prefix should be "Certificate" or "Ca"
func CertFromPem(input string, ca bool) *x509.Certificate {
	type certificateValue struct {
		Ca          string `yaml:"ca,omitempty"`
		Certificate string `yaml:"certificate,omitempty"`
	}
	type certificate struct {
		Value certificateValue `yaml:"value"`
	}

	cert := certificate{}
	err := yaml.Unmarshal([]byte(input), &cert)

	if err != nil {
		panic("failed to parse certificate: " + err.Error())
	}

	var pemCert string
	if ca {
		pemCert = cert.Value.Ca
	} else {
		pemCert = cert.Value.Certificate
	}

	block, _ := pem.Decode([]byte(pemCert))
	if block == nil {
		panic("failed to parse certificate PEM")
	}
	parsed_cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		panic("failed to parse certificate: " + err.Error())
	}
	return parsed_cert
}
