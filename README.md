# acls-to-yaml

Collect ACLs from a variety of data sources (mostly SaaS vendors), and output them into YAML files for review.

This helps with automating data collection for periodic user access reviews, and allows you to see drift over time.

## Usage

Turn a pile of CSV and HTML pages into a directory full of easily auditable YAML files.

```shell
go run . \
  --google-workspace-users-csv=$HOME/Downloads/User_Download.csv \
  --google-workspace-audit-csv=$HOME/Downloads/users_logs.csv \
  --github-org-members-csv=/home/t/Downloads/export.csv \
  --slack-members-csv="$HOME/Downloads/slack-members.csv" \
  --kolide-users-csv=$HOME/Downloads/kolide.csv \
  --out-dir=/tmp
```

## Supported Data Sources

* Ghost Blog Staff (HTML)
* Github Org Members (CSV)
* Google Workspace (CSV)
* Google Cloud Platform (gcloud)
* Kolide (CSV)
* Secureframe (CSV)
* Slack (CSV)
* Vercel (HTML)
* Webflow (HTML)
