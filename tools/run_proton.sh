#!/usr/bin/env bash
set -euo pipefail

if [[ $# -eq 0 || "$1" == "--help" ]]; then
  printf 'Usage: %s GAME_DIR [GAME_ARGUMENTS...]\n' "${0##*/}"
  exit "$(( $# == 0 ? 2 : 0 ))"
fi
game_dir=$(cd -- "$1" && pwd -P)
shift

config="${AZUR_PROTON_CONFIG:-${XDG_CONFIG_HOME:-$HOME/.config}/azurpromilia/proton.conf}"
if [[ -f "$config" ]]; then
  source "$config"
elif [[ -n "${AZUR_PROTON_CONFIG:-}" ]]; then
  printf 'Proton configuration not found: %s\n' "$config" >&2
  exit 1
fi

steam_dir=$(cd -- "${AZUR_STEAM_ROOT:-$HOME/.steam/root}" && pwd -P)
proton="${AZUR_PROTON_BIN:-$steam_dir/compatibilitytools.d/dwproton-11.0-16-x86_64/proton}"
sdk_dir="$game_dir/AzurPromilia_Data/Plugins/mj_sdk"
if [[ ! -f "$game_dir/AzurPromilia.exe" || ! -d "$sdk_dir" ]]; then
  printf 'Expected AzurPromilia.exe and the game SDK in: %s\n' "$game_dir" >&2
  exit 1
fi
if [[ ! -x "$proton" ]]; then
  printf 'Proton executable not found: %s\n' "$proton" >&2
  exit 1
fi

export STEAM_COMPAT_DATA_PATH="${AZUR_COMPATDATA:-$HOME/Games/compatdata/azurpromilia-sc000-dwproton}"
export STEAM_COMPAT_CLIENT_INSTALL_PATH="$steam_dir"
export STEAM_COMPAT_INSTALL_PATH="$game_dir"
export SteamGameId=0
export PROTON_ENABLE_WAYLAND="${PROTON_ENABLE_WAYLAND:-1}"
export WINEPATH="Z:${sdk_dir//\//\\}"
export PROTON_LOG="${PROTON_LOG:-0}"
export DXVK_LOG_LEVEL=none

mkdir -p -- "$STEAM_COMPAT_DATA_PATH"
cd -- "$game_dir"
exec env -u http_proxy -u https_proxy -u all_proxy -u HTTP_PROXY -u HTTPS_PROXY -u ALL_PROXY \
  "$proton" waitforexitandrun "$game_dir/AzurPromilia.exe" -start=azurpromilia_launcher \
  -screen-fullscreen "${AZUR_FULLSCREEN:-0}" -screen-width "${AZUR_WIDTH:-1280}" \
  -screen-height "${AZUR_HEIGHT:-720}" "$@"
