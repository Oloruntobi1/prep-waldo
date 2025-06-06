package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

func main() {
	if len(os.Args) < 4 {
		fmt.Println("Usage: prep-waldo <pr-number> <service-name> <endpoint-url> <repo-org>")
		fmt.Println("Example: prep-waldo 654 home-depot /v1/fair-lock mycompany")
		fmt.Println("  Note: endpoint-url will be prefixed with '/app' automatically")
		fmt.Println("  Requires: git access to <repo-org>/krakend and <repo-org>/gw-ingress repos")
		return
	}

	prNumber := os.Args[1]
	serviceName := os.Args[2]
	endpointURL := os.Args[3]
	repoOrg := os.Args[4]

	endpointURL = "/app" + endpointURL

	fmt.Println("Automating PR env setup for:")
	fmt.Printf("  PR: %s\n", prNumber)
	fmt.Printf("  Service: %s\n", serviceName)
	fmt.Printf("  Endpoint: %s\n", endpointURL)
	fmt.Printf("  Org: %s\n", repoOrg)

	// Step 1: Setup workspace and fetch repos
	workspaceDir := fmt.Sprintf("prep-waldo-workspace-%s", prNumber)
	if err := setupWorkspace(workspaceDir, repoOrg); err != nil {
		log.Fatalf("Failed to setup workspace: %v", err)
	}
	defer cleanupWorkspace(workspaceDir)

	// Step 2: Update configs in their respective repos
	krakendDir := filepath.Join(workspaceDir, "krakend")
	gwIngressDir := filepath.Join(workspaceDir, "gw-ingress")

	if err := updateKrakendConfig(krakendDir, prNumber, serviceName, endpointURL); err != nil {
		log.Printf("Error updating krakend config: %v", err)
	}

	if err := updateGwIngressValues(gwIngressDir, prNumber); err != nil {
		log.Printf("Error updating gw-ingress values: %v", err)
	}

	// Step 3: Create git branches and push changes
	if err := createKrakendBranch(krakendDir, repoOrg, prNumber, serviceName); err != nil {
		log.Printf("Error creating krakend branch: %v", err)
	}

	if err := createGwIngressBranch(gwIngressDir, repoOrg, prNumber); err != nil {
		log.Printf("Error creating gw-ingress branch: %v", err)
	}
}

func setupWorkspace(workspaceDir, repoOrg string) error {
	fmt.Printf("Setting up workspace: %s\n", workspaceDir)

	// Create workspace directory
	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		return fmt.Errorf("failed to create workspace: %v", err)
	}

	// Clone krakend repo
	krakendRepo := fmt.Sprintf("https://github.com/%s/krakend.git", repoOrg)
	krakendDir := filepath.Join(workspaceDir, "krakend")
	fmt.Printf("Cloning krakend repo: %s\n", krakendRepo)

	if err := exec.Command("git", "clone", krakendRepo, krakendDir).Run(); err != nil {
		return fmt.Errorf("failed to clone krakend repo: %v", err)
	}

	// Clone gw-ingress repo
	gwIngressRepo := fmt.Sprintf("https://github.com/%s/gw-ingress.git", repoOrg)
	gwIngressDir := filepath.Join(workspaceDir, "gw-ingress")
	fmt.Printf("Cloning gw-ingress repo: %s\n", gwIngressRepo)

	if err := exec.Command("git", "clone", gwIngressRepo, gwIngressDir).Run(); err != nil {
		return fmt.Errorf("failed to clone gw-ingress repo: %v", err)
	}

	fmt.Println("✅ Workspace setup complete")
	return nil
}

func cleanupWorkspace(workspaceDir string) {
	fmt.Printf("Cleaning up workspace: %s\n", workspaceDir)
	if err := os.RemoveAll(workspaceDir); err != nil {
		log.Printf("Warning: failed to cleanup workspace: %v", err)
	}
}

func updateKrakendConfig(repoDir, prNumber, serviceName, endpointURL string) error {
	fmt.Printf("Updating krakend config for %s-%s\n", serviceName, prNumber)

	configPath := filepath.Join(repoDir, "config.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read config.json: %v", err)
	}

	content := string(data)

	targetHost := fmt.Sprintf(`"http://%s"`, serviceName)
	newHost := fmt.Sprintf(`"http://%s-%s"`, serviceName, prNumber)

	// Find and verify the endpoint exists
	endpointPattern := fmt.Sprintf(`"endpoint":\s*"%s"`, regexp.QuoteMeta(endpointURL))
	if !regexp.MustCompile(endpointPattern).MatchString(content) {
		return fmt.Errorf("endpoint %s not found in config.json", endpointURL)
	}

	fmt.Printf("Found target endpoint: %s\n", endpointURL)

	if strings.Contains(content, targetHost) {
		originalContent := content
		content = strings.Replace(content, targetHost, newHost, 1)

		// Verify we actually made a change
		if content != originalContent {
			fmt.Printf("Updated host: %s -> %s\n", targetHost, newHost)
		} else {
			return fmt.Errorf("failed to update host %s", targetHost)
		}
	} else {
		return fmt.Errorf("host %s not found in config.json", targetHost)
	}

	// Write back the content with minimal changes
	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		return err
	}

	fmt.Printf("✅ Successfully updated %s\n", configPath)
	return nil
}

func updateGwIngressValues(repoDir, prNumber string) error {
	fmt.Printf("Updating gw-ingress values with instance: %s\n", prNumber)

	valuesPath := filepath.Join(repoDir, "kube", "values-staging-override.yaml")
	data, err := os.ReadFile(valuesPath)
	if err != nil {
		return fmt.Errorf("failed to read values file: %v", err)
	}

	lines := strings.Split(string(data), "\n")
	var result []string
	inKrakendSection := false
	updated := false

	addedEndpointOverride := false

	for _, line := range lines {
		// Add endpointFullNameOverride after albIdleTimeout
		if strings.Contains(line, "albIdleTimeout: 180") && !addedEndpointOverride {
			result = append(result, line)
			result = append(result, "  endpointFullNameOverride: api-fyre")
			addedEndpointOverride = true
			fmt.Println("Added endpointFullNameOverride: api-fyre")
		} else if strings.Contains(line, "krakend:") {
			inKrakendSection = true
			result = append(result, line)

			// Add the instance line after krakend:
			instanceLine := fmt.Sprintf("      instance: %s", prNumber)
			result = append(result, instanceLine)
			updated = true
			fmt.Printf("Added instance: %s to krakend section\n", prNumber)
		} else if inKrakendSection && strings.Contains(line, "instance:") {
			// Skip existing instance line, we already added the new one
			continue
		} else {
			result = append(result, line)

			// Check if we're leaving the krakend section
			if inKrakendSection && len(line) > 0 && line[0] != ' ' {
				inKrakendSection = false
			}
		}
	}

	if !updated {
		return fmt.Errorf("no krakend section found in values file")
	}

	// Write updated content back
	updatedContent := strings.Join(result, "\n")
	if err := os.WriteFile(valuesPath, []byte(updatedContent), 0o644); err != nil {
		return err
	}

	fmt.Printf("✅ Successfully updated %s\n", valuesPath)
	return nil
}

func createKrakendBranch(repoDir, repoOrg, prNumber, serviceName string) error {
	fmt.Println("Creating krakend branch and pushing changes...")

	// Change to repo directory
	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)

	if err := os.Chdir(repoDir); err != nil {
		return err
	}

	// Create branch, commit and push changes
	if err := runGitCommands("krakend", prNumber, serviceName); err != nil {
		return err
	}

	branchName := fmt.Sprintf("update-%s-pr-%s", serviceName, prNumber)
	fmt.Printf("✅ Branch created and pushed: %s\n", branchName)
	fmt.Printf("   Create PR manually at: https://github.com/%s/krakend/compare/%s\n", repoOrg, branchName)
	fmt.Printf("   Title: Update %s endpoint for PR-%s\n", serviceName, prNumber)
	fmt.Printf("   Add 'Preview' label after creating the PR\n")

	return nil
}

func createGwIngressBranch(repoDir, repoOrg, prNumber string) error {
	fmt.Println("Creating gw-ingress branch and pushing changes...")

	// Change to repo directory
	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)

	if err := os.Chdir(repoDir); err != nil {
		return err
	}

	// Create branch, commit and push changes
	if err := runGitCommands("gw-ingress", prNumber, ""); err != nil {
		return err
	}

	branchName := fmt.Sprintf("update-instance-pr-%s", prNumber)
	fmt.Printf("✅ Branch created and pushed: %s\n", branchName)
	fmt.Printf("   Create PR manually at: https://github.com/%s/gw-ingress/compare/%s\n", repoOrg, branchName)
	fmt.Printf("   Title: Update krakend instance for PR-%s\n", prNumber)
	fmt.Printf("   Add 'Preview' label after creating the PR\n")

	return nil
}

func runGitCommands(repoType, prNumber, serviceName string) error {
	var branchName string
	var commitMsg string

	if repoType == "krakend" {
		branchName = fmt.Sprintf("update-%s-pr-%s", serviceName, prNumber)
		commitMsg = fmt.Sprintf("Update %s host for PR-%s", serviceName, prNumber)
	} else {
		branchName = fmt.Sprintf("update-instance-pr-%s", prNumber)
		commitMsg = fmt.Sprintf("Add krakend instance %s", prNumber)
	}

	fmt.Printf("Creating branch: %s\n", branchName)
	if err := exec.Command("git", "checkout", "-b", branchName).Run(); err != nil {
		return fmt.Errorf("git checkout failed: %v", err)
	}

	fmt.Println("Adding changes...")
	if err := exec.Command("git", "add", ".").Run(); err != nil {
		return fmt.Errorf("git add failed: %v", err)
	}

	fmt.Println("Committing changes...")
	cmd := exec.Command("git", "commit", "-m", commitMsg)
	if output, err := cmd.CombinedOutput(); err != nil {
		// Check if it's because there are no changes to commit
		if strings.Contains(string(output), "nothing to commit") {
			fmt.Println("No changes to commit - files may already be up to date")
			return nil
		}
		return fmt.Errorf("git commit failed: %v\nOutput: %s", err, string(output))
	}

	fmt.Println("Pushing branch...")
	if err := exec.Command("git", "push", "origin", branchName).Run(); err != nil {
		return fmt.Errorf("git push failed: %v", err)
	}

	return nil
}
