#!/usr/bin/env bash
# Behavioral test suite for scripts/install.sh, using mock `curl` and
# `uname` executables instead of a real network or a real multi-arch
# machine. Each test case runs the real install.sh (unmodified) with a
# controlled PATH and environment, and checks its exit code and/or
# resulting filesystem state.
#
# Usage: bash scripts/tests/test-install.sh
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
INSTALL_SH="$ROOT/scripts/install.sh"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

# Resolve the sh interpreter by absolute path up front, from this
# harness's own unrestricted PATH - the per-test PATH used to run
# install.sh is deliberately narrowed (to simulate missing tools) and
# must not be relied on to find `sh` itself.
SH_BIN="$(command -v sh)"

pass_count=0
fail_count=0

# check_result CODE_EXPECTED_ZERO(0|1) ACTUAL_EXIT NAME
report() {
  local ok="$1" name="$2" detail="${3:-}"
  if [ "$ok" = "1" ]; then
    echo "PASS: $name"
    pass_count=$((pass_count + 1))
  else
    echo "FAIL: $name${detail:+ - $detail}"
    fail_count=$((fail_count + 1))
  fi
}

# Stand-in "binary": a tiny POSIX sh script, since install.sh's own last
# step actually executes the installed file with --version. Real sha256
# of this fixed fixture payload is computed once with the real tool
# (available in this harness's own unrestricted PATH).
FIXTURE_CONTENT='#!/bin/sh
echo "logquiet v9.9.9 (test fixture)"
'
FIXTURE_SHA="$(printf '%s' "$FIXTURE_CONTENT" | sha256sum | awk '{print $1}')"

make_mock_bin() {
  local dir="$1"
  mkdir -p "$dir"

  cat > "$dir/uname" <<'EOF'
#!/bin/sh
case "$1" in
  -s) printf '%s\n' "$MOCK_UNAME_S" ;;
  -m) printf '%s\n' "$MOCK_UNAME_M" ;;
esac
EOF
  chmod +x "$dir/uname"

  cat > "$dir/curl" <<'EOF'
#!/bin/sh
# Mock curl: understands `-fsSL -o OUTFILE URL` (and ignores extra flags).
outfile=""
url=""
prev=""
for a in "$@"; do
  if [ "$prev" = "-o" ]; then
    outfile="$a"
  fi
  prev="$a"
  url="$a"
done
case "$url" in
  *SHA256SUMS.txt)
    if [ "${MOCK_FAIL_SUMS:-0}" = "1" ]; then
      exit 22
    fi
    printf '%s\n' "$MOCK_SHA_LINE" > "$outfile"
    ;;
  *)
    if [ "${MOCK_FAIL_ASSET:-0}" = "1" ]; then
      exit 22
    fi
    printf '%s' "$MOCK_ASSET_CONTENT" > "$outfile"
    ;;
esac
exit 0
EOF
  chmod +x "$dir/curl"
}

# run_install: invokes install.sh with a given PATH and env, capturing
# exit code and output. Sets globals: RUN_EXIT, RUN_OUT.
run_install() {
  local test_path="$1"; shift
  set +e
  RUN_OUT="$(env -i \
    PATH="$test_path" \
    HOME="$WORK/home" \
    "$@" \
    "$SH_BIN" "$INSTALL_SH" 2>&1)"
  RUN_EXIT=$?
  set -e
}

mkdir -p "$WORK/home"

# ---- 1-4: OS/arch matrix, expected success ----
matrix_case() {
  local name="$1" uname_s="$2" uname_m="$3" goos="$4" goarch="$5"
  local case_dir="$WORK/$name"
  local mockbin="$case_dir/mockbin"
  local install_dir="$case_dir/installdir"
  make_mock_bin "$mockbin"

  local version="v9.9.9"
  local asset="logquiet-${version}-${goos}-${goarch}"
  local sha_line="${FIXTURE_SHA}  ${asset}"

  run_install "$mockbin:$PATH" \
    MOCK_UNAME_S="$uname_s" MOCK_UNAME_M="$uname_m" \
    MOCK_ASSET_CONTENT="$FIXTURE_CONTENT" MOCK_SHA_LINE="$sha_line" \
    LOGQUIET_VERSION="$version" LOGQUIET_INSTALL_DIR="$install_dir" \
    LOGQUIET_BASE_URL="https://mock.example/releases/download"

  if [ "$RUN_EXIT" = "0" ] && [ -f "$install_dir/logquiet" ] && [ -x "$install_dir/logquiet" ]; then
    report 1 "$name: installs successfully"
  else
    report 0 "$name: installs successfully" "exit=$RUN_EXIT out=$RUN_OUT"
  fi
}

matrix_case "linux_amd64"  "Linux"  "x86_64"  "linux"  "amd64"
matrix_case "linux_arm64"  "Linux"  "aarch64" "linux"  "arm64"
matrix_case "macos_amd64"  "Darwin" "x86_64"  "darwin" "amd64"
matrix_case "macos_arm64"  "Darwin" "arm64"   "darwin" "arm64"

# ---- 5: unsupported architecture ----
{
  case_dir="$WORK/unsupported_arch"
  mockbin="$case_dir/mockbin"
  make_mock_bin "$mockbin"
  run_install "$mockbin:$PATH" \
    MOCK_UNAME_S="Linux" MOCK_UNAME_M="riscv64" \
    LOGQUIET_VERSION="v9.9.9" LOGQUIET_INSTALL_DIR="$case_dir/installdir"
  if [ "$RUN_EXIT" != "0" ] && printf '%s' "$RUN_OUT" | grep -qi "unsupported architecture"; then
    report 1 "unsupported architecture: rejected with clear message"
  else
    report 0 "unsupported architecture: rejected with clear message" "exit=$RUN_EXIT out=$RUN_OUT"
  fi
}

# ---- 6: unsupported OS (bonus coverage) ----
{
  case_dir="$WORK/unsupported_os"
  mockbin="$case_dir/mockbin"
  make_mock_bin "$mockbin"
  run_install "$mockbin:$PATH" \
    MOCK_UNAME_S="SunOS" MOCK_UNAME_M="x86_64" \
    LOGQUIET_VERSION="v9.9.9" LOGQUIET_INSTALL_DIR="$case_dir/installdir"
  if [ "$RUN_EXIT" != "0" ] && printf '%s' "$RUN_OUT" | grep -qi "unsupported OS"; then
    report 1 "unsupported OS: rejected with clear message"
  else
    report 0 "unsupported OS: rejected with clear message" "exit=$RUN_EXIT out=$RUN_OUT"
  fi
}

# ---- 7: failed download (network failure) ----
{
  case_dir="$WORK/failed_download"
  mockbin="$case_dir/mockbin"
  install_dir="$case_dir/installdir"
  make_mock_bin "$mockbin"
  run_install "$mockbin:$PATH" \
    MOCK_UNAME_S="Linux" MOCK_UNAME_M="x86_64" \
    MOCK_FAIL_ASSET="1" \
    LOGQUIET_VERSION="v9.9.9" LOGQUIET_INSTALL_DIR="$install_dir"
  if [ "$RUN_EXIT" != "0" ] && [ ! -e "$install_dir/logquiet" ]; then
    report 1 "failed download: fails closed, nothing installed"
  else
    report 0 "failed download: fails closed, nothing installed" "exit=$RUN_EXIT out=$RUN_OUT"
  fi
}

# ---- 8: 404 on the release asset (curl -f collapses this into the same
#          exit behavior as a network failure - documented, not a bug) ----
{
  case_dir="$WORK/http_404"
  mockbin="$case_dir/mockbin"
  install_dir="$case_dir/installdir"
  make_mock_bin "$mockbin"
  run_install "$mockbin:$PATH" \
    MOCK_UNAME_S="Linux" MOCK_UNAME_M="x86_64" \
    MOCK_FAIL_ASSET="1" \
    LOGQUIET_VERSION="v0.0.0-does-not-exist" LOGQUIET_INSTALL_DIR="$install_dir"
  if [ "$RUN_EXIT" != "0" ] && [ ! -e "$install_dir/logquiet" ] && printf '%s' "$RUN_OUT" | grep -qi "download failed"; then
    report 1 "404 on release asset: fails closed with clear message"
  else
    report 0 "404 on release asset: fails closed with clear message" "exit=$RUN_EXIT out=$RUN_OUT"
  fi
}

# ---- 9: checksum mismatch ----
{
  case_dir="$WORK/checksum_mismatch"
  mockbin="$case_dir/mockbin"
  install_dir="$case_dir/installdir"
  make_mock_bin "$mockbin"
  version="v9.9.9"
  asset="logquiet-${version}-linux-amd64"
  bad_sha="0000000000000000000000000000000000000000000000000000000000000000"
  run_install "$mockbin:$PATH" \
    MOCK_UNAME_S="Linux" MOCK_UNAME_M="x86_64" \
    MOCK_ASSET_CONTENT="$FIXTURE_CONTENT" MOCK_SHA_LINE="${bad_sha}  ${asset}" \
    LOGQUIET_VERSION="$version" LOGQUIET_INSTALL_DIR="$install_dir"
  if [ "$RUN_EXIT" != "0" ] && [ ! -e "$install_dir/logquiet" ] && printf '%s' "$RUN_OUT" | grep -qi "checksum verification failed"; then
    report 1 "checksum mismatch: fails closed, nothing installed"
  else
    report 0 "checksum mismatch: fails closed, nothing installed" "exit=$RUN_EXIT out=$RUN_OUT"
  fi
}

# ---- 10: destination not writable (a file occupies the target path,
#           portable across platforms without relying on chmod/ACL
#           semantics that behave inconsistently under MSYS2/Windows) ----
{
  case_dir="$WORK/dest_not_writable"
  mockbin="$case_dir/mockbin"
  blocked="$case_dir/blocked-install-dir"
  make_mock_bin "$mockbin"
  mkdir -p "$case_dir"
  : > "$blocked"   # a plain file sits where install.sh wants to mkdir -p a directory
  version="v9.9.9"
  asset="logquiet-${version}-linux-amd64"
  run_install "$mockbin:$PATH" \
    MOCK_UNAME_S="Linux" MOCK_UNAME_M="x86_64" \
    MOCK_ASSET_CONTENT="$FIXTURE_CONTENT" MOCK_SHA_LINE="${FIXTURE_SHA}  ${asset}" \
    LOGQUIET_VERSION="$version" LOGQUIET_INSTALL_DIR="$blocked"
  if [ "$RUN_EXIT" != "0" ] && printf '%s' "$RUN_OUT" | grep -qi "could not create\|not writable"; then
    report 1 "destination not writable: fails with actionable message"
  else
    report 0 "destination not writable: fails with actionable message" "exit=$RUN_EXIT out=$RUN_OUT"
  fi
}

# ---- 11: missing checksum tool (both sha256sum and shasum absent) ----
# install.sh checks for a checksum tool as its second check, before any
# other external command is invoked, so a PATH containing only the mock
# curl/uname (and no coreutils at all) is sufficient to exercise this
# exact failure without needing an isolated coreutils copy.
{
  case_dir="$WORK/missing_checksum_tool"
  mockbin="$case_dir/mockbin"
  make_mock_bin "$mockbin"
  run_install "$mockbin" \
    LOGQUIET_VERSION="v9.9.9" LOGQUIET_INSTALL_DIR="$case_dir/installdir"
  if [ "$RUN_EXIT" != "0" ] && printf '%s' "$RUN_OUT" | grep -qi "no checksum tool found"; then
    report 1 "missing checksum tool: rejected with clear message"
  else
    report 0 "missing checksum tool: rejected with clear message" "exit=$RUN_EXIT out=$RUN_OUT"
  fi
}

# ---- 12: missing curl ----
# require_cmd curl is the very first check, so a PATH with only the mock
# uname (no curl, mock or real) is sufficient.
{
  case_dir="$WORK/missing_curl"
  mockbin="$case_dir/mockbin"
  make_mock_bin "$mockbin"
  rm -f "$mockbin/curl"
  run_install "$mockbin" \
    LOGQUIET_VERSION="v9.9.9" LOGQUIET_INSTALL_DIR="$case_dir/installdir"
  if [ "$RUN_EXIT" != "0" ] && printf '%s' "$RUN_OUT" | grep -qi "'curl' is required"; then
    report 1 "missing curl: rejected with clear message"
  else
    report 0 "missing curl: rejected with clear message" "exit=$RUN_EXIT out=$RUN_OUT"
  fi
}

echo
echo "== $pass_count passed, $fail_count failed =="
[ "$fail_count" = "0" ]
