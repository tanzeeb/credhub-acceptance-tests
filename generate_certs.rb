#!/usr/bin/env ruby

require 'fileutils'
require 'optparse'
require 'openssl'

cert_dir = File.join(FileUtils.pwd, 'certs/')

def setup_cert_dir(cert_dir)
  puts "initializing certificate directory"
  FileUtils.rm_rf(cert_dir)
  FileUtils.mkdir(cert_dir)
end

def generate_cert(issuer, valid_not_before, valid_not_after, key, signing_key)
  cert = OpenSSL::X509::Certificate.new
  cert.subject = OpenSSL::X509::Name.parse "/CN=credhub_test_client"
  cert.issuer = issuer || cert.subject
  cert.public_key = key.public_key
  cert.not_before = valid_not_before
  cert.not_after = valid_not_after
  cert.sign(signing_key, OpenSSL::Digest::SHA256.new)
  return cert.to_pem, key.to_pem
end

def generate_cert_with_ca(ca_cert_path, ca_key_path, valid_not_before, valid_not_after)
  rawCaCert = File.read(ca_cert_path)
  rawCaKey = File.read(ca_key_path)
  ca_cert = OpenSSL::X509::Certificate.new(rawCaCert)
  ca_key = OpenSSL::PKey::RSA.new(rawCaKey)
  key = OpenSSL::PKey::RSA.new(2048)
  generate_cert(ca_cert.subject, valid_not_before, valid_not_after, key, ca_key)
end

def generate_self_signed_cert(valid_not_before, valid_not_after)
  key = OpenSSL::PKey::RSA.new(2048)
  generate_cert(nil, valid_not_before, valid_not_after, key, key)
end

def create_pem_files(cert_pem, key_pem, cert_dir, file_base_name)
  cert_path = File.join(cert_dir, file_base_name + ".pem")
  key_path = File.join(cert_dir, file_base_name + "_key.pem")
  File.new(cert_path, "w").write(cert_pem)
  File.new(key_path, "w").write(key_pem)
end

def generate_valid_cert(ca_cert_path, ca_key_path, cert_dir)
  now = Time.now
  cert, key = generate_cert_with_ca(ca_cert_path, ca_key_path, now, now + 365 * 24 * 60 * 60) # valid for one year
  create_pem_files(cert, key, cert_dir, "client")
end

def generate_untrusted_cert(cert_dir)
  now = Time.now
  cert, key = generate_self_signed_cert(now, now + 365 * 24 * 60 * 60)
  create_pem_files(cert, key, cert_dir, "invalid")
end

def generate_expired_cert(ca_cert_path, ca_key_path, cert_dir)
  day = 24 * 60 * 60
  now = Time.now
  cert, key = generate_self_signed_cert(now - 2* day, now - 2)
  create_pem_files(cert, key, cert_dir, "expired")
end


if ARGV.size != 2
  puts "Usage: generate_certs.rb <PATH_TO_CA_CERTIFICATE> <PATH_TO_CA_PRIVATE_KEY>"
  exit
end

ca_cert_path = ARGV[0]
ca_key_path = ARGV[1]

setup_cert_dir(cert_dir)

generate_valid_cert(ca_cert_path, ca_key_path, cert_dir)
generate_untrusted_cert(cert_dir)
generate_expired_cert(ca_cert_path, ca_key_path, cert_dir)
# generate_not_yet_valid_cert(ca_cert_path, ca_key_path, cert_dir)

