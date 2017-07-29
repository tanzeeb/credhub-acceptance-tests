#!/bin/bash

set -eu

if [[ -n "${BOSH_DEPLOYMENT:-}" ]]; then
    cat <<EOF > config.json
{
  "director_host":"${DIRECTOR_IP}:22",
  "api_url": "https://${CREDHUB_IP}:8844",
  "api_username":"${CREDHUB_USERNAME}",
  "api_password":"${CREDHUB_PASSWORD}",
  "bosh": {
    "host":"${API_IP}:22",
    "director_username":"${BOSH_DIRECTOR_USERNAME}",
    "director_password":"${BOSH_DIRECTOR_PASSWORD}",
    "deployment":"${BOSH_DEPLOYMENT}"
  },
  "credential_root":"${CREDENTIAL_ROOT}",
  "uaa_ca":"${UAA_CA}"
}
EOF
else
    cat <<EOF > config.json
{
  "director_host":"${DIRECTOR_IP}",
  "api_url": "https://${CREDHUB_IP}:8844",
  "api_username":"${CREDHUB_USERNAME}",
  "api_password":"${CREDHUB_PASSWORD}",
  "bosh": {
    "host":"${DIRECTOR_IP}:22",
    "bosh_ssh_username":"${BOSH_SSH_USERNAME}",
    "bosh_ssh_private_key_path":"${BOSH_SSH_PRIVATE_KEY_PATH}"
  },
  "uaa_ca":"${UAA_CA}"
}
EOF
fi

ginkgo -r bbr_integration_test
