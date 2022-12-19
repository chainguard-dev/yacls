# acls-in-yaml

Collect user ACLs from SaaS platforms and export them to YAML files optimized for readability.

![acls-in-yaml](images/logo-small.png?raw=true "acls-in-yaml logo")

acls-in-yaml is designed to make regular access control audits easy by
offering a familiar standardized format (YAML) for easy reviews and diffing.

The output is optimized for being reviewed by humans within a Github PR periodically
and is carefully tuned to make policy drift easy to notice.

## Supported Data Sources

* 1Password (CSV)
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

This is the output of `acls-in-yaml --vercel-html=</path/to/members.html>`:

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
        - Execute 'acls-in-yaml --vercel-members-html=Members - Team Settings – Dashboard – Vercel.html'
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
  --google-users-csv=User_Download.csv \
  --google-audit-csv=users_logs.csv \
  --github-org-csv=export-github.csv \
  --slack-csv=members.csv \
  --kolide-csv=kolide.csv \
  --out-dir=/path/to/github/repo
```

You can also pass in a single input file at a time.

## Usage

Flags for `acls-in-yaml`:

```yaml
  -gcp-identity-project string
     project to use for GCP Cloud Identity lookups
  -input string
     path to input file
  -kind string
     kind of input to process. valid values:
       * 1password
       * gcp
       * ghost
       * github-org
       * google-workspace-audit
       * google-workspace-users
       * kolide
       * secureframe
       * slack
       * vercel
       * webflow

     Detailed steps for each kind:

     # Ghost Blog Permissions

      * Open the corporate Ghost blog
      * Click 'Settings'
      * Click 'Staff'
      * Zoom out so that all users are visible on one screen
      * Save this page (Complete)
      * Collect resulting .html file for analysis (the other files are not necessary)
      * Execute 'acls-in-yaml --kind={{.Kind}} --input={{.Path}}'

     # Github Organization Members

      * Open https://github.com/orgs/<org>/people
      * Click Export
      * Select 'CSV'
      * Download resulting CSV file for analysis
      * Execute 'acls-in-yaml --kind={{.Kind}} --input={{.Path}}'

     # Google Cloud Project IAM Policies

      * Execute 'acls-in-yaml --kind={{.Kind}} --project={{.Project}}'

     # Google Workspace User Audit

      * Open https://admin.google.com/ac/reporting/report/user/accounts
      * Click Download icon
      * Select All Columns
      * Click CSV
      * Download resulting CSV file for analysis
      * Execute 'acls-in-yaml --kind={{.Kind}} --input={{.Path}}'

     # Google Workspace Users

      * Open https://admin.google.com/ac/users
      * Click Download users
      * Select 'All user info Columns'
      * Select 'Comma-separated values (.csv)'
      * Download resulting CSV file for analysis
      * Execute 'acls-in-yaml --kind={{.Kind}} --input={{.Path}}'

     # Kolide Users

      * Open https://k2.kolide.com/3361/settings/admin/users
      * Click CSV
      * Download resulting CSV file for analysis
      * Execute 'acls-in-yaml --kind={{.Kind}} --input={{.Path}}'

     # 1Password Team Members

      * To be documented
      * Download resulting CSV file for analysis
      * Execute 'acls-in-yaml --kind={{.Kind}} --input={{.Path}}'

     # Secureframe Personnel

      * Open https://app.secureframe.com/personnel
      * Deselect any active filters
      * Click Export...
      * Select 'Direct Download'
      * Download resulting CSV file for analysis
      * Execute 'acls-in-yaml --kind={{.Kind}} --input={{.Path}}'

     # Slack Members

      * Open Slack
      * Click <org name>▼
      * Select 'Settings & Administration'
      * Select 'Manage Members'
      * Select 'Export Member List'
      * Download resulting CSV file for analysis
      * Execute 'acls-in-yaml --kind={{.Kind}} --input={{.Path}}'

     # Vercel Site Permissions

      * Open https://vercel.com/
      * Select your company/team
      * Click 'Settings'
      * Click 'Members'
      * Save this page (Complete)
      * Collect resulting .html file for analysis (the other files are not necessary)
      * Execute 'acls-in-yaml --kind={{.Kind}} --input={{.Path}}'

     # Webflow Site Permissions

      * Open https://webflow.com/dashboard/sites/<site>/members
      * Save this page (Complete)
      * Collect resulting .html file for analysis (the other files are not necessary)
      * Execute 'acls-in-yaml --kind={{.Kind}} --input={{.Path}}'

  -out-dir string
     output YAML files to this directory
  -project string
     specific project to process within the kind
  -serve
     Enable server mode (web UI)
```

## FAQ

### Why not use the APIs provided by each vendor?

The current structure was put in place because of a separation of duties, where the person running the tool was not the one who had admin access to each SaaS platform. It doesn't help that many SaaS platforms do not provide a documented API to retrieve user lists (Vercel, I'm looking at you!)

At the moment, the only fully automated audit is GCP, though we would like to add more direct API support. HELP WANTED!
