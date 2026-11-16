#!/bin/sh
# Copyright 2025-2026 Richard Shade. Licensed under Apache-2.0.
# SPDX-License-Identifier: Apache-2.0
#
# Runs the cost-check flow when infrastructure code changed and prints the
# cost change and threshold status in at most five lines.
#
# Usage: cost-check-hook.sh <claude-stop|pre-commit|pre-push|manual>
#
# claude-stop  Claude Code Stop hook. Reads the hook JSON on stdin and answers
#              with hook JSON on stdout.
# pre-commit   git pre-commit hook. Checks staged files.
# pre-push     git pre-push hook. Checks the commits being pushed.
# manual       Checks uncommitted changes and prints the summary.
#
# Thresholds come from .cost-check.yml (see references/thresholds.md). A
# tooling problem never blocks: the hook says why it skipped and exits 0.

trigger="${1:-manual}"
case "$trigger" in
claude-stop | pre-commit | pre-push | manual) ;;
*)
	echo "cost-check-hook: unknown trigger '$trigger' (use claude-stop, pre-commit, pre-push, or manual)" >&2
	exit 0
	;;
esac

hook_input=""
if [ "$trigger" = "claude-stop" ] || [ "$trigger" = "pre-push" ]; then
	hook_input="$(cat)"
fi

stop_hook_active=false
if [ "$trigger" = "claude-stop" ]; then
	case "$hook_input" in
	*'"stop_hook_active":true'* | *'"stop_hook_active": true'*) stop_hook_active=true ;;
	esac
fi

have_jq=false
command -v jq >/dev/null 2>&1 && have_jq=true

# say prints a message where the trigger's caller will show it.
say() {
	if [ "$trigger" = "claude-stop" ]; then
		if [ "$have_jq" = true ]; then
			jq -cn --arg msg "$1" '{systemMessage: $msg}'
		else
			printf '%s\n' "$1" >&2
		fi
	else
		printf '%s\n' "$1"
	fi
}

skip() {
	say "FinFocus cost check skipped: $1"
	exit 0
}

root="$(git rev-parse --show-toplevel 2>/dev/null)" || exit 0
project_dir="${FINFOCUS_COST_HOOK_PROJECT_DIR:-$root}"
cd "$project_dir" 2>/dev/null || skip "project directory '$project_dir' does not exist"
project_dir="$(pwd)"
git_dir="$(git rev-parse --absolute-git-dir 2>/dev/null)" || exit 0
prefix="$(git rev-parse --show-prefix 2>/dev/null)"

if [ -f Pulumi.yaml ] || [ -f Pulumi.yml ]; then
	kind=pulumi
elif ls ./*.tf >/dev/null 2>&1; then
	kind=terraform
else
	exit 0
fi

# changed_files lists repository-relative paths the trigger covers.
changed_files() {
	case "$trigger" in
	pre-commit)
		git -C "$root" -c core.quotePath=false diff --cached --name-only
		;;
	pre-push)
		zero="0000000000000000000000000000000000000000"
		printf '%s\n' "$hook_input" | while read -r _ local_sha _ remote_sha; do
			[ -n "$local_sha" ] || continue
			[ "$local_sha" = "$zero" ] && continue
			if [ "$remote_sha" != "$zero" ] && [ -n "$remote_sha" ]; then
				git -C "$root" -c core.quotePath=false diff --name-only "$remote_sha" "$local_sha"
			else
				# A new branch: every commit no remote has yet, root commit included.
				git -C "$root" -c core.quotePath=false log --format= --name-only "$local_sha" --not --remotes
			fi
		done
		;;
	*)
		git -C "$root" -c core.quotePath=false diff --name-only HEAD 2>/dev/null
		git -C "$root" -c core.quotePath=false ls-files --others --exclude-standard --full-name
		;;
	esac
}

# is_iac reports whether a repository-relative path is infrastructure code of
# this project.
is_iac() {
	case "$1" in
	"$prefix"*) ;;
	*) return 1 ;;
	esac
	rel="${1#"$prefix"}"
	case "$rel" in
	node_modules/* | */node_modules/* | .terraform/* | */.terraform/* | .github/* | *.md) return 1 ;;
	esac
	if [ "$kind" = pulumi ]; then
		case "$rel" in
		*Pulumi*.yaml | *Pulumi*.yml | *.ts | *.tsx | *.js | *.mjs | *.cjs | *.py | *.go | *.cs | *.fs | *.vb | *.java | *.yaml | *.yml) return 0 ;;
		esac
	else
		case "$rel" in
		*.tf | *.tf.json | *.tfvars) return 0 ;;
		esac
	fi
	return 1
}

iac_changes="$(changed_files | sort -u | while IFS= read -r path; do
	[ -n "$path" ] && is_iac "$path" && printf '%s\n' "$path"
done)"
[ -n "$iac_changes" ] || exit 0

config=".cost-check.yml"

# cfg prints a value from .cost-check.yml: cfg <section or ""> <key> <default>.
cfg() {
	value=""
	if [ -f "$config" ]; then
		value="$(awk -v sec="$1" -v key="$2" '
			/^[[:space:]]*#/ || /^[[:space:]]*$/ { next }
			/^[^[:space:]]/ {
				name = $0; sub(/:.*/, "", name); current = name
				if (sec == "" && name == key) { found = $0 }
				next
			}
			sec != "" && current == sec {
				line = $0; sub(/^[[:space:]]+/, "", line)
				name = line; sub(/[[:space:]]*:.*/, "", name)
				if (name == key) { found = line }
			}
			END {
				if (found == "") { exit }
				sub(/^[^:]*:/, "", found); sub(/[[:space:]]#.*/, "", found)
				gsub(/^[[:space:]]+|[[:space:]]+$/, "", found)
				gsub(/^["\047]|["\047]$/, "", found)
				print found
			}' "$config")"
	fi
	if [ -n "$value" ]; then printf '%s' "$value"; else printf '%s' "$3"; fi
}

tf_plan=""
if [ "$kind" = terraform ]; then
	tf_plan="$(cfg hook terraform_plan "")"
	[ -n "$tf_plan" ] || skip "set hook.terraform_plan in $config to a saved 'tofu show -json' plan"
	case "$tf_plan" in
	/*) ;;
	*) tf_plan="$project_dir/$tf_plan" ;;
	esac
	[ -f "$tf_plan" ] || skip "the plan $tf_plan does not exist; save one with 'tofu show -json'"
	stale="$(printf '%s\n' "$iac_changes" | while IFS= read -r path; do
		[ "$root/$path" -nt "$tf_plan" ] && printf '%s\n' "$path"
	done)"
	[ -z "$stale" ] || skip "the plan $tf_plan is older than your .tf changes; save a new one with 'tofu show -json'"
fi

if [ "$trigger" = "claude-stop" ]; then
	fingerprint="$(cd "$root" && { printf '%s\n' "$iac_changes" | while IFS= read -r path; do
		printf '%s\n' "$path"
		if [ -f "$path" ]; then cat "$path"; else echo deleted; fi
	done 2>/dev/null; [ -n "$tf_plan" ] && cat "$tf_plan"; } | cksum)"
	last_file="$git_dir/finfocus-cost-hook.last"
	if [ -f "$last_file" ] && [ "$(cat "$last_file")" = "$fingerprint" ]; then
		exit 0
	fi
fi

# The lock holds its owner's pid, so a run that was killed (for example by the
# Claude Code hook timeout) never blocks the next one.
lock="$git_dir/finfocus-cost-hook.lock"
take_lock() {
	mkdir "$lock" 2>/dev/null && echo $$ >"$lock/pid"
}
if ! take_lock; then
	owner="$(cat "$lock/pid" 2>/dev/null)"
	if [ -n "$owner" ] && kill -0 "$owner" 2>/dev/null; then
		skip "another cost check is running"
	fi
	if [ -z "$owner" ] && [ -z "$(find "$lock" -maxdepth 0 -mmin +1 2>/dev/null)" ]; then
		skip "another cost check is running"
	fi
	rm -rf "$lock"
	take_lock || skip "another cost check is running"
fi
work=""
# shellcheck disable=SC2329 # Called from the traps below.
cleanup() {
	rm -rf "$work" "$lock"
}
trap cleanup EXIT
trap 'cleanup; trap - EXIT; exit 130' INT
trap 'cleanup; trap - EXIT; exit 143' TERM
work="$(mktemp -d 2>/dev/null)" || skip "cannot create a temporary directory"

[ "$have_jq" = true ] || skip "jq is not installed"
command -v finfocus >/dev/null 2>&1 || skip "finfocus is not installed or not on PATH"

# require_number skips when a threshold is not a non-negative number. It runs
# in the main shell, so skip ends the hook rather than a subshell.
require_number() {
	case "$2" in
	'' | *[!0-9.]* | *.*.* | .) skip "thresholds.$1 in $config is not a non-negative number" ;;
	esac
}

t_resource="$(cfg thresholds resource 100)"
require_number resource "$t_resource"
t_increase="$(cfg thresholds increase 50)"
require_number increase "$t_increase"
t_pct="$(cfg thresholds increase_pct 20)"
require_number increase_pct "$t_pct"
t_total="$(cfg thresholds total 1000)"
require_number total "$t_total"
t_currency="$(cfg "" currency USD)"
block="$(cfg hook block false)"

if [ "$kind" = pulumi ]; then
	command -v pulumi >/dev/null 2>&1 || skip "pulumi is not installed or not on PATH"
	if [ -n "${FINFOCUS_COST_HOOK_STACK:-}" ]; then
		pulumi preview --json --non-interactive --stack "$FINFOCUS_COST_HOOK_STACK" >"$work/plan.json" 2>"$work/preview.err" </dev/null
	else
		pulumi preview --json --non-interactive >"$work/plan.json" 2>"$work/preview.err" </dev/null
	fi || skip "pulumi preview failed; run it yourself to see why (login, stack, passphrase, or credentials)"
	finfocus cost projected --pulumi-json "$work/plan.json" ${FINFOCUS_COST_HOOK_STACK:+--stack "$FINFOCUS_COST_HOOK_STACK"} --output json --exit-on-threshold >"$work/cost.json" 2>"$work/cost.err" </dev/null
	status=$?
else
	finfocus cost projected --terraform-plan "$tf_plan" --output json --exit-on-threshold >"$work/cost.json" 2>"$work/cost.err" </dev/null
	status=$?
fi

budget_exceeded=false
if [ "$status" -ne 0 ]; then
	if grep -q budget_exceeded "$work/cost.err" 2>/dev/null; then
		budget_exceeded=true
	else
		skip "finfocus cost projected exited $status; run 'finfocus cost projected' to see why"
	fi
fi

summary="$(jq -r \
	--argjson tres "$t_resource" --argjson tinc "$t_increase" \
	--argjson tpct "$t_pct" --argjson ttot "$t_total" \
	--arg tcur "$t_currency" --argjson budget "$budget_exceeded" '
	def money: (. * 100 | round) as $c
		| ($c | if . < 0 then -. else . end) as $a
		| (if $c < 0 then "-" else "" end) + "\($a / 100 | floor)." + ("\($a % 100)" | if length < 2 then "0" + . else . end);
	def signed: (if . > 0 then "+" else "" end) + money;
	.finfocus as $f
	| ($f.resources // []) as $rs
	| ($f.diff.totalDelta // 0) as $delta
	| ($f.diff.totalBefore // 0) as $before
	| ($f.summary.totalMonthly // 0) as $total
	| ($f.summary.currency // $f.diff.currency // "USD") as $cur
	| [$rs[] | select(.error != null)] as $errored
	| ($errored | map(.resourceId)) as $erroredIds
	# A failed baseline (the old side of an update or replacement) is only in
	# errors[]; the diff counts it as 0, so its delta is unavailable.
	| ([$f.errors[]? | .resourceId | select(. as $id | $erroredIds | index($id) | not)] | unique) as $baseFailed
	| ($baseFailed | length == 0) as $deltaKnown
	| (($f.emptyPlan // false) or (($rs | length) > 0 and ($errored | length) == ($rs | length))) as $nothingPriced
	| ($cur == $tcur) as $sameCurrency
	| ([$rs[] | select(.error == null and (.monthly // 0) > $tres)] | length) as $expensive
	| (if $nothingPriced or ($sameCurrency | not) then [] else [
		(if $deltaKnown and $delta > $tinc then "increase \($delta | signed) > \($tinc)" else empty end),
		(if $deltaKnown and $before > 0 and ($delta / $before * 100) > $tpct then "increase \($delta / $before * 100 | round)% > \($tpct)%" else empty end),
		(if $total > $ttot then "plan total \($total | money) > \($ttot)" else empty end),
		(if $expensive > 0 then "\($expensive) resource(s) over \($tres)/mo" else empty end)
	] end) as $over
	| ((if $budget then ["budget exceeded"] else [] end) + $over) as $flags
	| ($flags | length > 0) as $flagged
	| (if ($f.stack.complete // false) then "stack total \($f.stack.totalMonthly | money) \($cur)/mo"
	   else "plan total \($total | money) \($cur)/mo" end) as $totalText
	| "flagged=\($flagged)",
	  (if $nothingPriced then "FinFocus cost check: nothing in this change has a price (\($rs | length) resource(s)), \($totalText)"
	   else "FinFocus cost check: \(if $deltaKnown then ($delta | signed) + " \($cur)/mo" else "change unavailable" end), \($totalText)"
		+ " (\($f.diff.creates // 0) create, \($f.diff.updates // 0) update, \($f.diff.deletes // 0) delete)" end),
	  ("Status: "
		+ (if $flagged then "over threshold: " + ($flags | join("; "))
		   elif $nothingPriced then "not priced (no installed plugin prices these resources; see finfocus plugin list)"
		   else "OK" end)
		+ (if ($sameCurrency | not) and ($nothingPriced | not) then " (thresholds not applied: costs are in \($cur), .cost-check.yml uses \($tcur))" else "" end)
		+ (if ($deltaKnown | not) then " (increase unavailable: the current cost of \($baseFailed | length) resource(s) failed to price)" else "" end)
		+ (if ($nothingPriced | not) and ($errored | length) > 0 then " (\($errored | length) unpriced, so totals are a lower bound)" else "" end)),
	  (if $nothingPriced then empty else
		[$rs[] | select(.error == null and (.deltaMonthly // 0) != 0 and (.resourceId as $id | $baseFailed | index($id) | not))]
		| sort_by(-(.deltaMonthly | if . < 0 then -. else . end)) | .[:3][]
		| "  \(.deltaMonthly | signed) \(.operation // "change") \(.resourceId | split("::") | last) (\(.resourceType))" end)
' "$work/cost.json" 2>/dev/null)" || skip "could not read the finfocus JSON output"

flagged=false
[ "$(printf '%s\n' "$summary" | head -n 1)" = "flagged=true" ] && flagged=true
message="$(printf '%s\n' "$summary" | sed 1d)"

if [ "$trigger" = "claude-stop" ]; then
	printf '%s' "$fingerprint" >"$last_file"
	if [ "$flagged" = true ] && [ "$block" = true ] && [ "$stop_hook_active" = false ]; then
		jq -cn --arg msg "$message" '{decision: "block", reason: ($msg + "\nReduce the cost, or tell the user why it is needed, before finishing.")}'
	else
		say "$message"
	fi
	exit 0
fi

printf '%s\n' "$message"
if [ "$flagged" = true ] && [ "$block" = true ]; then
	exit 1
fi
exit 0
