#!/usr/bin/env bash

set -euo pipefail

base_url="https://tokilake.abrdns.com/"
workers="1"

usage() {
  cat <<'EOF'
Usage:
  ./deploy/verify-i18n-playwright.sh [--url <url>] [--workers <n>]

Options:
  --url <url>       Target site URL, default https://tokilake.abrdns.com/
  --workers <n>     Playwright worker count, default 1
  --help            Show this help

Requirements:
  - node and npm must be installed
  - internet access is required to install @playwright/test and Chromium on first run
EOF
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    --url)
      base_url="${2:-}"
      shift 2
      ;;
    --workers)
      workers="${2:-}"
      shift 2
      ;;
    --help|-h)
      usage
      exit 0
      ;;
    *)
      echo "unknown argument: $1" >&2
      usage >&2
      exit 1
      ;;
  esac
done

for required_cmd in node npm mktemp; do
  if ! command -v "$required_cmd" >/dev/null 2>&1; then
    echo "required command not found: $required_cmd" >&2
    exit 1
  fi
done

tmpdir="$(mktemp -d)"
cleanup() {
  rm -rf "$tmpdir"
}
trap cleanup EXIT

cat >"$tmpdir/i18n.spec.js" <<'EOF'
const { test, expect } = require('@playwright/test');

const BASE_URL = process.env.BASE_URL || 'https://tokilake.abrdns.com/';
const cases = [
  { locale: 'en-US', expected: 'en_US' },
  { locale: 'ja-JP', expected: 'ja_JP' },
  { locale: 'zh-CN', expected: 'zh_CN' },
  { locale: 'zh-TW', expected: 'zh_HK' },
  { locale: 'fr-FR', expected: 'zh_CN' }
];

for (const { locale, expected } of cases) {
  test(`${locale} -> ${expected}`, async ({ browser }) => {
    const context = await browser.newContext({ locale });
    const page = await context.newPage();

    await page.goto(BASE_URL, { waitUntil: 'domcontentloaded' });
    await page.waitForLoadState('networkidle');

    await expect
      .poll(() => page.evaluate(() => localStorage.getItem('appLanguage')))
      .toBe(expected);

    const result = await page.evaluate(() => ({
      appLanguage: localStorage.getItem('appLanguage'),
      defaultLanguage: localStorage.getItem('default_language'),
      text: document.body.innerText.replace(/\s+/g, ' ').trim().slice(0, 120)
    }));

    console.log(JSON.stringify({ locale, expected, ...result }));
    await context.close();
  });
}
EOF

cd "$tmpdir"
npm init -y >/dev/null 2>&1
npm install -D @playwright/test >/dev/null
npx playwright install chromium >/dev/null

echo "Running i18n checks against: $base_url"
BASE_URL="$base_url" npx playwright test i18n.spec.js --reporter=line --workers="$workers"
