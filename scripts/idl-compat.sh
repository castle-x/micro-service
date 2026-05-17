#!/usr/bin/env bash
# scripts/idl-compat.sh
# Thrift IDL 向后兼容性检查
#
# 检测以下破坏性变更：
#   1. 字段编号被复用（同一 struct 中 fid 历史上指向 A 字段，PR 中改成指向 B 字段）
#   2. required 字段被删除（老 client 必带，新 server 不再认）
#   3. optional 改成 required（新 server 强制要求老 client 不带的字段）
#   4. enum 值被删除或改名（同 fid 不同含义）
#   5. struct / service / function 被删除
#
# 用法：
#   bash scripts/idl-compat.sh                    # 对比 origin/develop 与当前工作树
#   BASE_REF=v1.2.0 bash scripts/idl-compat.sh    # 对比指定 ref
#
# 退出码：
#   0 = 兼容
#   1 = 检测到破坏性变更
#   2 = 脚本执行错误（缺工具、git 异常等）

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"

BASE_REF="${BASE_REF:-origin/develop}"
IDL_DIR="idl"
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

red()    { printf '\033[31m%s\033[0m\n' "$*"; }
green()  { printf '\033[32m%s\033[0m\n' "$*"; }
yellow() { printf '\033[33m%s\033[0m\n' "$*"; }

if ! git rev-parse --git-dir >/dev/null 2>&1; then
  red "Error: not in a git repository"
  exit 2
fi

if ! git rev-parse --verify "$BASE_REF" >/dev/null 2>&1; then
  yellow "Warning: BASE_REF=$BASE_REF not found, falling back to HEAD~1"
  BASE_REF="HEAD~1"
fi

echo ">>> IDL compat check: comparing $BASE_REF -> working tree"

CHANGED=$(git diff --name-only "$BASE_REF" -- "$IDL_DIR" | grep -E '\.thrift$' || true)

if [ -z "$CHANGED" ]; then
  green "✓ No thrift files changed since $BASE_REF"
  exit 0
fi

echo ">>> Changed thrift files:"
echo "$CHANGED" | sed 's/^/    /'
echo

ERRORS=0
report_error() {
  red "✗ $1"
  ERRORS=$((ERRORS + 1))
}

# 提取 struct 内的字段编号映射（fid -> name + required/optional）
# 输入：thrift 内容
# 输出：每行 "<struct>.<fid> <required|optional|default> <name>"
extract_fields() {
  awk '
    /^[[:space:]]*struct[[:space:]]+/ {
      match($0, /struct[[:space:]]+([A-Za-z0-9_]+)/, m); cur=m[1]; in_struct=1; next
    }
    /^[[:space:]]*service[[:space:]]+/ { in_struct=0 }
    /^[[:space:]]*}/ { in_struct=0; cur="" }
    in_struct && /^[[:space:]]*[0-9]+:/ {
      line=$0
      # 提取 fid
      match(line, /^[[:space:]]*([0-9]+):/, fm); fid=fm[1]
      # 提取 required / optional（默认 default）
      mod="default"
      if (line ~ /[[:space:]]required[[:space:]]/) mod="required"
      else if (line ~ /[[:space:]]optional[[:space:]]/) mod="optional"
      # 提取字段名（最后一个 identifier，紧邻 = 或 ; 或 ,）
      name="?"
      if (match(line, /([A-Za-z_][A-Za-z0-9_]*)[[:space:]]*[;,=]/, nm)) name=nm[1]
      else if (match(line, /([A-Za-z_][A-Za-z0-9_]*)[[:space:]]*$/, nm)) name=nm[1]
      printf "%s.%s %s %s\n", cur, fid, mod, name
    }
  ' "$1"
}

# 提取 enum 值映射
extract_enums() {
  awk '
    /^[[:space:]]*enum[[:space:]]+/ {
      match($0, /enum[[:space:]]+([A-Za-z0-9_]+)/, m); cur=m[1]; in_enum=1; next
    }
    /^[[:space:]]*}/ { in_enum=0; cur="" }
    in_enum && /=/ {
      gsub(/[,;]/, "")
      n=split($0, a, "=")
      key=a[1]; val=a[2]
      gsub(/^[[:space:]]+|[[:space:]]+$/, "", key)
      gsub(/^[[:space:]]+|[[:space:]]+$/, "", val)
      if (key != "" && val ~ /^[0-9]+$/) printf "%s.%s %s\n", cur, val, key
    }
  ' "$1"
}

for f in $CHANGED; do
  echo ">>> Checking $f"
  OLD="$TMP_DIR/old_$(echo "$f" | tr '/' '_')"
  NEW="$f"

  # 取 base 版本；新增文件视为兼容
  if ! git show "$BASE_REF:$f" > "$OLD" 2>/dev/null; then
    yellow "    (new file, skip)"
    continue
  fi

  if [ ! -f "$NEW" ]; then
    report_error "[deleted] $f was removed (breaks all consumers)"
    continue
  fi

  # 1. 字段编号 / required / 字段名变更检测
  OLD_FIELDS="$TMP_DIR/old_fields.txt"
  NEW_FIELDS="$TMP_DIR/new_fields.txt"
  extract_fields "$OLD" | sort > "$OLD_FIELDS"
  extract_fields "$NEW" | sort > "$NEW_FIELDS"

  while IFS=' ' read -r key old_mod old_name; do
    new_line=$(grep -E "^${key} " "$NEW_FIELDS" || true)
    if [ -z "$new_line" ]; then
      if [ "$old_mod" = "required" ]; then
        report_error "[required removed] $f: field $key ($old_name) was required and is now removed"
      else
        yellow "    (note) $f: field $key ($old_name) removed (optional/default — soft break, ensure all consumers updated)"
      fi
      continue
    fi
    new_mod=$(echo "$new_line" | awk '{print $2}')
    new_name=$(echo "$new_line" | awk '{print $3}')

    # 字段编号被改名（同 fid 但 name 变了）
    if [ "$old_name" != "$new_name" ] && [ "$old_name" != "?" ] && [ "$new_name" != "?" ]; then
      report_error "[fid reused] $f: $key was '$old_name', now '$new_name' (field number reuse breaks wire format)"
    fi
    # optional/default → required 是破坏性
    if [ "$old_mod" != "required" ] && [ "$new_mod" = "required" ]; then
      report_error "[modifier tightened] $f: field $key ($new_name) became required (rejects old clients)"
    fi
  done < "$OLD_FIELDS"

  # 2. enum 值变更
  OLD_ENUMS="$TMP_DIR/old_enums.txt"
  NEW_ENUMS="$TMP_DIR/new_enums.txt"
  extract_enums "$OLD" | sort > "$OLD_ENUMS"
  extract_enums "$NEW" | sort > "$NEW_ENUMS"
  while IFS=' ' read -r ekey old_ename; do
    new_line=$(grep -E "^${ekey} " "$NEW_ENUMS" || true)
    if [ -z "$new_line" ]; then
      report_error "[enum removed] $f: enum value $ekey ($old_ename) was removed"
      continue
    fi
    new_ename=$(echo "$new_line" | awk '{print $2}')
    if [ "$old_ename" != "$new_ename" ]; then
      report_error "[enum renamed] $f: enum value $ekey: '$old_ename' -> '$new_ename' (value reuse breaks wire format)"
    fi
  done < "$OLD_ENUMS"

  # 3. service / function 删除（粗粒度：删除的 function 名）
  OLD_FNS=$(grep -oE '^[[:space:]]+[A-Za-z0-9_]+[[:space:]]*\(' "$OLD" | tr -d ' (' | sort -u)
  NEW_FNS=$(grep -oE '^[[:space:]]+[A-Za-z0-9_]+[[:space:]]*\(' "$NEW" | tr -d ' (' | sort -u)
  for fn in $OLD_FNS; do
    if ! echo "$NEW_FNS" | grep -qx "$fn"; then
      report_error "[fn removed] $f: function/method '$fn' was removed (breaks consumers)"
    fi
  done
done

echo
if [ "$ERRORS" -gt 0 ]; then
  red "✗ $ERRORS breaking change(s) detected"
  echo
  yellow "If this is intentional (major version bump), explicitly bypass with:"
  yellow "  IDL_COMPAT_ALLOW_BREAKING=1 make idl-compat"
  if [ "${IDL_COMPAT_ALLOW_BREAKING:-0}" = "1" ]; then
    yellow ">>> IDL_COMPAT_ALLOW_BREAKING=1 set, exiting 0"
    exit 0
  fi
  exit 1
fi

green "✓ IDL backward compatible"
