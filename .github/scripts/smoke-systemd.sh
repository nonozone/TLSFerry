#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
test_root=$(mktemp -d "${RUNNER_TEMP:-/tmp}/tlsferry-systemd-smoke.XXXXXX")
binary_path="$test_root/tlsferry"
user_id=$(id -u)
linger_before=$(loginctl show-user "$USER" --property=Linger --value)
service_installed=false

cleanup() {
  if [[ "$service_installed" == true && -x "$binary_path" ]]; then
    "$binary_path" service uninstall >/dev/null 2>&1 || true
  fi
  if [[ "$linger_before" != yes ]]; then
    sudo loginctl disable-linger "$USER" >/dev/null 2>&1 || true
  fi
  rm -R -- "$test_root"
}
trap cleanup EXIT

cd "$repo_root"
go build -trimpath -o "$binary_path" ./cmd/tlsferry
go run ./internal/releasetestfixture --root "$test_root"

if [[ "$linger_before" != yes ]]; then
  sudo loginctl enable-linger "$USER"
fi
sudo systemctl start "user@${user_id}.service"
export XDG_RUNTIME_DIR="/run/user/$user_id"
export DBUS_SESSION_BUS_ADDRESS="unix:path=$XDG_RUNTIME_DIR/bus"

loginctl show-user "$USER" --property=Linger
systemctl --user is-system-running || true

current_epoch=$(date +%s)
target_epoch=$(( (current_epoch / 60 + 1) * 60 ))
schedule_hour=$(date --date="@$target_epoch" +%-H)
schedule_minute=$(date --date="@$target_epoch" +%-M)
"$binary_path" service install \
  --config "$test_root/config.json" \
  --state-dir "$test_root/state" \
  --output-dir "$test_root/certificates" \
  --hour "$schedule_hour" \
  --minute "$schedule_minute" \
  --accept-tos \
  --execute
service_installed=true

"$binary_path" service status
systemctl --user cat tlsferry-renew.timer

# Activate once to establish the persistent timer stamp, stop it before the
# next scheduled minute, then restart it after that minute has been missed.
systemctl --user stop tlsferry-renew.timer
while (( $(date +%s) < target_epoch + 2 )); do
  sleep 1
done
systemctl --user start tlsferry-renew.timer

deadline=$((SECONDS + 30))
while true; do
  last_trigger=$(systemctl --user show tlsferry-renew.timer --property=LastTriggerUSec --value)
  service_result=$(systemctl --user show tlsferry-renew.service --property=Result --value)
  if [[ -n "$last_trigger" && "$last_trigger" != n/a && "$service_result" == success ]]; then
    break
  fi
  if (( SECONDS >= deadline )); then
    echo "systemd Persistent=true timer did not recover the missed run" >&2
    exit 1
  fi
  sleep 1
done

systemctl --user show tlsferry-renew.timer --property=ActiveState --property=LastTriggerUSec --property=NextElapseUSecRealtime
systemctl --user show tlsferry-renew.service --property=ActiveState --property=Result --property=ExecMainStatus

"$binary_path" service run-now
systemctl --user show tlsferry-renew.service --property=ActiveState --property=Result --property=ExecMainStatus
if [[ "$(systemctl --user show tlsferry-renew.service --property=Result --value)" != success ]]; then
  echo "systemd run-now did not complete successfully" >&2
  exit 1
fi
"$binary_path" service logs
journalctl --user --unit tlsferry-renew.service --no-pager --lines=20

"$binary_path" service uninstall
service_installed=false
"$binary_path" service status
if systemctl --user is-enabled --quiet tlsferry-renew.timer; then
  echo "systemd timer remains enabled after uninstall" >&2
  exit 1
fi

trap - EXIT
cleanup
