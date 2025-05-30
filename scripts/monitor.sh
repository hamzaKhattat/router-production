#!/bin/bash
# Monitor router and providers

while true; do
   clear
   echo "=== ROUTER MONITOR - $(date) ==="
   echo ""
   
   # Get stats via API
   curl -s http://localhost:8001/api/stats 2>/dev/null | jq . || echo "Router not running"
   
   echo ""
   echo "Press Ctrl+C to exit"
   sleep 5
done
