#!/usr/bin/env bash
# ==============================================================================
# OpenMaster Kernel Firewall & Anti-DDoS Rate-Limiter
# ==============================================================================
# Applies Linux kernel hashlimit rules to the DOCKER-USER chain to drop incoming
# packet floods and abusive connection bursts BEFORE they reach Docker containers.
# ==============================================================================

set -euo pipefail

BIND_IP="${BIND_IP:-135.148.72.145}"

# Master server game ports
UDP_PORTS="27900,27950,20510,20610,20710,20810,28060,29060,20110,27953,29250,27777"
TCP_PORTS="28900,28902,28910"
WEB_PORTS="80,8080"

echo "=========================================================="
echo " Setting up OpenMaster Firewall on ${BIND_IP}"
echo "=========================================================="

if [[ $EUID -ne 0 ]]; then
   echo "Error: This script must be run as root (use sudo)."
   exit 1
fi

# Ensure DOCKER-USER chain exists (Docker creates it automatically on startup)
if ! iptables -L DOCKER-USER -n >/dev/null 2>&1; then
    echo "Creating DOCKER-USER chain..."
    iptables -N DOCKER-USER || true
fi

echo "[1/3] Flushing existing OpenMaster DOCKER-USER rules..."
# Remove any prior OpenMaster hashlimit rules from DOCKER-USER to prevent duplication
iptables -D DOCKER-USER -d "${BIND_IP}" -p udp -m multiport --dports "${UDP_PORTS}" -m hashlimit --hashlimit-upto 10/sec --hashlimit-burst 20 --hashlimit-mode srcip --hashlimit-name om_udp_rate -j ACCEPT 2>/dev/null || true
iptables -D DOCKER-USER -d "${BIND_IP}" -p udp -m multiport --dports "${UDP_PORTS}" -j DROP 2>/dev/null || true
iptables -D DOCKER-USER -d "${BIND_IP}" -p tcp -m multiport --dports "${TCP_PORTS}" -m state --state NEW -m hashlimit --hashlimit-upto 5/sec --hashlimit-burst 15 --hashlimit-mode srcip --hashlimit-name om_tcp_rate -j ACCEPT 2>/dev/null || true
iptables -D DOCKER-USER -d "${BIND_IP}" -p tcp -m multiport --dports "${TCP_PORTS}" -m state --state NEW -j DROP 2>/dev/null || true

echo "[2/3] Installing kernel hashlimit rules..."

# Rule A: Allow up to 10 UDP packets/sec per source IP (burst of 20) on master UDP ports
iptables -I DOCKER-USER 1 -d "${BIND_IP}" -p udp -m multiport --dports "${UDP_PORTS}" \
    -m hashlimit --hashlimit-upto 10/sec --hashlimit-burst 20 \
    --hashlimit-mode srcip --hashlimit-name om_udp_rate -j ACCEPT

# Rule B: Drop all excess UDP packets beyond the rate limit
iptables -I DOCKER-USER 2 -d "${BIND_IP}" -p udp -m multiport --dports "${UDP_PORTS}" -j DROP

# Rule C: Allow up to 5 new TCP connections/sec per source IP (burst of 15) on master TCP ports
iptables -I DOCKER-USER 3 -d "${BIND_IP}" -p tcp -m multiport --dports "${TCP_PORTS}" \
    -m state --state NEW \
    -m hashlimit --hashlimit-upto 5/sec --hashlimit-burst 15 \
    --hashlimit-mode srcip --hashlimit-name om_tcp_rate -j ACCEPT

# Rule D: Drop all excess new TCP connection attempts
iptables -I DOCKER-USER 4 -d "${BIND_IP}" -p tcp -m multiport --dports "${TCP_PORTS}" \
    -m state --state NEW -j DROP

echo "[3/3] Verifying active DOCKER-USER rules:"
iptables -L DOCKER-USER -v -n --line-numbers

echo ""
echo "=========================================================="
echo " Firewall rules successfully applied!"
echo " Incoming floods beyond limits will be dropped by kernel."
echo "=========================================================="
echo "To persist rules across reboots, run:"
echo "  sudo apt-get install -y iptables-persistent && sudo netfilter-persistent save"
