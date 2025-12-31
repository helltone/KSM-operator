#!/bin/bash

echo "══════════════════════════════════════════════════════════════════════════════"
echo "                        KSM Sharding Analysis Report                          "
echo "══════════════════════════════════════════════════════════════════════════════"
echo ""

# Get all KSM pods
kubectl get pods -n monitoring -l app.kubernetes.io/name=kube-state-metrics \
  -o custom-columns=POD:metadata.name,NODE:spec.nodeName,SHARD:metadata.labels.shard-id,REPLICA:metadata.labels.shard-replica,STATUS:status.phase \
  --no-headers > /tmp/ksm_pods.txt

echo "┌──────────────────────────────────────────┬─────────────────────┬───────┬─────────┬──────────┬──────────┐"
echo "│ Pod Name                                 │ Node                │ Shard │ Replica │ Status   │ Metrics  │"
echo "├──────────────────────────────────────────┼─────────────────────┼───────┼─────────┼──────────┼──────────┤"

port=9090
while IFS= read -r line; do
    pod=$(echo "$line" | awk '{print $1}')
    node=$(echo "$line" | awk '{print $2}')
    shard=$(echo "$line" | awk '{print $3}')
    replica=$(echo "$line" | awk '{print $4}')
    status=$(echo "$line" | awk '{print $5}')
    
    if [ "$status" = "Running" ]; then
        kubectl port-forward -n monitoring $pod $port:8080 > /dev/null 2>&1 &
        pf_pid=$!
        sleep 2
        
        metrics=$(curl -s localhost:$port/metrics 2>/dev/null)
        if [ -n "$metrics" ]; then
            total=$(echo "$metrics" | grep -c "^kube_" || echo "0")
        else
            total="0"
        fi
        
        kill $pf_pid 2>/dev/null || true
        wait $pf_pid 2>/dev/null || true
        port=$((port + 1))
    else
        total="0"
    fi
    
    printf "│ %-40s │ %-19s │ %5s │ %7s │ %-8s │ %8s │\n" "$pod" "$node" "$shard" "$replica" "$status" "$total"
done < /tmp/ksm_pods.txt

echo "└──────────────────────────────────────────┴─────────────────────┴───────┴─────────┴──────────┴──────────┘"
echo ""
echo "Sharding verification:"
echo ""

# Show which shards are active
for shard_id in $(awk '{print $3}' /tmp/ksm_pods.txt | sort -u); do
    echo "Shard $shard_id:"
    grep " $shard_id " /tmp/ksm_pods.txt | while IFS= read -r line; do
        pod=$(echo "$line" | awk '{print $1}')
        replica=$(echo "$line" | awk '{print $4}')
        status=$(echo "$line" | awk '{print $5}')
        echo "  - Replica $replica: $pod ($status)"
    done
    echo ""
done

rm -f /tmp/ksm_pods.txt
echo "Analysis complete!"