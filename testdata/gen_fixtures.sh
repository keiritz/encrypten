#!/usr/bin/env bash
#
# gen_fixtures.sh — generate git-crypt test fixtures for encrypten
#
# Prerequisites: git-crypt 0.7.0, git, python3
# Usage: bash testdata/gen_fixtures.sh
#
set -euo pipefail

# ── Verify git-crypt version ────────────────────────────────────────────────
GITCRYPT_VERSION=$(git-crypt version 2>/dev/null || true)
if [[ "$GITCRYPT_VERSION" != *"0.7.0"* ]]; then
    echo "ERROR: git-crypt 0.7.0 required (got: ${GITCRYPT_VERSION:-not found})" >&2
    exit 1
fi

# ── Paths ────────────────────────────────────────────────────────────────────
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
FIXTURES_DIR="$SCRIPT_DIR/fixtures"

# Clean and recreate fixtures directory
rm -rf "$FIXTURES_DIR"
mkdir -p "$FIXTURES_DIR"

# ── Temporary repository ────────────────────────────────────────────────────
TMPDIR="$(mktemp -d)"
trap 'rm -rf "$TMPDIR"' EXIT

cd "$TMPDIR"
git init -q
git-crypt init

# ── Configure encryption patterns ───────────────────────────────────────────
cat > .gitattributes <<'ATTR'
*.txt filter=git-crypt diff=git-crypt
*.bin filter=git-crypt diff=git-crypt
ATTR
git add .gitattributes
git commit -q -m "add gitattributes"

# ── Create plaintext files ──────────────────────────────────────────────────
printf 'Hello, git-crypt!\n' > plain.txt

touch plain_empty.txt

python3 -c "
import sys
pattern = bytes(range(256))
sys.stdout.buffer.write(pattern * 4096)
" > plain_large.bin

# ── Commit encrypted files ──────────────────────────────────────────────────
git add plain.txt plain_empty.txt plain_large.bin
git commit -q -m "add test files"

# ── Export key ───────────────────────────────────────────────────────────────
git-crypt export-key "$FIXTURES_DIR/key_default"

KEY_SIZE=$(wc -c < "$FIXTURES_DIR/key_default" | tr -d ' ')
if [[ "$KEY_SIZE" -ne 148 ]]; then
    echo "ERROR: key_default expected 148 bytes, got $KEY_SIZE" >&2
    exit 1
fi

# ── Copy plaintext files ────────────────────────────────────────────────────
cp plain.txt      "$FIXTURES_DIR/plain.txt"
cp plain_empty.txt "$FIXTURES_DIR/plain_empty.txt"
cp plain_large.bin "$FIXTURES_DIR/plain_large.bin"

# ── Lock and copy encrypted files ───────────────────────────────────────────
git-crypt lock

cp plain.txt      "$FIXTURES_DIR/encrypted.bin"
cp plain_empty.txt "$FIXTURES_DIR/encrypted_empty.bin"
cp plain_large.bin "$FIXTURES_DIR/encrypted_large.bin"

# ── Verify encrypted headers ────────────────────────────────────────────────
HEADER=$(xxd -l 10 -p "$FIXTURES_DIR/encrypted.bin")
EXPECTED="00474954435259505400"
if [[ "$HEADER" != "$EXPECTED" ]]; then
    echo "ERROR: encrypted.bin missing GITCRYPT header" >&2
    echo "  expected: $EXPECTED" >&2
    echo "  got:      $HEADER" >&2
    exit 1
fi

# ── Summary ──────────────────────────────────────────────────────────────────
echo ""
echo "=== Fixtures generated ==="
for f in "$FIXTURES_DIR"/*; do
    SIZE=$(wc -c < "$f" | tr -d ' ')
    printf "  %-25s %s bytes\n" "$(basename "$f")" "$SIZE"
done
echo ""
echo "Done."
