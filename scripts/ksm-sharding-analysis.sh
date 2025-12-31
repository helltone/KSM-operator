#!/bin/bash

# KSM Sharding Analysis Script
# Analyzes kube-state-metrics sharding configuration and metrics distribution

set -euo pipefail

NAMESPACE=${1:-monitoring}

echo "══════════════════════════════════════════════════════════════════════════════"
echo "                        KSM Sharding Analysis Report                          "
echo "══════════════════════════════════════════════════════════════════════════════"
echo ""

# Create temp files
rm -f /tmp/ksm_*.txt
touch /tmp/ksm_analysis.txt

# Get all KSM pods
echo "Searching for KSM pods in namespace: $NAMESPACE"
if ! kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/name=kube-state-metrics \
    -o custom-columns=POD:metadata.name,NODE:spec.nodeName,SHARD:metadata.labels.shard-id,REPLICA:metadata.labels.shard-replica,STATUS:status.phase \
    --no-headers > /tmp/ksm_pods.txt; then
    echo "Error: No KSM pods found in namespace '$NAMESPACE'"
    exit 1
fi

if [ ! -s /tmp/ksm_pods.txt ]; then
    echo "Error: No KSM pods found in namespace '$NAMESPACE'"
    exit 1
fi

# Process each pod
echo "Collecting metrics from pods..." >&2
port=9090
while IFS= read -r line; do
    pod=$(echo "$line" | awk '{print $1}')
    node=$(echo "$line" | awk '{print $2}')
    shard=$(echo "$line" | awk '{print $3}')
    replica=$(echo "$line" | awk '{print $4}')
    status=$(echo "$line" | awk '{print $5}')
    
    [ -z "$pod" ] && continue
    printf "  %-45s" "$pod..." >&2
    
    if [ "$status" = "Running" ]; then
        kubectl port-forward -n "$NAMESPACE" "$pod" "$port:8080" > /dev/null 2>&1 &
        pf_pid=$!
        sleep 2
        
        if kill -0 $pf_pid 2>/dev/null; then
            metrics=$(curl -s "localhost:$port/metrics" 2>/dev/null || echo "")
            if [ -n "$metrics" ]; then
                total=$(echo "$metrics" | grep -c "^kube_" || echo "0")
                pods=$(echo "$metrics" | grep -c "^kube_pod_info" || echo "0")
                deployments=$(echo "$metrics" | grep -c "^kube_deployment_" || echo "0")
                services=$(echo "$metrics" | grep -c "^kube_service_" || echo "0")
                nodes=$(echo "$metrics" | grep -c "^kube_node_" || echo "0")
                namespaces=$(echo "$metrics" | grep -c "^kube_namespace_" || echo "0")
                echo " ✓" >&2
            else
                total="0"; pods="0"; deployments="0"; services="0"; nodes="0"; namespaces="0"
                echo " ✗ (no metrics)" >&2
            fi
        else
            total="0"; pods="0"; deployments="0"; services="0"; nodes="0"; namespaces="0"
            echo " ✗ (port-forward failed)" >&2
        fi
        
        kill $pf_pid 2>/dev/null || true
        wait $pf_pid 2>/dev/null || true
        port=$((port + 1))
    else
        total="0"; pods="0"; deployments="0"; services="0"; nodes="0"; namespaces="0"
        echo " ✗ ($status)" >&2
    fi
    
    echo "$pod|$node|$shard|$replica|$total|$pods|$deployments|$services|$nodes|$namespaces" >> /tmp/ksm_analysis.txt
done < /tmp/ksm_pods.txt

echo "" >&2
echo ""

# Display results
printf "%-42s %-22s %8s %8s %10s %8s\n" "Pod Name" "Node" "Shard" "Replica" "Status" "Metrics"
printf "%-42s %-22s %8s %8s %10s %8s\n" "--------" "----" "-----" "-------" "------" "-------"

while IFS='|' read -r pod node shard replica total pods deployments services nodes namespaces; do
    if [ -n "$pod" ] && [ "$pod" != "POD" ]; then
        if [ "$total" = "0" ]; then
            status="FAILED"
        else
            status="OK"
        fi
        printf "%-42s %-22s %8s %8s %10s %8s\n" "$pod" "$node" "$shard" "$replica" "$status" "$total"
    fi
done < /tmp/ksm_analysis.txt

echo ""
echo "Metrics Breakdown by Shard:"
echo ""
printf "%6s %8s %8s %8s %8s %8s %8s %12s %8s\n" "Shard" "Replica" "Total" "Pods" "Deploy" "Service" "Nodes" "Namespaces" "Status"
printf "%6s %8s %8s %8s %8s %8s %8s %12s %8s\n" "-----" "-------" "-----" "----" "------" "-------" "-----" "-----------" "------"

sort -t'|' -k3,3n -k4,4n /tmp/ksm_analysis.txt | while IFS='|' read -r pod node shard replica total pods deploys services nodes ns; do
    if [ -n "$pod" ] && [ "$pod" != "POD" ] && [ -n "$shard" ]; then
        if [ "$total" = "0" ]; then
            status="FAILED"
        else
            status="OK"
        fi
        printf "%6s %8s %8s %8s %8s %8s %8s %12s %8s\n" \
            "$shard" "$replica" "$total" "$pods" "$deploys" "$services" "$nodes" "$ns" "$status"
    fi
done

echo ""
echo "Summary:"
echo ""

total_pods=$(wc -l < /tmp/ksm_pods.txt | tr -d ' ')
configured_shards=$(kubectl get ksmscales -n "$NAMESPACE" -o jsonpath='{.items[0].spec.sharding.shardCount}' 2>/dev/null || echo "N/A")
min_replicas=$(kubectl get ksmscales -n "$NAMESPACE" -o jsonpath='{.items[0].spec.sharding.minReplicasPerShard}' 2>/dev/null || echo "N/A")
max_replicas=$(kubectl get ksmscales -n "$NAMESPACE" -o jsonpath='{.items[0].spec.sharding.maxReplicasPerShard}' 2>/dev/null || echo "N/A")
total_sum=$(awk -F'|' '{sum+=$5} END {print sum}' /tmp/ksm_analysis.txt)
avg_metrics=$(awk -F'|' 'BEGIN{sum=0;count=0} {if($5>0){sum+=$5;count++}} END {if(count>0) printf "%.0f", sum/count; else print "0"}' /tmp/ksm_analysis.txt)

printf "%-40s: %s\n" "Total KSM Pods" "$total_pods"
printf "%-40s: %s\n" "Configured Shards" "$configured_shards"
printf "%-40s: %s\n" "Replicas per Shard" "$min_replicas-$max_replicas"
printf "%-40s: %s\n" "Total Metrics (all replicas)" "$total_sum"
printf "%-40s: %s\n" "Average Metrics per Working Pod" "$avg_metrics"

echo ""
echo "Shard Configuration Verification:"
if [ "$configured_shards" != "N/A" ] && [ "$configured_shards" != "0" ]; then
    echo "Expected shards: 0 to $((configured_shards - 1)) (total: $configured_shards shards)"
else
    echo "Expected shards: Not configured or non-sharded mode"
fi

actual_shards=$(cut -d'|' -f3 /tmp/ksm_analysis.txt | sort -u | grep -v '^$' | tr '\n' ' ')
echo "Actual shards found: $actual_shards"

echo ""
echo "Shard Consistency Check:"
for shard_id in $(cut -d'|' -f3 /tmp/ksm_analysis.txt | sort -u | grep -v '^$'); do
    if [ "$shard_id" != "<none>" ] && [ -n "$shard_id" ]; then
        echo "  Shard $shard_id:"
        grep "|$shard_id|" /tmp/ksm_analysis.txt | while IFS='|' read -r pod node shard replica total rest; do
            if [ -n "$pod" ] && [ -n "$replica" ]; then
                printf "    Replica %s: %5s metrics (Pod: %s)\n" "$replica" "$total" "$pod"
            fi
        done
    fi
done

# Cleanup
rm -f /tmp/ksm_*.txt

echo ""
echo "Analysis complete!"