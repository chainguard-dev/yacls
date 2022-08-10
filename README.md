# axsdump

Automate data collection for user access reviews.

## Usage

Turn a pile of CSV and HTML pages into a directory full of easily auditable YAML files.

```shell
go run . \
  --google-workspace-users-csv=$HOME/Downloads/User_Download_09082022_132441.csv \
  --google-workspace-audit-csv=$HOME/Downloads/users_logs_1660017600000.csv \
  --github-org-members-csv=/home/t/Downloads/export-chainguard-dev-1660070616.csv \
  --slack-members-csv="$HOME/Downloads/slack-chainguard-dev-members (3).csv" \
  --kolide-users-csv=$HOME/Downloads/Users\ \ Access\ ·\ Kolide\ \(2\).csv \
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
