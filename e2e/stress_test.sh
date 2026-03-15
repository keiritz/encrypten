#!/usr/bin/env bash
# encrypten stress test — validates lock/unlock under extreme concurrency.
#
# Required environment variables:
#   STRESS_REPO_URL   — git clone URL of a repository with git-crypt encrypted files
#   STRESS_KEY_FILE   — path to the git-crypt / encrypten key file
#   STRESS_ENC_FILES  — colon-separated list of encrypted file paths in the repo
#
# Optional:
#   STRESS_NUM_WT     — number of worktrees to create (default: 20)
#   STRESS_GIT_CRYPT  — path to git-crypt binary (default: git-crypt; skip interop if absent)
#   STRESS_BASE_DIR   — working directory (default: /tmp/encrypten-stress)
#
# Example:
#   export STRESS_REPO_URL="https://github.com/example/repo.git"
#   export STRESS_KEY_FILE="/path/to/key"
#   export STRESS_ENC_FILES="secrets/config.json:src/app/env.yaml"
#   bash e2e/stress_test.sh

set -uo pipefail

# === Configuration ===
: "${STRESS_REPO_URL:?STRESS_REPO_URL is required}"
: "${STRESS_KEY_FILE:?STRESS_KEY_FILE is required}"
: "${STRESS_ENC_FILES:?STRESS_ENC_FILES is required}"
: "${STRESS_NUM_WT:=20}"
: "${STRESS_GIT_CRYPT:=git-crypt}"
: "${STRESS_BASE_DIR:=/tmp/encrypten-stress}"

BASE_DIR="$STRESS_BASE_DIR"
KEY="$STRESS_KEY_FILE"
NUM_WT="$STRESS_NUM_WT"

# Parse colon-separated encrypted files into array
IFS=':' read -ra ENC_FILES <<< "$STRESS_ENC_FILES"

PASS=0; FAIL=0; TOTAL=0

# === Colors ===
RED='\033[0;31m'; GREEN='\033[0;32m'; CYAN='\033[0;36m'; BOLD='\033[1m'; NC='\033[0m'

# === Helpers ===
ok() { TOTAL=$((TOTAL+1)); PASS=$((PASS+1)); echo -e "  ${GREEN}✓${NC} [$TOTAL] $1"; }
ng() { TOTAL=$((TOTAL+1)); FAIL=$((FAIL+1)); echo -e "  ${RED}✗${NC} [$TOTAL] $1"; [ -n "${2:-}" ] && echo -e "    ${RED}→ $2${NC}"; }
t() { if eval "$2"; then ok "$1"; else ng "$1" "${3:-}"; fi; }
section() { echo ""; echo -e "${CYAN}${BOLD}━━━ $1 ━━━${NC}"; }

is_enc() { [ -f "$1" ] && head -c 10 "$1" 2>/dev/null | xxd -p | grep -q "0047495443525950540"; }
all_enc() { for f in "${ENC_FILES[@]}"; do [ -f "$1/$f" ] && ! is_enc "$1/$f" && return 1; done; return 0; }
all_plain() { for f in "${ENC_FILES[@]}"; do [ -f "$1/$f" ] && is_enc "$1/$f" && return 1; done; return 0; }
hashes() { for f in "${ENC_FILES[@]}"; do [ -f "$1/$f" ] && shasum -a 256 "$1/$f"; done; }

has_git_crypt() { command -v "$STRESS_GIT_CRYPT" >/dev/null 2>&1; }

# === Setup ===
rm -rf "$BASE_DIR"; mkdir -p "$BASE_DIR"
section "SETUP"
git clone "$STRESS_REPO_URL" "$BASE_DIR/repo" 2>&1 | tail -1
cd "$BASE_DIR/repo" || exit 1
echo "  Cloned. Encrypted files: ${#ENC_FILES[@]}"

# ============================================================
section "A: Basic Operations"
# ============================================================
t "A1: encrypted after clone" "all_enc $BASE_DIR/repo"
encrypten unlock "$KEY" 2>&1 >/dev/null
t "A2: unlock" "all_plain $BASE_DIR/repo"
encrypten lock 2>&1 >/dev/null
t "A3: lock" "all_enc $BASE_DIR/repo"
encrypten unlock "$KEY" 2>&1 >/dev/null
out=$(encrypten unlock "$KEY" 2>&1) || true
t "A4: double unlock" 'echo "$out" | grep -qi "already unlocked"'
encrypten lock 2>&1 >/dev/null
out=$(encrypten lock 2>&1) || true
t "A5: double lock" 'echo "$out" | grep -qi "already locked"'

encrypten unlock "$KEY" 2>&1 >/dev/null
c_ok=true; for i in $(seq 1 20); do encrypten lock 2>&1 >/dev/null || { c_ok=false; break; }; encrypten unlock "$KEY" 2>&1 >/dev/null || { c_ok=false; break; }; done
t "A6: 20 rapid cycles" '[ "$c_ok" = true ] && all_plain "$BASE_DIR/repo"'

# ============================================================
section "B: git-crypt Interop"
# ============================================================
if has_git_crypt; then
    hashes "$BASE_DIR/repo" > /tmp/encrypten_stress_en_h.txt
    encrypten lock 2>&1 >/dev/null
    git config --worktree --remove-section filter.git-crypt 2>/dev/null || true
    git config --worktree --remove-section diff.git-crypt 2>/dev/null || true
    "$STRESS_GIT_CRYPT" unlock "$KEY" 2>&1 >/dev/null
    hashes "$BASE_DIR/repo" > /tmp/encrypten_stress_gc_h.txt
    t "B1: encrypten == git-crypt" "diff /tmp/encrypten_stress_en_h.txt /tmp/encrypten_stress_gc_h.txt >/dev/null 2>&1"
    "$STRESS_GIT_CRYPT" lock 2>&1 >/dev/null; encrypten unlock "$KEY" 2>&1 >/dev/null
    hashes "$BASE_DIR/repo" > /tmp/encrypten_stress_en2_h.txt
    t "B2: git-crypt lock → encrypten unlock" "diff /tmp/encrypten_stress_en_h.txt /tmp/encrypten_stress_en2_h.txt >/dev/null 2>&1"
else
    echo "  (skipped — git-crypt not found)"
fi

# ============================================================
section "C: Dirty Check"
# ============================================================
encrypten lock 2>&1 >/dev/null
echo "dirty" >> README.md
out=$(encrypten unlock "$KEY" 2>&1) || true
t "C1: dirty blocks unlock" 'echo "$out" | grep -qi "not clean"'
t "C1b: files still encrypted" "all_enc $BASE_DIR/repo"
t "C1c: dirty preserved" '[ "$(tail -1 README.md)" = "dirty" ]'
git checkout -- README.md 2>/dev/null

echo "dirty" >> README.md
encrypten unlock --force "$KEY" 2>&1 >/dev/null
t "C2: --force unlock" "all_plain $BASE_DIR/repo"
git checkout -- README.md 2>/dev/null

echo "dirty" >> README.md
out=$(encrypten lock 2>&1) || true
t "C3: dirty blocks lock" 'echo "$out" | grep -qi "not clean"'
git checkout -- README.md 2>/dev/null
encrypten lock 2>&1 >/dev/null

# ============================================================
section "D: autocrlf"
# ============================================================
for mode in true input false; do
    git config core.autocrlf "$mode"
    encrypten unlock "$KEY" 2>&1 >/dev/null
    t "D: unlock autocrlf=$mode" "all_plain $BASE_DIR/repo"
    encrypten lock 2>&1 >/dev/null
    t "D: lock autocrlf=$mode" "all_enc $BASE_DIR/repo"
done
git config core.autocrlf false

# ============================================================
section "E: Worktree Isolation (sequential)"
# ============================================================
encrypten lock 2>&1 >/dev/null || true
mkdir -p "$BASE_DIR/wt"
for i in $(seq 1 $NUM_WT); do
    git branch "s-$i" HEAD 2>/dev/null || true
    git worktree add "$BASE_DIR/wt/$i" "s-$i" 2>/dev/null
done
t "E0: $NUM_WT WTs created" '[ $(git worktree list | grep -c "/wt/") -eq $NUM_WT ]'

encrypten unlock "$KEY" 2>&1 >/dev/null
iso=true; for i in $(seq 1 $NUM_WT); do all_enc "$BASE_DIR/wt/$i" || { iso=false; break; }; done
t "E1: main unlock → WTs encrypted" '[ "$iso" = true ]'

(cd "$BASE_DIR/wt/1" && encrypten unlock "$KEY" 2>&1 >/dev/null)
t "E2: unlock wt-1 while main unlocked" "all_plain $BASE_DIR/wt/1"
t "E2b: main still unlocked" "all_plain $BASE_DIR/repo"

(cd "$BASE_DIR/wt/1" && encrypten lock 2>&1 >/dev/null)
t "E3: lock wt-1" "all_enc $BASE_DIR/wt/1"
t "E3b: main still unlocked" "all_plain $BASE_DIR/repo"

(cd "$BASE_DIR/wt/2" && encrypten unlock "$KEY" 2>&1 >/dev/null)
encrypten lock 2>&1 >/dev/null
t "E4: lock main" "all_enc $BASE_DIR/repo"
t "E4b: wt-2 still unlocked" "all_plain $BASE_DIR/wt/2"

for i in $(seq 1 2 $NUM_WT); do (cd "$BASE_DIR/wt/$i" && encrypten unlock "$KEY" 2>&1 >/dev/null); done
odd_ok=true; even_ok=true
for i in $(seq 1 $NUM_WT); do
    if [ $((i % 2)) -eq 1 ]; then all_plain "$BASE_DIR/wt/$i" || odd_ok=false
    else [ "$i" -eq 2 ] && continue; all_enc "$BASE_DIR/wt/$i" || even_ok=false; fi
done
t "E5: odd WTs unlocked" '[ "$odd_ok" = true ]'
t "E5b: even WTs encrypted" '[ "$even_ok" = true ]'

# ============================================================
section "F: PARALLEL Worktree Operations"
# ============================================================

# Reset
for i in $(seq 1 $NUM_WT); do (cd "$BASE_DIR/wt/$i" && encrypten lock --force 2>&1 >/dev/null) || true; done
encrypten lock --force 2>&1 >/dev/null || true

# F1: Concurrent unlock
encrypten unlock "$KEY" 2>&1 >/dev/null
pids=()
for i in $(seq 1 $NUM_WT); do
    (cd "$BASE_DIR/wt/$i" && encrypten unlock "$KEY" 2>&1 >/dev/null) &
    pids+=($!)
done
f1_ok=true; for pid in "${pids[@]}"; do wait "$pid" || f1_ok=false; done
for i in $(seq 1 $NUM_WT); do all_plain "$BASE_DIR/wt/$i" || f1_ok=false; done
t "F1: concurrent unlock $NUM_WT WTs" '[ "$f1_ok" = true ]'

# F2: Concurrent lock
pids=()
for i in $(seq 1 $NUM_WT); do
    (cd "$BASE_DIR/wt/$i" && encrypten lock 2>&1 >/dev/null) &
    pids+=($!)
done
f2_ok=true; for pid in "${pids[@]}"; do wait "$pid" || f2_ok=false; done
for i in $(seq 1 $NUM_WT); do all_enc "$BASE_DIR/wt/$i" || f2_ok=false; done
t "F2: concurrent lock $NUM_WT WTs" '[ "$f2_ok" = true ]'

# F3: Mixed concurrent
for i in $(seq 1 2 $NUM_WT); do (cd "$BASE_DIR/wt/$i" && encrypten unlock "$KEY" 2>&1 >/dev/null); done
pids=()
for i in $(seq 1 $NUM_WT); do
    if [ $((i % 2)) -eq 1 ]; then
        (cd "$BASE_DIR/wt/$i" && encrypten lock 2>&1 >/dev/null) &
    else
        (cd "$BASE_DIR/wt/$i" && encrypten unlock "$KEY" 2>&1 >/dev/null) &
    fi
    pids+=($!)
done
f3_ok=true; for pid in "${pids[@]}"; do wait "$pid" || f3_ok=false; done
for i in $(seq 1 $NUM_WT); do
    if [ $((i % 2)) -eq 1 ]; then all_enc "$BASE_DIR/wt/$i" || f3_ok=false
    else all_plain "$BASE_DIR/wt/$i" || f3_ok=false; fi
done
t "F3: mixed concurrent lock/unlock" '[ "$f3_ok" = true ]'

# F4: WT unlock + main lock simultaneously
for i in $(seq 1 $NUM_WT); do (cd "$BASE_DIR/wt/$i" && encrypten lock --force 2>&1 >/dev/null) || true; done
encrypten unlock "$KEY" 2>&1 >/dev/null
pids=()
for i in $(seq 1 10); do
    (cd "$BASE_DIR/wt/$i" && encrypten unlock "$KEY" 2>&1 >/dev/null) &
    pids+=($!)
done
(cd "$BASE_DIR/repo" && encrypten lock 2>&1 >/dev/null) &
pids+=($!)
f4_ok=true; for pid in "${pids[@]}"; do wait "$pid" || f4_ok=false; done
t "F4: 10 WT unlock + main lock simultaneously" '[ "$f4_ok" = true ]'
t "F4b: main locked" "all_enc $BASE_DIR/repo"
f4_wt=true; for i in $(seq 1 10); do all_plain "$BASE_DIR/wt/$i" || f4_wt=false; done
t "F4c: 10 WTs unlocked" '[ "$f4_wt" = true ]'

# F5: WT lock + main lock simultaneously
encrypten lock --force 2>&1 >/dev/null || true
encrypten unlock "$KEY" 2>&1 >/dev/null
for i in $(seq 1 10); do (cd "$BASE_DIR/wt/$i" && encrypten unlock "$KEY" 2>&1 >/dev/null); done
pids=()
for i in $(seq 1 10); do
    (cd "$BASE_DIR/wt/$i" && encrypten lock 2>&1 >/dev/null) &
    pids+=($!)
done
encrypten lock 2>&1 >/dev/null &
pids+=($!)
f5_ok=true; for pid in "${pids[@]}"; do wait "$pid" || f5_ok=false; done
t "F5: 10 WT lock + main lock simultaneously" '[ "$f5_ok" = true ]'

# ============================================================
section "G: EXTREME Parallel Stress"
# ============================================================

# Reset
encrypten unlock "$KEY" 2>&1 >/dev/null || true
for i in $(seq 1 $NUM_WT); do (cd "$BASE_DIR/wt/$i" && encrypten lock --force 2>&1 >/dev/null) || true; done

# G1: All WTs cycling + main
fail_f="$BASE_DIR/.stress_fail_1"; echo "0" > "$fail_f"
pids=()
for i in $(seq 1 $NUM_WT); do
    (
        cd "$BASE_DIR/wt/$i"
        for c in $(seq 1 5); do
            encrypten unlock "$KEY" 2>&1 >/dev/null || { echo "1" > "$fail_f"; exit 1; }
            encrypten lock 2>&1 >/dev/null || { echo "1" > "$fail_f"; exit 1; }
        done
    ) &
    pids+=($!)
done
(
    cd "$BASE_DIR/repo"
    for c in $(seq 1 5); do
        encrypten lock 2>&1 >/dev/null || { echo "1" > "$fail_f"; exit 1; }
        encrypten unlock "$KEY" 2>&1 >/dev/null || { echo "1" > "$fail_f"; exit 1; }
    done
) &
pids+=($!)
for pid in "${pids[@]}"; do wait "$pid" 2>/dev/null || true; done
g1_failed=$(cat "$fail_f"); rm -f "$fail_f"
t "G1: $NUM_WT WTs × 5 cycles + main" '[ "$g1_failed" = "0" ]'

# G2: 10 WTs rapid cycles
encrypten unlock "$KEY" 2>&1 >/dev/null || true
fail_f="$BASE_DIR/.stress_fail_2"; echo "0" > "$fail_f"
pids=()
for i in $(seq 1 10); do
    (
        cd "$BASE_DIR/wt/$i"
        for c in $(seq 1 10); do
            encrypten unlock "$KEY" 2>&1 >/dev/null || { echo "1" > "$fail_f"; exit 1; }
            encrypten lock 2>&1 >/dev/null || { echo "1" > "$fail_f"; exit 1; }
        done
    ) &
    pids+=($!)
done
for pid in "${pids[@]}"; do wait "$pid" 2>/dev/null || true; done
g2_failed=$(cat "$fail_f"); rm -f "$fail_f"
t "G2: 10 WTs × 10 rapid cycles" '[ "$g2_failed" = "0" ]'

# G3: Chaos — random ops
encrypten unlock "$KEY" 2>&1 >/dev/null || true
fail_f="$BASE_DIR/.stress_fail_3"; echo "0" > "$fail_f"
pids=()
for i in $(seq 1 $NUM_WT); do
    (
        cd "$BASE_DIR/wt/$i"
        for c in $(seq 1 8); do
            case $((RANDOM % 3)) in
                0) encrypten unlock "$KEY" 2>&1 >/dev/null || true ;;
                1) encrypten lock 2>&1 >/dev/null || true ;;
                2) encrypten status 2>&1 >/dev/null || { echo "1" > "$fail_f"; exit 1; } ;;
            esac
        done
    ) &
    pids+=($!)
done
(
    cd "$BASE_DIR/repo"
    for c in $(seq 1 8); do
        case $((RANDOM % 3)) in
            0) encrypten unlock "$KEY" 2>&1 >/dev/null || true ;;
            1) encrypten lock 2>&1 >/dev/null || true ;;
            2) encrypten status 2>&1 >/dev/null || { echo "1" > "$fail_f"; exit 1; } ;;
        esac
    done
) &
pids+=($!)
for pid in "${pids[@]}"; do wait "$pid" 2>/dev/null || true; done
g3_failed=$(cat "$fail_f"); rm -f "$fail_f"
t "G3: chaos — $NUM_WT WTs + main × 8 random ops" '[ "$g3_failed" = "0" ]'

# ============================================================
section "H: Edge Cases"
# ============================================================
cd "$BASE_DIR/repo"
encrypten lock --force 2>&1 >/dev/null || true

# H1: stash workflow
echo "stash" >> README.md; git stash 2>/dev/null
encrypten unlock "$KEY" 2>&1 >/dev/null
t "H1: unlock after stash" "all_plain $BASE_DIR/repo"
git stash pop 2>/dev/null
t "H1b: stash pop" '[ "$(tail -1 README.md)" = "stash" ]'
git checkout -- README.md 2>/dev/null

# H2: detached HEAD
encrypten lock 2>&1 >/dev/null
git checkout --detach HEAD 2>/dev/null
encrypten unlock "$KEY" 2>&1 >/dev/null
t "H2: unlock on detached HEAD" "all_plain $BASE_DIR/repo"
encrypten lock 2>&1 >/dev/null
t "H2b: lock on detached HEAD" "all_enc $BASE_DIR/repo"
git checkout main 2>/dev/null || git checkout master 2>/dev/null

# H3: shallow clone
cd "$BASE_DIR"
git clone --depth 1 "$STRESS_REPO_URL" "$BASE_DIR/shallow" 2>&1 | tail -1
cd "$BASE_DIR/shallow"
encrypten unlock "$KEY" 2>&1 >/dev/null
t "H3: shallow clone unlock" "all_plain $BASE_DIR/shallow"
encrypten lock 2>&1 >/dev/null
t "H3b: shallow clone lock" "all_enc $BASE_DIR/shallow"

# ============================================================
section "I: Final Integrity"
# ============================================================
if has_git_crypt; then
    for i in $(seq 1 $NUM_WT); do (cd "$BASE_DIR/wt/$i" && encrypten lock --force 2>&1 >/dev/null) || true; done
    cd "$BASE_DIR/repo"
    encrypten lock --force 2>&1 >/dev/null || true
    encrypten unlock "$KEY" 2>&1 >/dev/null
    hashes "$BASE_DIR/repo" > /tmp/encrypten_stress_final_en.txt
    encrypten lock 2>&1 >/dev/null
    git config --worktree --remove-section filter.git-crypt 2>/dev/null || true
    git config --worktree --remove-section diff.git-crypt 2>/dev/null || true
    "$STRESS_GIT_CRYPT" unlock "$KEY" 2>&1 >/dev/null
    hashes "$BASE_DIR/repo" > /tmp/encrypten_stress_final_gc.txt
    t "I1: post-stress encrypten == git-crypt" "diff /tmp/encrypten_stress_final_en.txt /tmp/encrypten_stress_final_gc.txt >/dev/null 2>&1"
else
    echo "  (skipped — git-crypt not found)"
fi

# ============================================================
section "CLEANUP"
cd "$BASE_DIR/repo"
"$STRESS_GIT_CRYPT" lock 2>&1 >/dev/null || true
git worktree list 2>/dev/null | grep "/wt/" | awk '{print $1}' | while read wt; do git worktree remove --force "$wt" 2>/dev/null || true; done
git worktree prune 2>/dev/null
for i in $(seq 1 $NUM_WT); do git branch -D "s-$i" 2>/dev/null || true; done
rm -f /tmp/encrypten_stress_*.txt
echo "  Done."

# ============================================================
echo ""
echo -e "${CYAN}${BOLD}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${CYAN}${BOLD}  RESULTS: $PASS/$TOTAL passed, $FAIL failed${NC}"
echo -e "${CYAN}${BOLD}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
[ "$FAIL" -eq 0 ] && echo -e "  ${GREEN}${BOLD}ALL TESTS PASSED${NC}" && exit 0
echo -e "  ${RED}${BOLD}SOME TESTS FAILED${NC}" && exit 1
