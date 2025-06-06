# Prep-Waldo - PR Environment Automation Tool

Automates the manual process of updating krakend and gw-ingress configurations for PR environment testing.

## What it does

1. **Clones repos** - Fetches krakend and gw-ingress from their source repositories
2. **Updates krakend config** - Changes service host from `http://service` to `http://service-{pr}`
3. **Updates gw-ingress values** - Adds `instance: {pr}` and `endpointFullNameOverride: api-fyre`
4. **Creates git branches** - Creates feature branches for both repos
5. **Commits & pushes** - Commits changes and pushes to origin
6. **Provides PR links** - Shows direct GitHub links to create PRs manually

## Before (Manual Process)

1. Go to krakend repo, find endpoint in config.json
2. Update host field manually
3. Create PR, add Preview label
4. Wait for ArgoCD/Groundcover verification
5. Go to gw-ingress repo
6. Update values-staging-override.yaml with instance number
7. Create PR, add Preview label
8. Wait for deployment

## After (Automated)

```bash
prep-waldo 654 home-depot /v1/fair-lock mycompany
```

## Prerequisites

1. **Git access** - SSH keys or HTTPS authentication
2. **Repository access** - Read/write permissions to krakend and gw-ingress repos

## Usage

```bash
# Build the tool
go build -o prep-waldo main.go

# Run automation  
./prep-waldo <pr-number> <service-name> <endpoint-url> <repo-org>

# Example
./prep-waldo 654 home-depot /v1/fair-lock mycompany
```

## Repository Structure Expected

```
mycompany/krakend/
├── config.json              # Main krakend configuration
└── ...

mycompany/gw-ingress/
├── kube/
│   └── values-staging-override.yaml
└── ...
```

## Example Output

```
Automating PR env setup for:
  PR: 654
  Service: home-depot
  Endpoint: /app/v1/fair-lock
  Org: mycompany

Setting up workspace: prep-waldo-workspace-654
Cloning krakend repo: https://github.com/mycompany/krakend.git
Cloning gw-ingress repo: https://github.com/mycompany/gw-ingress.git
✅ Workspace setup complete

Updating krakend config for home-depot-654
Found target endpoint: /app/v1/fair-lock
Updated host: http://home-depot -> http://home-depot-654
✅ Successfully updated config.json

Updating gw-ingress values with instance: 654
Added endpointFullNameOverride: api-fyre
Added instance: 654 to krakend section
✅ Successfully updated values-staging-override.yaml

Creating krakend branch and pushing changes...
✅ Branch created and pushed: update-home-depot-pr-654
   Create PR manually at: https://github.com/mycompany/krakend/compare/update-home-depot-pr-654
   Title: Update home-depot endpoint for PR-654
   Add 'Preview' label after creating the PR

Creating gw-ingress branch and pushing changes...
✅ Branch created and pushed: update-instance-pr-654
   Create PR manually at: https://github.com/mycompany/gw-ingress/compare/update-instance-pr-654
   Title: Update krakend instance for PR-654
   Add 'Preview' label after creating the PR
```

## What happens next

1. ArgoCD picks up the PRs with Preview labels
2. Deployments start automatically
3. You can verify in Groundcover
4. Test your changes in the PR environment
5. When done, merge/close the PRs to clean up

## Troubleshooting

**Git clone fails:**
- Check repository URLs and access permissions
- Ensure SSH keys are set up or use HTTPS with token

**PR creation:**
- Click the provided GitHub compare links 
- Create PRs manually with the suggested titles
- Add 'Preview' labels to trigger deployments

**Config not found:**
- Verify repository structure matches expected paths
- Check if config.json exists in krakend repo root

## Implementation Details

- **Language**: Go (for simplicity and tooling)
- **Dependencies**: Standard library only
- **Workspace**: Creates temporary directory, cleans up after
- **Git operations**: Uses system git commands only
- **GitHub integration**: Provides compare links for manual PR creation
