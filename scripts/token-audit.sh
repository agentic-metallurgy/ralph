#!/usr/bin/env bash
# token-audit.sh — ground-truth token/cost accounting for a Claude Code session.
#
# Why this exists: the `assistant` events on the CLI's stdout stream-json carry a
# PARTIAL output_tokens (a snapshot taken when the event was flushed), and the CLI
# never re-emits a corrected one. input/cache counts on those events are exact;
# output_tokens is not. The accurate figures live in two other places:
#   * the `result` line's top-level `.usage` (per invocation), and
#   * the on-disk session transcript, which records final per-message usage.
# This script reads those, so its numbers are the ones to trust.
#
# Usage:
#   scripts/token-audit.sh                          # newest session for $PWD
#   scripts/token-audit.sh --project /path/to/repo  # newest session for that repo
#   scripts/token-audit.sh --session <session-id>   # a specific session (any project)
#   scripts/token-audit.sh --all                    # every session for the project
#   scripts/token-audit.sh --stream captured.jsonl  # audit a captured ralph stdout stream
#
# --stream mode is the diagnostic: it replays a captured stream the way the
# streamed events read on their own (dedup by message id) and diffs that against
# the authoritative `result` usage, showing how far the streamed output_tokens
# lags. Ralph reconciles this gap at each result line; the drift shown here is
# the CLI's emission behavior, not a ralph bug.
#
# Capture a stream to audit with:
#   claude --print --output-format stream-json --verbose ... > captured.jsonl
set -euo pipefail

PROJECTS_DIR="${CLAUDE_PROJECTS_DIR:-$HOME/.claude/projects}"

# List prices, USD per 1M tokens. Cache write is 1.25x input at 5m TTL and 2x at
# 1h TTL; cache read is 0.1x input. Sonnet 5 carries an introductory $2/$10 rate
# through 2026-08-31 — set SONNET5_INTRO=0 once that lapses.
SONNET5_INTRO="${SONNET5_INTRO:-1}"

die() { printf 'token-audit: %s\n' "$*" >&2; exit 2; }
command -v jq >/dev/null || die "jq is required"

slug_for() {
  # Claude Code slugifies the absolute project path: every non-alphanumeric
  # character becomes a dash.
  printf '%s' "$1" | sed 's/[^a-zA-Z0-9]/-/g'
}

# Shared awk program: reads TSV rows of
#   model  msg_id  input  output  cache_write_1h  cache_write_5m  cache_read  speed
# on stdin, dedups by msg_id, and prints a per-model breakdown plus a total.
read -r -d '' PRICE_AWK <<'AWK' || true
function rate(model, speed,   m) {
  m = tolower(model)
  if (m ~ /fable|mythos/)      { IN_=10.0; OUT_=50.0; return }
  if (m ~ /opus/)              { if (speed == "fast") { IN_=10.0; OUT_=50.0 } else { IN_=5.0; OUT_=25.0 }; return }
  if (m ~ /sonnet/)            { if (m ~ /sonnet-5/ && INTRO == "1") { IN_=2.0; OUT_=10.0 } else { IN_=3.0; OUT_=15.0 }; return }
  if (m ~ /haiku/)             { IN_=1.0; OUT_=5.0; return }
  IN_=3.0; OUT_=15.0  # unknown model: assume Sonnet rates, and say so
  UNKNOWN[model] = 1
}
BEGIN { FS = "\t" }
{
  model = $1; id = $2
  if (id != "" && seen[id]++) next          # dedup: same message emitted once per content block
  msgs[model]++
  in_[model]  += $3; out_[model] += $4
  cw1[model]  += $5; cw5[model]  += $6
  cr_[model]  += $7
  if ($8 == "fast") fast[model] = "fast"
  order[model] = 1
}
END {
  printf "%-22s %9s %9s %11s %11s %11s %12s\n", \
         "MODEL", "MSGS", "INPUT", "OUTPUT", "CACHE-W", "CACHE-R", "COST USD"
  printf "%-22s %9s %9s %11s %11s %11s %12s\n", \
         "----------------------", "---------", "---------", "-----------", "-----------", "-----------", "------------"
  for (m in order) {
    rate(m, fast[m])
    # cache write: 1.25x input at 5m TTL, 2x at 1h TTL. cache read: 0.1x input.
    c = (in_[m]*IN_ + out_[m]*OUT_ + cw1[m]*IN_*2.0 + cw5[m]*IN_*1.25 + cr_[m]*IN_*0.1) / 1000000
    printf "%-22s %9d %9d %11d %11d %11d %12.4f\n", \
           m, msgs[m], in_[m], out_[m], cw1[m]+cw5[m], cr_[m], c
    T_msg += msgs[m]; T_in += in_[m]; T_out += out_[m]
    T_cw += cw1[m]+cw5[m]; T_cw1 += cw1[m]; T_cr += cr_[m]; T_cost += c
  }
  printf "%-22s %9s %9s %11s %11s %11s %12s\n", \
         "----------------------", "---------", "---------", "-----------", "-----------", "-----------", "------------"
  printf "%-22s %9d %9d %11d %11d %11d %12.4f\n", "TOTAL", T_msg, T_in, T_out, T_cw, T_cr, T_cost
  printf "\ntotal tokens: %d   (of cache writes, %d were 1h TTL — billed at 2x input, not 1.25x)\n", \
         T_in + T_out + T_cw + T_cr, T_cw1
  for (u in UNKNOWN) printf "note: unrecognized model %s priced at Sonnet rates\n", u
}
AWK

# Emit TSV rows from one or more transcript files.
transcript_rows() {
  jq -r '
    select(.message.usage != null)
    | .message.usage as $u
    | [ (.message.model // "unknown"),
        (.message.id // ""),
        ($u.input_tokens // 0),
        ($u.output_tokens // 0),
        ($u.cache_creation.ephemeral_1h_input_tokens // 0),
        ($u.cache_creation.ephemeral_5m_input_tokens
           // (if $u.cache_creation == null then ($u.cache_creation_input_tokens // 0) else 0 end)),
        ($u.cache_read_input_tokens // 0),
        ($u.speed // "standard")
      ] | @tsv
  ' "$@"
}

audit_transcripts() {
  local label="$1"; shift
  [ "$#" -gt 0 ] || die "no transcript files found for $label"
  printf 'Session: %s\n' "$label"
  printf 'Source:  %d transcript file(s) — final per-message usage\n\n' "$#"
  transcript_rows "$@" | awk -v INTRO="$SONNET5_INTRO" "$PRICE_AWK"
}

# Replay a captured ralph stdout stream and diff ralph's accounting against truth.
audit_stream() {
  local f="$1"
  [ -f "$f" ] || die "no such file: $f"

  printf 'Stream:  %s\n\n' "$f"

  # Streamed view: assistant-event usage, deduped by message id.
  local ralph
  ralph=$(jq -r '
    select(.type == "assistant" and .message.usage != null)
    | [ .message.id, .message.usage.input_tokens, .message.usage.output_tokens,
        .message.usage.cache_creation_input_tokens, .message.usage.cache_read_input_tokens ] | @tsv
  ' "$f" | awk -F'\t' '!seen[$1]++ { i+=$2; o+=$3; w+=$4; r+=$5 } END { printf "%d\t%d\t%d\t%d", i, o, w, r }')

  # Ground truth: the result line's top-level usage, summed over invocations.
  local truth
  truth=$(jq -r '
    select(.type == "result" and .usage != null)
    | [ (.usage.input_tokens // 0), (.usage.output_tokens // 0),
        (.usage.cache_creation_input_tokens // 0), (.usage.cache_read_input_tokens // 0),
        (.total_cost_usd // 0) ] | @tsv
  ' "$f" | awk -F'\t' '{ i+=$1; o+=$2; w+=$3; r+=$4; c+=$5 } END { printf "%d\t%d\t%d\t%d\t%.6f", i, o, w, r, c }')

  [ -n "$truth" ] || die "stream has no result line with usage — was it captured with --verbose?"

  printf '%-14s %14s %14s %14s %10s\n' "COUNTER" "STREAMED" "SETTLED" "DRIFT" "RATIO"
  printf '%-14s %14s %14s %14s %10s\n' "--------------" "--------------" "--------------" "--------------" "----------"
  paste <(printf '%s\n' "$ralph" | tr '\t' '\n') <(printf '%s\n' "$truth" | cut -f1-4 | tr '\t' '\n') \
    | awk -F'\t' 'BEGIN { split("input output cache-write cache-read", n, " ") }
        { d = $2 - $1
          ratio = ($1 > 0) ? sprintf("%.1fx", $2 / $1) : "n/a"
          if (d == 0) ratio = "exact"
          printf "%-14s %14d %14d %+14d %10s\n", n[NR], $1, $2, d, ratio
          if (d != 0) bad = 1 }
        END { exit bad }' && rc=0 || rc=$?

  printf '\nCLI-reported cost for this stream: $%s\n' "$(printf '%s' "$truth" | cut -f5)"
  if [ "${rc:-0}" -ne 0 ]; then
    printf '\nStreamed events lag the settled figures — expected. The CLI reports\n'
    printf 'output_tokens on an assistant event as a snapshot of what had been generated\n'
    printf 'when the event was flushed, and never re-emits a corrected one. Ralph\n'
    printf 'reconciles against the result line, so its totals track the SETTLED column.\n'
    return 0
  fi
  printf '\nNo drift — streamed accumulation already matches the result usage.\n'
}

main() {
  local mode=newest project="$PWD" session="" stream=""
  while [ "$#" -gt 0 ]; do
    case "$1" in
      --project) project="${2:?--project needs a path}"; shift 2 ;;
      --session) mode=session; session="${2:?--session needs an id}"; shift 2 ;;
      --all)     mode=all; shift ;;
      --stream)  mode=stream; stream="${2:?--stream needs a file}"; shift 2 ;;
      -h|--help) sed -n '2,25p' "$0"; exit 0 ;;
      *)         [ -f "$1" ] && { mode=file; stream="$1"; shift; } || die "unknown argument: $1" ;;
    esac
  done

  case "$mode" in
    stream) audit_stream "$stream" ;;
    file)   audit_transcripts "$stream" "$stream" ;;
    session)
      local files
      IFS=$'\n' read -r -d '' -a files < <(find "$PROJECTS_DIR" -name "${session}.jsonl" -o -path "*/${session}/*.jsonl" | sort; printf '\0') || true
      audit_transcripts "$session" "${files[@]}"
      ;;
    *)
      local dir; dir="$PROJECTS_DIR/$(slug_for "$(cd "$project" && pwd)")"
      [ -d "$dir" ] || die "no transcripts for $project (looked in $dir)"
      if [ "$mode" = all ]; then
        local files; IFS=$'\n' read -r -d '' -a files < <(find "$dir" -name '*.jsonl' ! -name 'journal.jsonl' | sort; printf '\0') || true
        audit_transcripts "$project (all sessions)" "${files[@]}"
      else
        local newest; newest=$(find "$dir" -maxdepth 1 -name '*.jsonl' -exec ls -t {} + | head -1)
        [ -n "$newest" ] || die "no session transcripts in $dir"
        local sid; sid=$(basename "$newest" .jsonl)
        local files; IFS=$'\n' read -r -d '' -a files < <(find "$dir" -name "${sid}.jsonl" -o -path "*/${sid}/*.jsonl" ! -name 'journal.jsonl' | sort; printf '\0') || true
        audit_transcripts "$sid" "${files[@]}"
      fi
      ;;
  esac
}

main "$@"
