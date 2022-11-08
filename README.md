# acls-in-yaml

Collect user ACLs from SaaS platforms and export them to YAML files.

![acls-in-yaml](images/logo-small.png?raw=true "acls-in-yaml logo")

acls-in-yaml is designed to make regular access control audits easy by
offering a familiar standardized format (YAML) for easy reviews and diffing.

The output is optimized for being reviewed by humans within a Github PR periodically,
and is carefully tuned to make policy drift over time easy to notice.

## Supported Data Sources

* Ghost Blog Staff (HTML)
* Github Org Members (CSV)
* Google Cloud Platform (gcloud)
* Google Workspace (CSV)
* Kolide (CSV)
* Secureframe (CSV)
* Slack (CSV)
* Vercel (HTML)
* Webflow (HTML)

## Requirements

* The Go Programming Language

## Installation

```shell
go install github.com/chainguard-dev/acls-in-yaml@latest
```

## Sample Output

This is the output of `acls-in-yaml --vercel-members-html=</path/to/members.html>`:

```yaml
metadata:
    kind: vercel_members
    name: Vercel Members
    source_date: "2022-09-21"
    generated_at: 2022-09-21T17:01:57.546028-07:00
    generated_by: t
    process:
        - Open https://vercel.com/
        - Select your company/team
        - Click 'Settings'
        - Click 'Members'
        - Save this page (Complete)
        - Collect resulting .html file for analysis (the other files are not necessary)
        - Execute 'axsdump --vercel-members-html=Members - Team Settings – Dashboard – Vercel.html'
user_count: 7
users:
    - account: john@chainguard.dev
      role: Member

    - account: kamelot@chainguard.dev
      role: Member

    - account: t@chainguard.dev
      role: Owner
role_count: 2
roles:
    Member:
        - john@chainguard.dev
        - kamelot@chainguard.dev
    Owner:
        - t@chainguard.dev
```

## Example command-line

Turn a pile of CSV and HTML pages into a directory full of easily auditable YAML files.

```shell
acls-in-yaml \
  --google-workspace-users-csv=User_Download_09082022_132441.csv \
  --google-workspace-audit-csv=users_logs_1660017600000.csv \
  --github-org-members-csv=export-github-1660070616.csv \
  --slack-members-csv="slack-members (3).csv" \
  --kolide-users-csv=kolide.csv \
  --out-dir=/path/to/github/repo
```

You can also pass in a single input file at a time.

## Usage

`acls-in-yaml` takes many flags, though for most cases it is only necessary to pass one in at a time:

```
 -gcp-iam-projects string
     Comma-separated list of GCP projects to fetch IAM policies for
  -gcp-identity-project string
     Optional GCP project for group resolution (requires cloudidentity API)
  -ghost-staff-html string
     Path to Ghost Staff HTML
     Steps:
       * Open the corporate Ghost blog
       * Click 'Settings'
       * Click 'Staff'
       * Zoom out so that all users are visible on one screen
       * Save this page (Complete)
       * Collect resulting .html file for analysis (the other files are not necessary)
  -github-org-members-csv string
     Path to Github Org Members CSV
     Steps:
       * Open https://github.com/orgs/<org>/people
       * Click Export
       * Select 'CSV'
       * Download resulting CSV file for analysis
  -google-workspace-audit-csv string
     Path to Google Workspace Audit CSV (delayed).
     Steps:
       * Open https://admin.google.com/ac/reporting/report/user/accounts
       * Click Download icon
       * Select All Columns
       * Click CSV
       * Download resulting CSV file for analysis
  -google-workspace-users-csv string
     Path to Google Workspace Users CSV (live)
     Steps:
       * Open https://admin.google.com/ac/users
       * Click Download users
       * Select 'All user info Columns'
       * Select 'Comma-separated values (.csv)'
       * Download resulting CSV file for analysis
  -kolide-users-csv string
     Path to Kolide Users CSV
     Steps:
       * Open https://k2.kolide.com/3361/settings/admin/users
       * Click CSV
       * Download resulting CSV file for analysis
  -out-dir string
     output YAML files to this directory
  -secureframe-personnel-csv string
     Path to Secureframe Personnel CSV
     Steps:
       * Open https://app.secureframe.com/personnel
       * Deselect any active filters
       * Click Export...
       * Select 'Direct Download'
       * Download resulting CSV file for analysis
  -slack-members-csv string
     Path to Slack Members CSV
     Steps:
       * Open Slack
       * Click <org name>▼
       * Select 'Settings & Administration'
       * Select 'Manage Members'
       * Select 'Export Member List'
       * Download resulting CSV file for analysis
  -vercel-members-html string
     Path to Vercel Members HTML
     Steps:
       * Open https://vercel.com/
       * Select your company/team
       * Click 'Settings'
       * Click 'Members'
       * Save this page (Complete)
       * Collect resulting .html file for analysis (the other files are not necessary)
  -webflow-members-html string
     Path to Ghost Members HTML
     Steps:
       * Open https://webflow.com/dashboard/sites/<site>/members
       * Save this page (Complete)
       * Collect resulting .html file for analysis (the other files are not necessary)
```

## FAQ

### Why not use the APIs provided by each vendor?

We'd like to add more direct API support (HELP WANRTED)!

The current structure was put in place because of a separation of duties,
where the person running this tool was not the one who had admin access to each SaaS platform.
