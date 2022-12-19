#!/bin/sh
PROJECT="acl-auditor"
export KO_DOCKER_REPO="gcr.io/${PROJECT}/acls-in-yaml"

gcloud run deploy acls-in-yaml --image="$(ko publish --bare .)" --args=-serve \
  --region us-central1 --project "${PROJECT}"
