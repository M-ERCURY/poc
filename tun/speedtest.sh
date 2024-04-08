#!/bin/sh
set -e

fatal () { echo "FATAL: $@"; exit 1; }

for cmd in mercury curl; do
    command -v "$cmd" >/dev/null || fatal "$cmd was not found in $PATH"
done

MERCURY="$(which mercury)"
CURL="curl"

speed () {
    set -x
    $CURL -sS \
        -o /dev/null \
        -w 'scale=2;%{speed_download}/1024/1024\n' \
        'https://speed.hetzner.de/100MB.bin' | bc
    set +x
}

end () {
    set -x
    sudo $MERCURY tun stop || true
    mercury stop || true
    set +x
    echo "Results written to $OUT"
    exit 1
}

trap end ERR

echo 'Warning, this test suite may consume several hundreds of MB of bandwidth.'
echo 'Are you sure you want to proceed? (Return to continue, ^C to quit)'
read

echo 'Commands will be echoed before being ran. At any point, ^C to cancel and quit.'

OUT="/tmp/speedtest-$(date +'%s').md"

cat <<EOF> "$OUT"
# Speed test results for $(date +'%F %T %z')

Avg speed vanilla: $(speed) MiB/s
EOF

set -x
$MERCURY status >/dev/null || $MERCURY start
set +x

sleep 3
echo "Avg speed via SOCKSv5: $(CURL="$MERCURY exec curl" speed) MiB/s" >> "$OUT"

set -x
sudo $MERCURY tun start
set +x

echo "Avg speed via tun device: $(speed) MiB/s" >> "$OUT"

set -x
sudo $MERCURY tun stop
mercury stop
set +x

echo "Done."
echo "Results written to $OUT"
