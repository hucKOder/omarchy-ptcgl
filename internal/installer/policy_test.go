package installer_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestReleasePolicyFailsClosed(t *testing.T) {
	script, err := os.ReadFile("../../scripts/check-release-policy.sh")
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"protected", "wrong repository", "wrong ref", "API failure", "admin bypass", "default branch alias", "no review", "unmergeable self review", "stale approvals", "unresolved conversations", "no CI", "unbound CI", "wrong CI integration", "mutable tags", "tag bypass", "excluded tags", "unprotected environment", "self approval blocked", "wrong release reviewer", "admin release bypass", "branch deployment", "hidden bypass", "public hidden bypass", "public admin bypass", "public tag bypass", "public no CI", "unknown option"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			bin := filepath.Join(root, "bin")
			main := map[string]any{
				"target": "branch", "enforcement": "active", "bypass_actors": []any{},
				"conditions": map[string]any{"ref_name": map[string]any{"include": []string{"refs/heads/main"}, "exclude": []string{}}},
			}
			if mode == "admin bypass" || mode == "public admin bypass" {
				main["bypass_actors"] = []any{map[string]any{"actor_type": "RepositoryRole", "actor_id": 5}}
			}
			if mode == "default branch alias" {
				main["conditions"] = map[string]any{"ref_name": map[string]any{"include": []string{"~DEFAULT_BRANCH"}, "exclude": []string{}}}
			}
			mainRules := []any{map[string]any{"type": "non_fast_forward"}, map[string]any{"type": "deletion"}}
			if mode != "no review" {
				approvals := 0
				if mode == "unmergeable self review" {
					approvals = 1
				}
				mainRules = append(mainRules, map[string]any{"type": "pull_request", "parameters": map[string]any{
					"required_approving_review_count": approvals, "require_code_owner_review": false,
					"dismiss_stale_reviews_on_push": mode != "stale approvals", "require_last_push_approval": false,
					"required_review_thread_resolution": mode != "unresolved conversations",
				}})
			}
			if mode != "no CI" && mode != "public no CI" {
				checks := []any{map[string]any{"context": "test", "integration_id": 15368}, map[string]any{"context": "vulnerabilities", "integration_id": 15368}}
				if mode == "unbound CI" {
					delete(checks[0].(map[string]any), "integration_id")
				}
				if mode == "wrong CI integration" {
					checks[1].(map[string]any)["integration_id"] = 12345
				}
				mainRules = append(mainRules, map[string]any{"type": "required_status_checks", "parameters": map[string]any{
					"strict_required_status_checks_policy": true,
					"required_status_checks":               checks,
				}})
			}
			main["rules"] = mainRules
			ruleset := map[string]any{
				"target": "tag", "enforcement": "active", "bypass_actors": []any{},
				"conditions": map[string]any{"ref_name": map[string]any{"include": []string{"refs/tags/v*"}, "exclude": []string{}}},
				"rules":      []any{map[string]any{"type": "update"}, map[string]any{"type": "deletion"}},
			}
			if mode == "mutable tags" {
				ruleset["rules"] = []any{}
			}
			if mode == "tag bypass" || mode == "public tag bypass" {
				ruleset["bypass_actors"] = []any{map[string]any{"actor_type": "RepositoryRole", "actor_id": 5}}
			}
			if mode == "excluded tags" {
				ruleset["conditions"] = map[string]any{"ref_name": map[string]any{"include": []string{"refs/tags/v*"}, "exclude": []string{"refs/tags/v0.*"}}}
			}
			if mode == "hidden bypass" || mode == "public hidden bypass" {
				delete(main, "bypass_actors")
				delete(ruleset, "bypass_actors")
			}
			reviewer := "hucKOder"
			if mode == "wrong release reviewer" {
				reviewer = "someone-else"
			}
			environment := map[string]any{
				"can_admins_bypass":        mode == "admin release bypass",
				"protection_rules":         []any{map[string]any{"type": "required_reviewers", "prevent_self_review": mode == "self approval blocked", "reviewers": []any{map[string]any{"type": "User", "reviewer": map[string]any{"login": reviewer}}}}},
				"deployment_branch_policy": map[string]any{"custom_branch_policies": true},
			}
			if mode == "unprotected environment" {
				environment["protection_rules"] = []any{}
			}
			deploymentType := "tag"
			if mode == "branch deployment" {
				deploymentType = "branch"
			}
			for name, value := range map[string]any{
				"main": main, "ruleset": ruleset, "environment": environment,
				"rulesets": []any{[]any{map[string]any{"id": 1, "target": "branch", "enforcement": "active"}, map[string]any{"id": 2, "target": "tag", "enforcement": "active"}}},
				"branches": []any{map[string]any{"branch_policies": []any{map[string]any{"name": "v*", "type": deploymentType}}}},
			} {
				data, err := json.Marshal(value)
				if err != nil {
					t.Fatal(err)
				}
				fixture(t, filepath.Join(root, name+".json"), string(data), 0o600)
			}
			fixture(t, filepath.Join(bin, "gh"), `#!/bin/bash
set -euo pipefail
[[ $TEST_MODE != 'API failure' ]] || exit 1
case "${*: -1}" in
  */rulesets) name=rulesets ;;
  */rulesets/1) name=main ;;
  */rulesets/2) name=ruleset ;;
  */environments/release) name=environment ;;
  */deployment-branch-policies) name=branches ;;
  *) exit 2 ;;
esac
exec cat -- "$TEST_POLICY/$name.json"
`, 0o755)
			for _, tool := range []string{"bash", "jq", "mktemp", "rm", "cat"} {
				path, err := exec.LookPath(tool)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(path, filepath.Join(bin, tool)); err != nil {
					t.Fatal(err)
				}
			}
			repo, ref := "hucKOder/omarchy-ptcgl", "refs/tags/v0.4.1"
			if mode == "wrong repository" {
				repo = "attacker/omarchy-ptcgl"
			}
			if mode == "wrong ref" {
				ref = "refs/heads/main"
			}
			args := []string{"-e", "-s", "--"}
			if strings.HasPrefix(mode, "public ") {
				args = append(args, "--public-metadata")
			}
			if mode == "unknown option" {
				args = append(args, "--ignore-protections")
			}
			cmd := exec.Command("bash", args...)
			cmd.Stdin = strings.NewReader(string(script))
			cmd.Env = append(os.Environ(), "PATH="+bin, "TEST_POLICY="+root, "TEST_MODE="+mode,
				"GITHUB_REPOSITORY="+repo, "GITHUB_EVENT_NAME=push", "GITHUB_REF="+ref)
			output, err := cmd.CombinedOutput()
			if (err == nil) != (mode == "protected" || mode == "public hidden bypass") {
				t.Fatalf("policy result = %v\n%s", err, output)
			}
		})
	}
}
