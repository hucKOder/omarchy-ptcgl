#!/bin/bash

set -euo pipefail

fail() { printf 'Release protection check failed: %s\n' "$*" >&2; exit 1; }
public_metadata=false
if (( $# == 1 )) && [[ $1 == --public-metadata ]]; then
  public_metadata=true
elif (( $# != 0 )); then
  fail 'Usage: check-release-policy.sh [--public-metadata]'
fi
[[ ${GITHUB_REPOSITORY:-} == hucKOder/omarchy-ptcgl ]] || fail 'Unexpected release repository.'
[[ ${GITHUB_EVENT_NAME:-} == push && ${GITHUB_REF:-} =~ ^refs/tags/v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$ ]] || fail 'A version-tag push is required.'

work_dir=$(mktemp -d "${TMPDIR:-/tmp}/ptcgl-release-policy.XXXXXXXX")
trap 'rm -rf -- "$work_dir"' EXIT
base="repos/$GITHUB_REPOSITORY"

# GitHub hides bypass_actors from read-only tokens. The hosted metadata gate
# explicitly defers that field to the owner's full check before release approval;
# the default local check still fails closed when bypass data is unavailable.
gh api --paginate --slurp "$base/rulesets" > "$work_dir/rulesets.json" || fail 'Could not inspect tag rulesets.'
mapfile -t rule_ids < <(jq -r '.[][] | select((.target == "tag" or .target == "branch") and .enforcement == "active") | .id' "$work_dir/rulesets.json")
main_protected=false
tags_protected=false
for id in "${rule_ids[@]}"; do
  [[ $id =~ ^[0-9]+$ ]] || fail 'Invalid ruleset identity.'
  gh api "$base/rulesets/$id" > "$work_dir/ruleset.json" || fail 'Could not inspect a tag ruleset.'
  if jq -e --argjson public_metadata "$public_metadata" '
    .target == "branch" and .enforcement == "active" and
    (if has("bypass_actors") then (.bypass_actors | type == "array" and length == 0) else $public_metadata end) and
    (.conditions.ref_name.exclude | type == "array" and length == 0) and
    (.conditions.ref_name.include | any(. == "refs/heads/main" or . == "~ALL")) and
    (.rules | any(.type == "non_fast_forward")) and (.rules | any(.type == "deletion")) and
    (.rules | any(.type == "pull_request" and
      .parameters.required_approving_review_count == 0 and
      .parameters.require_code_owner_review == false and
      .parameters.dismiss_stale_reviews_on_push == true and
      .parameters.require_last_push_approval == false and
      .parameters.required_review_thread_resolution == true)) and
    (.rules | any(.type == "required_status_checks" and
      .parameters.strict_required_status_checks_policy == true and
      (.parameters.required_status_checks | any(.context == "test" and .integration_id == 15368)) and
      (.parameters.required_status_checks | any(.context == "vulnerabilities" and .integration_id == 15368))))
  ' "$work_dir/ruleset.json" >/dev/null; then
    main_protected=true
  fi
  if jq -e --argjson public_metadata "$public_metadata" '
    .target == "tag" and .enforcement == "active" and
    (if has("bypass_actors") then (.bypass_actors | type == "array" and length == 0) else $public_metadata end) and
    (.conditions.ref_name.exclude | type == "array" and length == 0) and
    (.conditions.ref_name.include | any(. == "refs/tags/v*" or . == "~ALL")) and
    (.rules | any(.type == "update")) and (.rules | any(.type == "deletion"))
  ' "$work_dir/ruleset.json" >/dev/null; then
    tags_protected=true
  fi
done
[[ $main_protected == true ]] || fail 'An active, no-bypass ruleset explicitly covering main must enforce solo-maintainer PRs, resolved review threads, GitHub Actions-bound test/vulnerabilities checks, and prevent force-push/deletion.'
[[ $tags_protected == true ]] || fail 'An active, no-bypass ruleset must prevent updates/deletion of v* tags.'

gh api "$base/environments/release" > "$work_dir/environment.json" || fail 'Configure the release environment before publishing.'
jq -e '
  .can_admins_bypass == false and
  (.protection_rules | any(.type == "required_reviewers" and .prevent_self_review == false and
    (.reviewers | any(.type == "User" and .reviewer.login == "hucKOder")))) and
  .deployment_branch_policy.custom_branch_policies == true
' "$work_dir/environment.json" >/dev/null || fail 'The release environment must require maintainer approval, permit the sole maintainer to approve their release, disable admin bypass, and use custom tag restrictions.'
gh api --paginate --slurp "$base/environments/release/deployment-branch-policies" > "$work_dir/branches.json" || fail 'Could not inspect release deployment restrictions.'
jq -e '
  [.[].branch_policies[]] | length > 0 and all(.type == "tag" and .name == "v*")
' "$work_dir/branches.json" >/dev/null || fail 'The release environment must allow only v* tags.'

if [[ $public_metadata == true ]]; then
  printf 'Public release-protection metadata passed. The owner must pass the full no-bypass check before approving publication.\n'
else
  printf 'Full release source, tag, environment, and no-bypass protections passed.\n'
fi
