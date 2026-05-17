#!/usr/bin/env bash
set -euo pipefail

if [[ "${ASSET_OSS_INTEGRATION:-}" != "1" ]]; then
  echo "ASSET_OSS_INTEGRATION=1 is required to run the real OSS upload smoke test." >&2
  exit 2
fi

require_env() {
  local name="$1"
  if [[ -z "${!name:-}" ]]; then
    echo "$name is required." >&2
    exit 2
  fi
}

require_env ASSET_BASE_URL
require_env ASSET_ADMIN_EMAIL
require_env ASSET_ADMIN_PASSWORD
require_env ALIYUN_OSS_REGION
require_env ALIYUN_OSS_ENDPOINT
require_env ALIYUN_OSS_BUCKET
require_env ALIYUN_OSS_ACCESS_KEY_ID
require_env ALIYUN_OSS_ACCESS_KEY_SECRET

command -v curl >/dev/null || { echo "curl is required." >&2; exit 2; }
command -v jq >/dev/null || { echo "jq is required." >&2; exit 2; }
command -v base64 >/dev/null || { echo "base64 is required." >&2; exit 2; }
if base64 -d </dev/null >/dev/null 2>&1; then
  base64_decode=(base64 -d)
else
  base64_decode=(base64 -D)
fi

base_url="${ASSET_BASE_URL%/}"
tmp_dir="$(mktemp -d)"
trap 'rm -rf "$tmp_dir"' EXIT

api_post() {
  local path="$1"
  local body="$2"
  curl -fsS \
    -H "Content-Type: application/json" \
    -H "Authorization: Bearer ${access_token}" \
    --data "$body" \
    "${base_url}${path}"
}

api_put() {
  local path="$1"
  local body="$2"
  curl -fsS \
    -X PUT \
    -H "Content-Type: application/json" \
    -H "Authorization: Bearer ${access_token}" \
    --data "$body" \
    "${base_url}${path}"
}

api_get() {
  local path="$1"
  curl -fsS \
    -H "Authorization: Bearer ${access_token}" \
    "${base_url}${path}"
}

echo "Logging in to local edge-api as ${ASSET_ADMIN_EMAIL}..."
login_resp="$(curl -fsS -H "Content-Type: application/json" --data "$(jq -n --arg email "$ASSET_ADMIN_EMAIL" --arg password "$ASSET_ADMIN_PASSWORD" '{email:$email,password:$password}')" "${base_url}/api/v1/auth/login")"
access_token="$(jq -r '.data.access_token // empty' <<<"$login_resp")"
if [[ -z "$access_token" ]]; then
  echo "Login did not return an access token." >&2
  exit 1
fi

suffix="$(date +%s)"
type_resp="$(api_post "/api/v1/assets/types" "$(jq -n --arg code "as04_smoke_${suffix}" '{name:"AS04 Smoke Type",code:$code,part_schemas:[{key:"reference_images",name:"Reference Images",allowed_value_kinds:[2],multiple:true,required:false,sort_order:1}]}')")"
type_id="$(jq -r '.data.assetTypeID // .data.asset_type_id // .data.AssetTypeID // empty' <<<"$type_resp")"
if [[ -z "$type_id" ]]; then
  echo "Create asset type response did not include an asset type id." >&2
  exit 1
fi

category_resp="$(api_post "/api/v1/assets/categories" '{"name":"AS04 Smoke","sort_order":1}')"
category_id="$(jq -r '.data.categoryID // .data.category_id // .data.CategoryID // empty' <<<"$category_resp")"
if [[ -z "$category_id" ]]; then
  echo "Create asset category response did not include a category id." >&2
  exit 1
fi

asset_resp="$(api_post "/api/v1/assets/" "$(jq -n --arg type_id "$type_id" --arg category_id "$category_id" '{type_id:$type_id,name:"AS04 Smoke Asset",saved_to_library:true,category_id:$category_id}')")"
asset_id="$(jq -r '.data.assetID // .data.asset_id // .data.AssetID // empty' <<<"$asset_resp")"
if [[ -z "$asset_id" ]]; then
  echo "Create asset response did not include an asset id." >&2
  exit 1
fi

png_path="${tmp_dir}/as04-smoke.png"
"${base64_decode[@]}" >"$png_path" <<'PNG'
iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+/p9sAAAAASUVORK5CYII=
PNG
png_size="$(wc -c <"$png_path" | tr -d ' ')"

upload_resp="$(api_post "/api/v1/assets/media/upload-sessions" "$(jq -n --argjson size "$png_size" '{content_type:"image/png",size:$size,filename:"as04-smoke.png"}')")"
session_id="$(jq -r '.data.session.sessionID // .data.session.session_id // empty' <<<"$upload_resp")"
session_id="${session_id:-$(jq -r '.data.session.SessionID // empty' <<<"$upload_resp")}"
upload_url="$(jq -r '.data.upload.url // .data.upload.URL // empty' <<<"$upload_resp")"
upload_method="$(jq -r '.data.upload.method // .data.upload.Method // "PUT"' <<<"$upload_resp")"
upload_content_type="$(jq -r '.data.upload.headers["Content-Type"] // .data.upload.headers["content-type"] // "image/png"' <<<"$upload_resp")"
if [[ -z "$session_id" || -z "$upload_url" ]]; then
  echo "Create upload session response did not include session id and signed upload URL." >&2
  exit 1
fi

echo "Uploading generated PNG to OSS via signed ${upload_method} URL..."
oss_put_body="${tmp_dir}/oss-put-error.xml"
if ! oss_put_status="$(curl -sS -X "$upload_method" -H "Content-Type: ${upload_content_type}" --data-binary @"$png_path" "$upload_url" -o "$oss_put_body" -w "%{http_code}")"; then
  echo "OSS upload request failed before receiving an HTTP response." >&2
  exit 1
fi
if [[ "$oss_put_status" -lt 200 || "$oss_put_status" -ge 300 ]]; then
  echo "OSS upload failed with HTTP ${oss_put_status}." >&2
  oss_code="$(sed -n 's:.*<Code>\(.*\)</Code>.*:\1:p' "$oss_put_body" | head -n 1)"
  oss_message="$(sed -n 's:.*<Message>\(.*\)</Message>.*:\1:p' "$oss_put_body" | head -n 1)"
  [[ -n "$oss_code" ]] && echo "OSS Code: ${oss_code}" >&2
  [[ -n "$oss_message" ]] && echo "OSS Message: ${oss_message}" >&2
  exit 1
fi

finalize_resp="$(api_post "/api/v1/assets/media/upload-sessions/${session_id}/finalize" '{"width":1,"height":1}')"
media_id="$(jq -r '.data.media.mediaID // .data.media.media_id // empty' <<<"$finalize_resp")"
media_id="${media_id:-$(jq -r '.data.media.MediaID // empty' <<<"$finalize_resp")}"
if [[ -z "$media_id" ]]; then
  echo "Finalize response did not include media id." >&2
  exit 1
fi

api_put "/api/v1/assets/${asset_id}" "$(jq -n --arg name "AS04 Smoke Asset" --arg category_id "$category_id" --arg media_id "$media_id" '{name:$name,category_id:$category_id,cover_media_id:$media_id}')" >/dev/null
api_post "/api/v1/assets/${asset_id}/versions" "$(jq -n --arg media_id "$media_id" '{parts:{reference_images:{value_kind:2,media_ids:[$media_id]}},change_reason:"AS04 OSS smoke"}')" >/dev/null

access_resp="$(api_get "/api/v1/assets/media/${media_id}/access-url?expires_in_seconds=60")"
access_url="$(jq -r '.data.access.url // .data.access.URL // empty' <<<"$access_resp")"
if [[ -z "$access_url" ]]; then
  echo "Access URL response did not include signed URL." >&2
  exit 1
fi

download_path="${tmp_dir}/downloaded.png"
curl -fsS "$access_url" -o "$download_path"
if [[ "$(wc -c <"$download_path" | tr -d ' ')" -le 0 ]]; then
  echo "Downloaded object is empty." >&2
  exit 1
fi

echo "AS-04 OSS upload smoke test completed."
