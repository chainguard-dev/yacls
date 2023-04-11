#!/bin/sh
PROJECT="acl-auditor"
export KO_DOCKER_REPO="gcr.io/${PROJECT}/yacls"

gcloud run deploy yacls --image="$(ko publish --bare .)" --args=-serve \
  --region us-central1 --project "${PROJECT}"
