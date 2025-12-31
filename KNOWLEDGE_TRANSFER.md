# Kubernetes Operator Knowledge Transfer
*Complete Guide for Go and Kubernetes Operator Beginners*

## Table of Contents
1. [Learning Prerequisites](#learning-prerequisites)
2. [Project Overview](#project-overview)
3. [Go Language Fundamentals Used](#go-language-fundamentals-used)
4. [Kubernetes Concepts](#kubernetes-concepts)
5. [Operator Pattern Deep Dive](#operator-pattern-deep-dive)
6. [Project Structure Analysis](#project-structure-analysis)
7. [Code Walkthrough](#code-walkthrough)
8. [Dependencies and Libraries](#dependencies-and-libraries)
9. [Development Workflow](#development-workflow)
10. [Testing Strategy](#testing-strategy)
11. [Learning Plan](#learning-plan)
12. [Common Patterns and Best Practices](#common-patterns-and-best-practices)
13. [Troubleshooting Guide](#troubleshooting-guide)
14. [Next Steps](#next-steps)
15. [Implementation Details](#implementation-details)

---

## Learning Prerequisites

Before diving into this operator, you should understand:

### Essential Knowledge
- **Basic Go syntax**: variables, functions, structs, interfaces
- **Basic Kubernetes**: pods, services, deployments, namespaces
- **Command line**: bash, git, kubectl basics
- **YAML**: Understanding of YAML syntax

### Helpful but Not Required
- Docker containerization
- Kubernetes RBAC (Role-Based Access Control)
- Prometheus/monitoring concepts

---

## Project Overview

### What This Operator Does
The **KSM Scale Operator** automatically scales `kube-state-metrics` pods based on cluster size. As your Kubernetes cluster grows, it ensures you have the right number of monitoring pods to handle the load.

**Simple Example:**
- 15 nodes in cluster → 1 kube-state-metrics pod
- 30 nodes in cluster → 2 kube-state-metrics pods  
- 45 nodes in cluster → 3 kube-state-metrics pods

### Project Name and Resource Type
- **Operator Name**: KSM Scale Operator
- **CRD Kind**: `KSMScale`
- **API Group**: `monitor.example.com`
- **API Version**: `v1alpha1`
- **Repository**: github.com/example/ksm-scale-operator

### Why Operators Exist
Kubernetes operators extend Kubernetes with custom logic. Instead of manually scaling monitoring pods, the operator does it automatically by:

1. **Watching** for changes in node count
2. **Calculating** how many monitoring pods are needed
3. **Creating/Deleting** pods as needed
4. **Maintaining** desired state continuously

---

## Go Language Fundamentals Used

This section explains Go concepts used in the operator code.

### 1. Struct Types
```go
type KSMScaleSpec struct {
    NodesPerMonitor int32  `json:"nodesPerMonitor,omitempty"`
    MonitorImage    string `json:"monitorImage,omitempty"`
    Namespace       string `json:"namespace,omitempty"`
    Resources       ResourceRequirements `json:"resources,omitempty"`
}
```

**What it is:** A struct defines a custom data type with fields
**Why used here:** Represents the configuration users want for their monitoring setup
**Key concept:** The `json:"..."` tags tell Go how to convert between Go structs and JSON/YAML

### 2. Interfaces
```go
type client.Client interface {
    Get(ctx context.Context, key client.ObjectKey, obj client.Object) error
    List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error
    Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error
    // ... more methods
}
```

**What it is:** An interface defines methods that types must implement
**Why used here:** The `client.Client` interface lets us interact with Kubernetes API
**Key concept:** Interfaces enable polymorphism - different implementations can satisfy the same interface

### 3. Embedded Fields
```go
type KSMScaleReconciler struct {
    client.Client  // Embedded field
    Scheme *runtime.Scheme
}
```

**What it is:** Embedding a field gives direct access to its methods
**Why used here:** `KSMScaleReconciler` can call `Get()`, `List()`, etc. directly
**Key concept:** This is Go's way of achieving composition over inheritance

### 4. Method Receivers
```go
func (r *KSMScaleReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
    // Implementation
}
```

**What it is:** Methods can be attached to types using receivers
**Why used here:** `Reconcile` is a method on `KSMScaleReconciler`
**Key concept:** `*KSMScaleReconciler` means the method can modify the receiver

### 5. Error Handling
```go
if err != nil {
    logger.Error(err, "Failed to list nodes")
    return ctrl.Result{}, err
}
```

**What it is:** Go uses explicit error checking instead of exceptions
**Why used here:** Every operation that can fail returns an error
**Key concept:** Always check errors immediately after operations

### 6. Context
```go
func (r *KSMScaleReconciler) Reconcile(ctx context.Context, req ctrl.Request) {
    logger := log.FromContext(ctx)
    err := r.Get(ctx, req.NamespacedName, ksmScale)
}
```

**What it is:** Context carries deadlines, cancellation signals, and values across API boundaries
**Why used here:** Kubernetes operations can be cancelled or timeout
**Key concept:** Always pass context to operations that might block

### 7. Pointers and References
```go
pod := &corev1.Pod{  // & creates a pointer
    ObjectMeta: metav1.ObjectMeta{
        Name: "my-pod",
    },
}
```

**What it is:** `&` creates a pointer to a value
**Why used here:** Kubernetes objects are large structs; we pass pointers for efficiency
**Key concept:** Use pointers when you need to modify objects or avoid copying

---

## Kubernetes Concepts

### 1. Custom Resource Definitions (CRDs)
```yaml
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: ksmscales.monitor.example.com
```

**What it is:** Extends Kubernetes API with custom resource types
**Purpose:** Lets us define our own `KSMScale` resources
**Key concept:** CRDs make Kubernetes programmable beyond built-in types

### 2. Controllers
**What it is:** A control loop that watches resources and maintains desired state
**Purpose:** Implements the operator logic
**Key concept:** Controllers reconcile actual state with desired state

### 3. Reconciliation Loop
```
1. Watch for changes to KSMScale resources
2. Calculate desired number of pods based on node count
3. Compare current pods vs desired pods
4. Create/delete pods as needed
5. Update status
6. Repeat
```

**Key concept:** This is the heart of operator pattern - continuous reconciliation

### 4. RBAC (Role-Based Access Control)
```yaml
# +kubebuilder:rbac:groups="",resources=nodes,verbs=get;list;watch
# +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch;create;update;patch;delete
```

**What it is:** Security mechanism that controls what the operator can do
**Purpose:** Principle of least privilege - only grant necessary permissions
**Key concept:** Operators need permissions to manage the resources they control

### 5. Namespaces
**What it is:** Virtual clusters within a Kubernetes cluster
**Purpose:** Isolation and organization of resources
**Key concept:** Our operator can deploy pods to any namespace specified by user

---

## Operator Pattern Deep Dive

### The Controller-Runtime Framework

This operator uses `controller-runtime`, which provides:

1. **Manager**: Coordinates multiple controllers
2. **Client**: Unified interface to Kubernetes API
3. **Controller**: Handles reconciliation logic
4. **Webhook**: Validates/mutates resources (not used in this operator)

### Operator Lifecycle

```
1. Startup
   ├── Initialize Kubernetes client
   ├── Register CRDs
   ├── Set up RBAC
   └── Start controller

2. Runtime (Continuous Loop)
   ├── Watch for KSMScale changes
   ├── Watch for Node changes  
   ├── Watch for Pod changes (owned by KSMScale)
   ├── Trigger reconciliation on any change
   └── Update resource status

3. Reconciliation (Per Event)
   ├── Get current KSMScale resource
   ├── Count cluster nodes
   ├── Calculate desired pod count
   ├── Get current pods
   ├── Scale up/down as needed
   └── Update status
```

### Event-Driven Architecture

The operator is **reactive**, not proactive:
- It doesn't poll for changes
- Kubernetes notifies it when things change
- It only acts when necessary

---

## Project Structure Analysis

```
ksm-scale-operator/
├── api/v1alpha1/                    # CRD definitions
│   ├── ksmscale_types.go            # Go structs for our CRD
│   ├── groupversion_info.go         # Group version registration
│   └── zz_generated.deepcopy.go     # Auto-generated Kubernetes boilerplate
├── cmd/manager/
│   └── main.go                      # Operator entry point
├── config/                          # Kubernetes manifests
│   ├── crd/bases/                   # Generated CRD YAML
│   ├── rbac/                        # RBAC permissions
│   ├── manager/                     # Operator deployment
│   └── dev/                         # Development configs
├── controllers/                     # Business logic
│   ├── ksmscale_controller.go       # Main controller logic
│   └── ksmscale_controller_test.go  # Unit tests
├── manifests/                       # Sample resources
├── scripts/                         # Development scripts
├── go.mod                          # Go module definition
├── go.sum                          # Go module checksums
├── Dockerfile                      # Production container
├── Dockerfile.dev                 # Development container
└── Makefile                        # Build automation
```

### Key File Purposes

| File | Purpose | When to Modify |
|------|---------|----------------|
| `api/v1alpha1/ksmscale_types.go` | Define CRD structure | Adding new fields to KSMScale |
| `controllers/ksmscale_controller.go` | Business logic | Changing scaling behavior |
| `config/crd/bases/monitor.example.com_ksmscales.yaml` | CRD definition | After modifying types.go |
| `config/rbac/*.yaml` | Permissions | When accessing new Kubernetes resources |
| `manifests/sample-ksmscale*.yaml` | Examples | Providing user examples |

---

## Code Walkthrough

Let's walk through the main components with detailed explanations.

### 1. CRD Definition (`api/v1alpha1/ksmscale_types.go`)

```go
package v1alpha1

import (
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// KSMScaleSpec defines the desired state of KSMScale
type KSMScaleSpec struct {
    // Number of nodes per kube-state-metrics pod
    // +kubebuilder:validation:Minimum=9
    // +kubebuilder:default=15
    NodesPerMonitor int32 `json:"nodesPerMonitor,omitempty"`
    
    // Container image for kube-state-metrics
    // +kubebuilder:default="registry.k8s.io/kube-state-metrics/kube-state-metrics:v2.10.1"
    MonitorImage string `json:"monitorImage,omitempty"`
    
    // Target namespace for kube-state-metrics pods
    // +kubebuilder:default="monitoring"
    Namespace string `json:"namespace,omitempty"`
    
    // Resource requirements for kube-state-metrics pods
    Resources ResourceRequirements `json:"resources,omitempty"`
}
```

**Breakdown:**
- `+kubebuilder:` comments are **code generation directives**
- `json:"..."` tags control JSON/YAML field names
- `omitempty` means the field is optional
- `int32` is used instead of `int` for Kubernetes compatibility

```go
// KSMScaleStatus defines the observed state of KSMScale
type KSMScaleStatus struct {
    CurrentReplicas int32       `json:"currentReplicas,omitempty"`
    DesiredReplicas int32       `json:"desiredReplicas,omitempty"`
    NodeCount       int32       `json:"nodeCount,omitempty"`
    Phase           string      `json:"phase,omitempty"`
    LastUpdated     metav1.Time `json:"lastUpdated,omitempty"`
}
```

**Why separate Spec and Status:**
- **Spec**: What the user wants (desired state)
- **Status**: What's actually happening (observed state)
- **Pattern**: This is standard Kubernetes design

```go
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:printcolumn:name="Nodes",type=integer,JSONPath=`.status.nodeCount`
// +kubebuilder:printcolumn:name="Desired",type=integer,JSONPath=`.status.desiredReplicas`
// +kubebuilder:printcolumn:name="Current",type=integer,JSONPath=`.status.currentReplicas`
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
type KSMScale struct {
    metav1.TypeMeta   `json:",inline"`
    metav1.ObjectMeta `json:"metadata,omitempty"`

    Spec   KSMScaleSpec   `json:"spec,omitempty"`
    Status KSMScaleStatus `json:"status,omitempty"`
}
```

**Embedding explanation:**
- `metav1.TypeMeta`: Provides `Kind` and `APIVersion` fields
- `metav1.ObjectMeta`: Provides `Name`, `Namespace`, `Labels`, etc.
- `json:",inline"` flattens the embedded struct in JSON

### 2. Controller Logic (`controllers/ksmscale_controller.go`)

#### Imports and Dependencies
```go
import (
    "context"           // For handling cancellation and timeouts
    "fmt"              // String formatting
    "math"             // Mathematical operations (ceil)
    "strconv"          // String conversions
    "time"             // Time operations

    corev1 "k8s.io/api/core/v1"                    // Core Kubernetes types (Pod, Node)
    "k8s.io/apimachinery/pkg/api/errors"           // Kubernetes error types
    "k8s.io/apimachinery/pkg/api/resource"         // Resource quantities (CPU, memory)
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"  // Common metadata types
    "k8s.io/apimachinery/pkg/runtime"              // Kubernetes runtime types
    "k8s.io/apimachinery/pkg/util/intstr"          // IntOrString type
    ctrl "sigs.k8s.io/controller-runtime"          // Controller framework
    "sigs.k8s.io/controller-runtime/pkg/client"    // Kubernetes client
    "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil" // Utilities
    "sigs.k8s.io/controller-runtime/pkg/log"       // Logging

    monitorv1alpha1 "github.com/example/ksm-scale-operator/api/v1alpha1"
)
```

#### Controller Struct
```go
type KSMScaleReconciler struct {
    client.Client          // Embedded Kubernetes client
    Scheme *runtime.Scheme // Kubernetes type registry
}
```

**Why embedding Client:**
- Gives direct access to `Get()`, `List()`, `Create()`, `Update()`, `Delete()`
- No need to write `r.Client.Get()`, just `r.Get()`

#### RBAC Annotations
```go
// +kubebuilder:rbac:groups=monitor.example.com,resources=ksmscales,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=monitor.example.com,resources=ksmscales/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=monitor.example.com,resources=ksmscales/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=nodes,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch;create;update;patch;delete
```

**Translation:**
- Can manage KSMScale resources (our CRD)
- Can read all nodes in cluster
- Can manage pods in any namespace
- Can update status and finalizers

#### Main Reconciliation Logic
```go
func (r *KSMScaleReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
    logger := log.FromContext(ctx)

    // 1. Get the KSMScale resource that triggered this reconciliation
    ksmScale := &monitorv1alpha1.KSMScale{}
    err := r.Get(ctx, req.NamespacedName, ksmScale)
    if err != nil {
        if errors.IsNotFound(err) {
            // Resource was deleted, nothing to do
            return ctrl.Result{}, nil
        }
        return ctrl.Result{}, err
    }

    // 2. Count nodes in the cluster
    nodeList := &corev1.NodeList{}
    if err := r.List(ctx, nodeList); err != nil {
        logger.Error(err, "Failed to list nodes")
        return ctrl.Result{}, err
    }
    nodeCount := len(nodeList.Items)

    // 3. Calculate desired number of monitoring pods
    nodesPerMonitor := int(ksmScale.Spec.NodesPerMonitor)
    if nodesPerMonitor == 0 {
        nodesPerMonitor = 15 // default
    }
    
    desiredReplicas := int(math.Ceil(float64(nodeCount) / float64(nodesPerMonitor)))
    if desiredReplicas == 0 && nodeCount > 0 {
        desiredReplicas = 1 // always have at least one if there are nodes
    }

    // 4. Get current monitoring pods
    namespace := ksmScale.Spec.Namespace
    if namespace == "" {
        namespace = "monitoring"
    }
    
    currentPods, err := r.getCurrentMonitorPods(ctx, ksmScale, namespace)
    if err != nil {
        logger.Error(err, "Failed to get current monitor pods")
        return ctrl.Result{}, err
    }
    currentReplicas := len(currentPods)

    // 5. Scale up or down as needed
    if currentReplicas < desiredReplicas {
        // Scale up: create missing pods
        // ... implementation details below
    } else if currentReplicas > desiredReplicas {
        // Scale down: delete excess pods
        // ... implementation details below
    }

    // 6. Update status
    ksmScale.Status.NodeCount = int32(nodeCount)
    ksmScale.Status.DesiredReplicas = int32(desiredReplicas)
    ksmScale.Status.CurrentReplicas = int32(desiredReplicas)  // Note: This should be currentReplicas after scaling
    ksmScale.Status.LastUpdated = metav1.NewTime(time.Now())
    
    if currentReplicas == desiredReplicas {
        ksmScale.Status.Phase = "Ready"
    } else {
        ksmScale.Status.Phase = "Scaling"
    }

    if err := r.Status().Update(ctx, ksmScale); err != nil {
        logger.Error(err, "Failed to update KSMScale status")
        return ctrl.Result{}, err
    }

    // 7. Requeue after 1 minute for regular health checks
    return ctrl.Result{RequeueAfter: time.Minute}, nil
}
```

**Key concepts:**
- **Reconcile** is called every time something changes
- **ctx context.Context** allows cancellation and carries request-scoped values
- **ctrl.Result** controls whether/when to run again
- **Error handling** at every step with explicit checks

#### Pod Creation Logic
```go
func (r *KSMScaleReconciler) createMonitorPod(ctx context.Context, ksmScale *monitorv1alpha1.KSMScale, namespace string, index int) error {
    // Set default image if not specified
    image := ksmScale.Spec.MonitorImage
    if image == "" {
        image = "registry.k8s.io/kube-state-metrics/kube-state-metrics:v2.10.1"
    }

    pod := &corev1.Pod{
        ObjectMeta: metav1.ObjectMeta{
            Name:      fmt.Sprintf("kube-state-metrics-%s-%d", ksmScale.Name, index),
            Namespace: namespace,
            Labels: map[string]string{
                "app.kubernetes.io/name":     "kube-state-metrics",
                "app.kubernetes.io/instance": ksmScale.Name,
                "app.kubernetes.io/component": "exporter",
                "ksmscale.example.com":       ksmScale.Name,
                "index":                      fmt.Sprintf("%d", index),
            },
        },
        Spec: corev1.PodSpec{
            ServiceAccountName: "kube-state-metrics",
            Containers: []corev1.Container{
                {
                    Name:  "kube-state-metrics",
                    Image: image,
                    Args: []string{
                        "--host=0.0.0.0",
                        "--port=8080",
                        "--telemetry-host=0.0.0.0",
                        "--telemetry-port=8081",
                    },
                    Ports: []corev1.ContainerPort{
                        {
                            Name:          "http-metrics",
                            ContainerPort: 8080,
                            Protocol:      corev1.ProtocolTCP,
                        },
                        {
                            Name:          "telemetry",
                            ContainerPort: 8081,
                            Protocol:      corev1.ProtocolTCP,
                        },
                    },
                    LivenessProbe: &corev1.Probe{
                        ProbeHandler: corev1.ProbeHandler{
                            HTTPGet: &corev1.HTTPGetAction{
                                Path: "/healthz",
                                Port: intstr.FromInt(8080),
                            },
                        },
                        InitialDelaySeconds: 5,
                        TimeoutSeconds:      5,
                    },
                    ReadinessProbe: &corev1.Probe{
                        ProbeHandler: corev1.ProbeHandler{
                            HTTPGet: &corev1.HTTPGetAction{
                                Path: "/",
                                Port: intstr.FromInt(8080),
                            },
                        },
                        InitialDelaySeconds: 5,
                        TimeoutSeconds:      5,
                    },
                    SecurityContext: &corev1.SecurityContext{
                        AllowPrivilegeEscalation: &[]bool{false}[0],
                        Capabilities: &corev1.Capabilities{
                            Drop: []corev1.Capability{"ALL"},
                        },
                        ReadOnlyRootFilesystem: &[]bool{true}[0],
                        RunAsNonRoot:          &[]bool{true}[0],
                        RunAsUser:             &[]int64{65534}[0],
                    },
                },
            },
        },
    }

    // Set owner reference so pods are deleted when KSMScale is deleted
    if err := controllerutil.SetControllerReference(ksmScale, pod, r.Scheme); err != nil {
        return err
    }

    // Create the pod
    if err := r.Create(ctx, pod); err != nil {
        if !errors.IsAlreadyExists(err) {
            return err
        }
    }

    return nil
}
```

**Important patterns:**
- **Default values**: Always provide sensible defaults
- **Security**: Run as non-root, drop all capabilities, read-only filesystem
- **Health checks**: Liveness and readiness probes for reliability
- **Owner references**: Establishes parent-child relationship for cleanup

### 3. Main Entry Point (`cmd/manager/main.go`)

```go
func main() {
    // 1. Parse command line flags
    var metricsAddr string
    var enableLeaderElection bool
    flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
    flag.BoolVar(&enableLeaderElection, "leader-elect", false, "Enable leader election for controller manager.")
    flag.Parse()

    // 2. Set up logging
    ctrl.SetLogger(zap.New(zap.UseDevMode(true)))

    // 3. Create manager
    mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
        Scheme:             scheme.Scheme,
        MetricsBindAddress: metricsAddr,
        Port:               9443,
        LeaderElection:     enableLeaderElection,
        LeaderElectionID:   "ksm-scale-operator",
    })
    if err != nil {
        setupLog.Error(err, "unable to start manager")
        os.Exit(1)
    }

    // 4. Register our controller
    if err = (&controllers.KSMScaleReconciler{
        Client: mgr.GetClient(),
        Scheme: mgr.GetScheme(),
    }).SetupWithManager(mgr); err != nil {
        setupLog.Error(err, "unable to create controller", "controller", "KSMScale")
        os.Exit(1)
    }

    // 5. Start the manager (and all controllers)
    setupLog.Info("starting manager")
    if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
        setupLog.Error(err, "problem running manager")
        os.Exit(1)
    }
}
```

**Explanation:**
- **Manager** coordinates all controllers
- **Leader election** ensures only one operator instance is active
- **Signal handler** enables graceful shutdown
- **SetupWithManager** registers our controller with the manager

---

## Dependencies and Libraries

### Core Dependencies (`go.mod`)

```go
module github.com/example/ksm-scale-operator

go 1.21

require (
    k8s.io/api v0.28.0                      // Core Kubernetes types (Pod, Node, etc.)
    k8s.io/apimachinery v0.28.0             // Core Kubernetes utilities
    k8s.io/client-go v0.28.0                // Kubernetes Go client library
    sigs.k8s.io/controller-runtime v0.16.0  // Controller framework
)
```

### Library Deep Dive

#### 1. `k8s.io/api`
**Purpose:** Provides Go structs for all Kubernetes resource types
```go
import corev1 "k8s.io/api/core/v1"

// Now you can use:
pod := &corev1.Pod{}
node := &corev1.Node{}
service := &corev1.Service{}
```

#### 2. `k8s.io/apimachinery`
**Purpose:** Utilities for working with Kubernetes APIs
```go
import (
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/apimachinery/pkg/api/resource"
    "k8s.io/apimachinery/pkg/util/intstr"
)

// Common utilities:
time := metav1.NewTime(time.Now())
cpu := resource.MustParse("100m")
port := intstr.FromInt(8080)
```

#### 3. `k8s.io/client-go`
**Purpose:** Client libraries for talking to Kubernetes API
```go
import "k8s.io/client-go/kubernetes/scheme"

// Provides:
// - REST clients
// - Authentication
// - Serialization/deserialization
// - Informers (watch for changes)
```

#### 4. `sigs.k8s.io/controller-runtime`
**Purpose:** High-level framework for building controllers
```go
import ctrl "sigs.k8s.io/controller-runtime"

// Provides:
// - Manager (orchestrates controllers)
// - Client (unified interface to K8s API)  
// - Controller (handles reconciliation)
// - Builder pattern for setup
```

### Testing Dependencies
```go
require (
    github.com/onsi/ginkgo/v2 v2.11.0      // BDD testing framework
    github.com/onsi/gomega v1.27.8         // Matcher/assertion library
)
```

**Why Ginkgo/Gomega:**
- **Ginkgo**: Behavior-driven development (BDD) style tests
- **Gomega**: Rich assertion library with readable matchers
- **Standard**: Used throughout Kubernetes ecosystem

---

## Development Workflow

### 1. Code Generation Workflow

```bash
# 1. Modify CRD types
vim api/v1alpha1/ksmscale_types.go

# 2. Generate boilerplate code
go generate ./...

# 3. Generate CRDs
make manifests

# 4. Generate RBAC
make manifests
```

**What gets generated:**
- `zz_generated.deepcopy.go`: DeepCopy methods for Kubernetes
- `config/crd/bases/*.yaml`: CRD manifests
- `config/rbac/*.yaml`: RBAC manifests

### 2. Build and Test Workflow

```bash
# Local development
make run              # Run operator locally
make test             # Run unit tests
make fmt              # Format Go code
make vet              # Run Go vet

# Container development
make docker-build     # Build container image
make docker-push      # Push to registry

# Kind (local Kubernetes) development
make dev              # Complete local setup
make logs-kind        # Watch operator logs
make clean-dev        # Clean up
```

### 3. Deployment Workflow

```bash
# Install CRDs
make install-crd

# Deploy operator
make deploy

# Create sample resource
make sample

# Check status
kubectl get ksmscales
kubectl get pods -n monitoring
```

---

## Testing Strategy

### Unit Tests Structure

```go
var _ = Describe("KSMScale Controller", func() {
    Context("When reconciling a KSMScale", func() {
        It("should create 1 pod for 1-15 nodes", func() {
            // 1. Setup test data
            nodes := createTestNodes(10)
            ksmScale := createTestKSMScale()
            
            // 2. Create fake client with test data
            fakeClient := fake.NewClientBuilder().
                WithScheme(scheme.Scheme).
                WithRuntimeObjects(nodes, ksmScale).
                WithStatusSubresource(&monitorv1alpha1.KSMScale{}).
                Build()
                
            // 3. Create reconciler with fake client
            reconciler := &KSMScaleReconciler{
                Client: fakeClient,
                Scheme: scheme.Scheme,
            }
            
            // 4. Run reconciliation
            result, err := reconciler.Reconcile(ctx, reconcileRequest)
            
            // 5. Verify results
            Expect(err).NotTo(HaveOccurred())
            Expect(result.RequeueAfter).To(Equal(time.Minute))
            
            // 6. Check created pods
            podList := &corev1.PodList{}
            err = fakeClient.List(ctx, podList, client.InNamespace("monitoring"))
            Expect(err).NotTo(HaveOccurred())
            Expect(podList.Items).To(HaveLen(1))
        })
    })
})
```

### Testing Patterns

1. **Fake Client**: Use `fake.NewClientBuilder()` for unit tests
2. **BDD Style**: Use Ginkgo's `Describe`, `Context`, `It` structure
3. **Assertions**: Use Gomega's `Expect()` with readable matchers
4. **Test Data**: Create helper functions for common test objects
5. **Isolation**: Each test is independent with its own fake client

### Test Implementation Deep Dive

#### Test Framework Stack
```go
// Testing frameworks used:
import (
    . "github.com/onsi/ginkgo/v2"  // BDD-style test organization
    . "github.com/onsi/gomega"     // Fluent assertion library
    "sigs.k8s.io/controller-runtime/pkg/client/fake" // Kubernetes fake client
)
```

#### Test Structure and Organization

**File**: `controllers/ksmscale_controller_test.go`

The test file contains **18 comprehensive test cases** covering:

1. **Non-Sharded Mode Tests (13 tests)**:
   - Basic scaling scenarios (1 pod for 1-15 nodes, 2 pods for 16-30 nodes, etc.)
   - Custom nodesPerMonitor ratios
   - Resource limits validation
   - Zero nodes handling
   - Pod recreation with gaps in indexes
   - Scale up/down scenarios
   - Dynamic ratio changes

2. **Sharded Mode Tests (5 tests)**:
   - Minimum pod enforcement (3 shards = minimum 3 pods)
   - Round-robin pod distribution across shards
   - PodDisruptionBudget creation
   - Efficiency warnings for over-sharding
   - Shard distribution calculation testing

#### Key Test Concepts and Patterns

##### 1. Fake Client Setup
```go
fakeClient := fake.NewClientBuilder().
    WithScheme(scheme.Scheme).                    // Add our CRD schemas
    WithRuntimeObjects(objs...).                  // Pre-populate with test data
    WithStatusSubresource(&monitorv1alpha1.KSMScale{}). // Enable status updates
    Build()
```

**Why this matters**: The fake client simulates a real Kubernetes API server but runs in-memory, making tests fast and isolated.

##### 2. Test Data Creation
```go
// Create test KSMScale resource
ksm := &monitorv1alpha1.KSMScale{
    ObjectMeta: metav1.ObjectMeta{Name: "test-monitor"},
    Spec: monitorv1alpha1.KSMScaleSpec{
        NodesPerMonitor: 15,
        MonitorImage:    "test-image:latest",
        Namespace:       "monitoring",
    },
}

// Create test nodes
for i := 0; i < 10; i++ {
    nodes = append(nodes, &corev1.Node{
        ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("node-%d", i)},
    })
}
```

##### 3. Reconciliation Testing Pattern
```go
// 1. Setup reconciler with fake client
reconciler := &KSMScaleReconciler{
    Client: fakeClient,
    Scheme: scheme.Scheme,
}

// 2. Create reconcile request
req := reconcile.Request{
    NamespacedName: types.NamespacedName{Name: ksm.Name},
}

// 3. Execute reconciliation
_, err := reconciler.Reconcile(ctx, req)
Expect(err).NotTo(HaveOccurred())

// 4. Verify results - Check actual pods created
podList := &corev1.PodList{}
err = fakeClient.List(ctx, podList, client.InNamespace("monitoring"))
Expect(err).NotTo(HaveOccurred())
Expect(podList.Items).To(HaveLen(1)) // Expected pod count
```

##### 4. Status Verification Patterns
```go
// Get updated resource to check status
updatedSM := &monitorv1alpha1.KSMScale{}
err = fakeClient.Get(ctx, types.NamespacedName{Name: ksm.Name}, updatedSM)
Expect(err).NotTo(HaveOccurred())

// Verify status fields
Expect(updatedSM.Status.NodeCount).To(Equal(int32(10)))
Expect(updatedSM.Status.DesiredReplicas).To(Equal(int32(1)))
// CurrentReplicas shows count BEFORE creation/deletion
Expect(updatedSM.Status.CurrentReplicas).To(Equal(int32(0)))
Expect(updatedSM.Status.Phase).To(Equal("Scaling"))
```

#### Critical Test Behavior Understanding

##### 1. Reconciliation Timing
```go
// IMPORTANT: Controller behavior during reconciliation
// 1. Controller counts existing pods → currentReplicas = 0
// 2. Controller creates missing pods → 1 pod created
// 3. Controller updates status with OLD count → CurrentReplicas = 0
// 4. On NEXT reconciliation → currentReplicas = 1, Phase = "Ready"
```

**Why this matters**: Tests must expect the "before" state in status, not the "after" state.

##### 2. Phase Logic
```go
// Non-sharded mode phases:
if currentReplicas == desiredReplicas {
    ksmScale.Status.Phase = "Ready"
} else {
    ksmScale.Status.Phase = "Scaling"
}

// Sharded mode phases:
if activeShards < shardCount {
    ksmScale.Status.Phase = "Incomplete"  // No ready pods in some shards
} else if len(existingPods) != desiredPods {
    ksmScale.Status.Phase = "Scaling"     // Wrong pod count
} else {
    ksmScale.Status.Phase = "Ready"       // All good
}
```

##### 3. Sharding Status Testing
```go
// Verify sharding configuration
for i, pod := range podList.Items {
    Expect(pod.Labels["shard-id"]).To(Equal(fmt.Sprintf("%d", i%shardCount)))
    Expect(pod.Labels["total-shards"]).To(Equal("3"))
    Expect(pod.Labels["sharding-enabled"]).To(Equal("true"))
    
    // Check container args include sharding parameters
    args := pod.Spec.Containers[0].Args
    Expect(args).To(ContainElement(fmt.Sprintf("--shard=%d", i%shardCount)))
    Expect(args).To(ContainElement("--total-shards=3"))
}
```

#### Test Execution Commands

```bash
# Run all controller tests
make test

# Run tests with verbose output
go test ./controllers/... -v

# Run specific test
go test ./controllers/... -v -ginkgo.focus "should create 1 pod"

# Run tests with coverage
make test-coverage

# Test results:
# ✅ 18 tests passing
# ✅ 60.6% code coverage
# ✅ No flaky tests
```

#### Test Debugging Techniques

##### 1. Debug Test Data
```go
// Add debug output in tests
fmt.Printf("Pods created: %d\n", len(podList.Items))
for _, pod := range podList.Items {
    fmt.Printf("  Pod: %s/%s\n", pod.Namespace, pod.Name)
}
fmt.Printf("Status: %+v\n", updatedSM.Status)
```

##### 2. Test Individual Components
```go
// Test shard distribution calculation separately
It("should handle shard distribution calculation correctly", func() {
    reconciler := &KSMScaleReconciler{}
    
    assignments := reconciler.calculateShardDistribution(5, 3)
    Expect(assignments[0]).To(Equal(int32(0))) // Pod 0 → Shard 0
    Expect(assignments[1]).To(Equal(int32(1))) // Pod 1 → Shard 1
    Expect(assignments[2]).To(Equal(int32(2))) // Pod 2 → Shard 2
    Expect(assignments[3]).To(Equal(int32(0))) // Pod 3 → Shard 0
    Expect(assignments[4]).To(Equal(int32(1))) // Pod 4 → Shard 1
})
```

#### Common Test Pitfalls and Solutions

##### 1. **Pitfall**: Expecting immediate status reflection
```go
// ❌ Wrong - expects status to show result immediately
Expect(updatedSM.Status.CurrentReplicas).To(Equal(int32(1)))

// ✅ Correct - status shows pre-reconciliation state
Expect(updatedSM.Status.CurrentReplicas).To(Equal(int32(0)))
```

##### 2. **Pitfall**: Forgetting status subresource
```go
// ❌ Wrong - status updates will fail silently
fakeClient := fake.NewClientBuilder().
    WithRuntimeObjects(objs...).
    Build()

// ✅ Correct - enables status updates
fakeClient := fake.NewClientBuilder().
    WithRuntimeObjects(objs...).
    WithStatusSubresource(&monitorv1alpha1.KSMScale{}).
    Build()
```

##### 3. **Pitfall**: Wrong phase expectations
```go
// ❌ Wrong for sharded mode with no pods
Expect(updatedSM.Status.Phase).To(Equal("Scaling"))

// ✅ Correct - no active shards = Incomplete
Expect(updatedSM.Status.Phase).To(Equal("Incomplete"))
```

#### Test Maintenance Best Practices

1. **Keep tests deterministic**: Use fixed test data, avoid randomness
2. **Test edge cases**: Zero nodes, single node, large clusters
3. **Test error conditions**: Missing namespaces, invalid configs
4. **Maintain test data helpers**: Centralize test object creation
5. **Document test intentions**: Clear test names and comments

#### Integration with CI/CD

The test suite integrates with the development workflow:

```makefile
# Makefile test targets
test:           # Run unit tests with coverage
test-coverage:  # Generate HTML coverage report  
test-e2e:       # Run end-to-end tests (when available)
test-all:       # Run all test suites
```

This comprehensive test suite ensures the operator works correctly across all scenarios before deployment.

---

## Learning Plan

### Phase 1: Foundation (Week 1-2)
1. **Go Basics** (if not familiar)
   - Complete [Tour of Go](https://tour.golang.org/)
   - Focus on: structs, interfaces, pointers, error handling
   
2. **Kubernetes Basics**
   - Understand: Pods, Services, Deployments, Namespaces
   - Practice: `kubectl` commands
   - Read: [Kubernetes Concepts](https://kubernetes.io/docs/concepts/)

3. **Project Setup**
   - Clone this repository
   - Run `make dev` to see it working
   - Explore the generated CRDs and RBAC

### Phase 2: Understanding (Week 3-4)
1. **Read Through Code**
   - Start with `api/v1alpha1/ksmscale_types.go`
   - Understand the data structures
   - Map structs to YAML examples

2. **Trace Reconciliation**
   - Put print statements in `Reconcile` function
   - Watch logs with `make logs-kind`
   - Create/modify KSMScale resources and observe

3. **Experiment with Changes**
   - Add a new field to KSMScaleSpec
   - Regenerate CRDs
   - Test the change

### Phase 3: Deep Dive (Week 5-6)
1. **Controller Patterns**
   - Study other operators: [Operator Hub](https://operatorhub.io/)
   - Read: [Programming Kubernetes](https://www.oreilly.com/library/view/programming-kubernetes/9781492047094/)

2. **Testing**
   - Add new test cases
   - Understand Ginkgo/Gomega patterns
   - Write integration tests

3. **Advanced Features**
   - Add validation webhooks
   - Implement finalizers
   - Add metrics collection

### Phase 4: Production (Week 7-8)
1. **Deployment**
   - Deploy to real cluster
   - Set up monitoring
   - Configure alerting

2. **Optimization**
   - Profile performance
   - Optimize reconciliation frequency
   - Handle edge cases

### Learning Resources

#### Books
- **"Programming Kubernetes"** by Michael Hausenblas - Comprehensive operator guide
- **"The Go Programming Language"** by Donovan & Kernighan - Go fundamentals
- **"Kubernetes Up & Running"** by Kelsey Hightower - Kubernetes basics

#### Documentation
- [Controller Runtime Book](https://book.kubebuilder.io/) - Official framework docs
- [Kubernetes API Reference](https://kubernetes.io/docs/reference/kubernetes-api/)
- [Go Documentation](https://golang.org/doc/)

#### Example Projects
- [kubebuilder samples](https://github.com/kubernetes-sigs/kubebuilder/tree/master/testdata)
- [controller-runtime examples](https://github.com/kubernetes-sigs/controller-runtime/tree/main/examples)

---

## Common Patterns and Best Practices

### 1. Error Handling Patterns
```go
// ❌ Bad: Ignoring errors
pods, _ := r.getCurrentMonitorPods(ctx, ksmScale, namespace)

// ✅ Good: Always handle errors
pods, err := r.getCurrentMonitorPods(ctx, ksmScale, namespace)
if err != nil {
    logger.Error(err, "Failed to get current monitor pods")
    return ctrl.Result{}, err
}
```

### 2. Logging Patterns
```go
// ✅ Good: Structured logging with context
logger.Info("Reconciling KSMScale",
    "nodeCount", nodeCount,
    "desiredReplicas", desiredReplicas,
    "currentReplicas", currentReplicas)

// ✅ Good: Error logging with details
logger.Error(err, "Failed to create monitor pod", 
    "podName", podName, 
    "namespace", namespace)
```

### 3. Resource Management Patterns
```go
// ✅ Good: Set owner references for cleanup
if err := controllerutil.SetControllerReference(ksmScale, pod, r.Scheme); err != nil {
    return err
}

// ✅ Good: Handle already exists gracefully
if err := r.Create(ctx, pod); err != nil {
    if !errors.IsAlreadyExists(err) {
        return err
    }
}
```

### 4. Status Update Patterns
```go
// ✅ Good: Update status after main logic
defer func() {
    if statusErr := r.Status().Update(ctx, ksmScale); statusErr != nil {
        logger.Error(statusErr, "Failed to update status")
        // Don't override the main error if there is one
        if err == nil {
            err = statusErr
        }
    }
}()
```

### 5. Reconciliation Patterns
```go
// ✅ Good: Return appropriate requeue behavior
if currentReplicas != desiredReplicas {
    // Scaling in progress, check back soon
    return ctrl.Result{RequeueAfter: time.Second * 30}, nil
}

// Normal operation, check back periodically
return ctrl.Result{RequeueAfter: time.Minute}, nil
```

---

## Troubleshooting Guide

### Common Issues and Solutions

#### 1. "no matches for kind KSMScale"
**Problem:** CRD not installed
**Solution:**
```bash
make install-crd
kubectl get crd ksmscales.monitor.example.com
```

#### 2. "pods is forbidden"
**Problem:** Insufficient RBAC permissions
**Solution:**
```bash
# Check current permissions
kubectl auth can-i create pods --as=system:serviceaccount:monitoring:ksm-scale-operator

# Update RBAC
make deploy
```

#### 3. "ImagePullBackOff"
**Problem:** Container image not available
**Solution:**
```bash
# For development with kind
make kind-load

# For production
docker pull registry.k8s.io/kube-state-metrics/kube-state-metrics:v2.10.1
```

#### 4. Controller not reconciling
**Problem:** Watch not set up correctly
**Solution:**
```bash
# Check controller setup
func (r *KSMScaleReconciler) SetupWithManager(mgr ctrl.Manager) error {
    return ctrl.NewControllerManagedBy(mgr).
        For(&monitorv1alpha1.KSMScale{}).  // Primary resource
        Owns(&corev1.Pod{}).                   // Secondary resources
        Complete(r)
}
```

#### 5. "DeepCopy method not found"
**Problem:** Missing generated code
**Solution:**
```bash
go generate ./...
```

### Debugging Techniques

#### 1. Add Debug Logging
```go
logger.V(1).Info("Debug info", "variable", value)
```

#### 2. Use kubectl for Investigation
```bash
# Check resource status
kubectl describe ksmscale my-monitor

# Check operator logs
kubectl logs -n monitoring deployment/ksm-scale-operator -f

# Check events
kubectl get events -n monitoring --sort-by='.lastTimestamp'
```

#### 3. Use Delve Debugger
```bash
# Install delve
go install github.com/go-delve/delve/cmd/dlv@latest

# Debug locally
dlv debug cmd/manager/main.go
```

---

## Next Steps

### Immediate Improvements
1. **Add Webhooks**
   - Validation: Ensure nodesPerMonitor > 0
   - Mutation: Set default values
   - Deletion: Prevent deletion if pods exist

2. **Add Metrics**
   - Reconciliation duration
   - Number of managed pods
   - Error rates

3. **Improve Observability**
   - Add more detailed status conditions
   - Add events for important actions
   - Improve error messages

### Advanced Features
1. **Multi-tenancy**
   - Support multiple KSMScale resources
   - Namespace isolation
   - Resource quotas

2. **Autoscaling Integration**
   - Watch for cluster autoscaler events
   - Predictive scaling based on trends
   - Integration with VPA (Vertical Pod Autoscaler)

3. **High Availability**
   - Leader election
   - Graceful shutdown
   - Rolling updates

### Production Considerations
1. **Security**
   - Pod security standards
   - Network policies
   - Service mesh integration

2. **Reliability**
   - Circuit breakers
   - Retry logic with backoff
   - Dead letter queues

3. **Operations**
   - Monitoring dashboards
   - Alerting rules
   - Runbooks

---

## Conclusion

This operator demonstrates core Kubernetes operator patterns:
- **Custom Resource Definitions** for configuration
- **Controllers** for business logic
- **Reconciliation loops** for maintaining desired state
- **Owner references** for resource lifecycle management

Understanding this operator provides a solid foundation for building more complex operators and understanding the broader Kubernetes ecosystem.

The key insight is that operators are about **encoding operational knowledge** into software that can run continuously, making systems self-healing and self-managing.

---

## Implementation Details

### Current Implementation Summary

This section provides specific details about the actual KSM Scale Operator implementation based on the code review.

#### Project Naming Consistency
- **CRD Kind**: `KSMScale`
- **API Group**: `monitor.example.com`
- **Controller**: `KSMScaleReconciler`
- **Module**: `github.com/example/ksm-scale-operator`

#### Key Implementation Files
1. **`api/v1alpha1/ksmscale_types.go`**
   - Defines the KSMScale CRD structure
   - Contains Spec and Status definitions
   - Includes resource requirements for pods
   - Has kubebuilder annotations for CRD generation

2. **`controllers/ksmscale_controller.go`**
   - Main reconciliation logic
   - Pod creation and deletion logic
   - Resource management with owner references
   - Label-based pod selection using `ksmscale.example.com` label

3. **`cmd/manager/main.go`**
   - Entry point for the operator
   - Sets up the manager with health checks
   - Configures leader election
   - Initializes the KSMScaleReconciler

#### Scaling Algorithm
The operator uses a simple but effective scaling algorithm:

```go
desiredReplicas := int(math.Ceil(float64(nodeCount) / float64(nodesPerMonitor)))
```

- **Default ratio**: 15 nodes per kube-state-metrics pod
- **Minimum pods**: Always at least 1 pod if nodes exist
- **Pod naming**: `kube-state-metrics-{ksmscale-name}-{index}`
- **Index tracking**: Each pod has an index label for ordered scaling

#### Pod Selection Strategy
The controller tracks pods using two key labels:
- `app.kubernetes.io/name: kube-state-metrics`
- `ksmscale.example.com: {resource-name}`

This ensures proper ownership and prevents conflicts with manually created pods.

#### Resource Management Features
1. **Custom Resource Requirements**:
   - Supports CPU/memory requests and limits
   - Configurable through the CRD spec
   - Applied to created pods dynamically

2. **Security Context**:
   - Runs as non-root user (65534)
   - Read-only root filesystem
   - Drops all capabilities
   - No privilege escalation

3. **Health Checks**:
   - Liveness probe: `/healthz` on port 8080
   - Readiness probe: `/` on port 8080
   - 5-second initial delay and timeout

#### Status Tracking
The operator tracks and reports:
- **NodeCount**: Current cluster node count
- **DesiredReplicas**: Calculated pod count needed
- **CurrentReplicas**: Actual running pod count
- **Phase**: "Ready" or "Scaling"
- **LastUpdated**: Timestamp of last reconciliation

#### Reconciliation Behavior
- **Requeue interval**: 1 minute for regular checks
- **Error handling**: Returns errors to controller-runtime for retry
- **Idempotency**: Uses `IsAlreadyExists` check for pod creation

#### Testing Approach
- Uses Ginkgo/Gomega BDD framework
- Fake client for unit testing
- Tests scaling scenarios (1-15, 16-30 nodes, etc.)
- Validates pod creation with correct labels

#### Configuration Files
1. **CRD**: `config/crd/bases/monitor.example.com_ksmscales.yaml`
   - Auto-generated from Go types
   - Includes validation and default values
   - Custom printer columns for kubectl output

2. **RBAC**: Minimal permissions
   - Read nodes cluster-wide
   - Manage pods in specified namespace
   - Update KSMScale status

3. **Sample manifests**: Two examples provided
   - `sample-ksmscale.yaml`: Production configuration
   - `sample-ksmscale-dev.yaml`: Development configuration

#### Known Issues and Considerations

1. **Status Update Bug**: 
   - Line 118 in controller sets `CurrentReplicas` to `desiredReplicas` instead of actual count
   - Should be fixed to properly reflect scaling state

2. **ServiceAccount Requirement**:
   - Pods expect `kube-state-metrics` ServiceAccount to exist
   - Must be created separately or pods will fail

3. **Namespace Handling**:
   - Default namespace is "monitoring"
   - Can be overridden per KSMScale resource
   - Operator needs permissions in target namespace

4. **Pod Deletion Strategy**:
   - Currently deletes pods with highest indexes first
   - No graceful shutdown period configuration
   - Relies on Kubernetes default termination grace period

#### Development Commands

```bash
# Generate CRDs and RBAC
make manifests

# Run locally against cluster
make run

# Build and push image
make docker-build docker-push IMG=<registry>/ksm-scale-operator:tag

# Deploy to cluster
make deploy

# Run tests
make test

# Local development with kind
make dev
make logs-kind
```

#### Future Improvements Suggested

1. **Enhanced Scaling Logic**:
   - Support for node selectors/affinity
   - Weighted node counting (by resources)
   - Multi-dimension scaling (nodes + metrics volume)

2. **Operational Improvements**:
   - Graceful pod rotation during updates
   - Support for PodDisruptionBudgets
   - Metrics exposure for monitoring

3. **Configuration Enhancements**:
   - Support for different images per pod
   - ConfigMap/Secret mounting
   - Environment variable injection

4. **Observability**:
   - Prometheus metrics for operator performance
   - Event generation for important state changes
   - Structured logging with trace IDs

---

## Sharding Implementation (NEW FEATURE)

### Overview

The KSM Scale Operator now supports **sharding with dynamic replicas**, combining the benefits of:
- **Fixed sharding** for predictable data distribution
- **Dynamic scaling** based on cluster size
- **Complete metric coverage** at all scales

### How Sharding Works

When sharding is enabled, the operator:
1. Maintains a **fixed number of shards** (e.g., 3)
2. **Dynamically scales pods** based on cluster size
3. **Distributes pods across shards** using round-robin
4. **Ensures minimum coverage** (at least 1 pod per shard)

### Sharding vs Non-Sharded Modes

#### Non-Sharded Mode (Default)
- **Use case**: Small to medium clusters (<60 nodes)
- **Behavior**: Dynamic scaling, 1 pod per N nodes
- **Pros**: Simple, resource-efficient
- **Cons**: Single point of failure, performance limits

#### Sharded Mode 
- **Use case**: Medium to large clusters (60+ nodes)
- **Behavior**: Fixed shards + dynamic replicas
- **Pros**: Scalable, resilient, load distributed
- **Cons**: Minimum resource overhead, complexity

### Configuration Examples

#### Basic Sharding Configuration
```yaml
apiVersion: monitor.example.com/v1alpha1
kind: KSMScale
metadata:
  name: medium-cluster
spec:
  nodesPerMonitor: 15
  namespace: monitoring
  
  sharding:
    enabled: true
    shardCount: 3           # 3 fixed shards
    minReplicasPerShard: 1  # Minimum 3 pods total
    maxReplicasPerShard: 4  # Maximum 12 pods total
    
    distribution:
      spreadAcrossNodes: true
      enablePDB: true
      maxUnavailable: 1
```

#### Scaling Behavior
| Nodes | Calculated Pods | Actual Pods | Distribution |
|-------|----------------|-------------|---------------|
| 15    | 1              | 3 (min)     | Shard 0: 1, Shard 1: 1, Shard 2: 1 |
| 45    | 3              | 3           | Shard 0: 1, Shard 1: 1, Shard 2: 1 |
| 75    | 5              | 5           | Shard 0: 2, Shard 1: 2, Shard 2: 1 |
| 90    | 6              | 6           | Shard 0: 2, Shard 1: 2, Shard 2: 2 |

### Technical Implementation

#### CRD Structure
```go
type KSMScaleSpec struct {
    // Existing fields
    NodesPerMonitor int32                `json:"nodesPerMonitor,omitempty"`
    MonitorImage    string               `json:"monitorImage,omitempty"`
    Namespace       string               `json:"namespace,omitempty"`
    Resources       ResourceRequirements `json:"resources,omitempty"`
    
    // New: Sharding configuration
    Sharding *ShardingConfig `json:"sharding,omitempty"`
}

type ShardingConfig struct {
    Enabled             bool                 `json:"enabled"`
    ShardCount          int32                `json:"shardCount"`          // 2-10 shards
    MinReplicasPerShard int32                `json:"minReplicasPerShard"` // Min pods per shard
    MaxReplicasPerShard int32                `json:"maxReplicasPerShard"` // Max pods per shard
    Distribution        *DistributionConfig  `json:"distribution,omitempty"`
}

type DistributionConfig struct {
    SpreadAcrossNodes bool  `json:"spreadAcrossNodes,omitempty"` // Anti-affinity
    EnablePDB         bool  `json:"enablePDB,omitempty"`         // PodDisruptionBudget
    MaxUnavailable    int32 `json:"maxUnavailable,omitempty"`    // Max unavailable shards
}
```

#### Pod Labels and Configuration
Each sharded pod gets these labels:
```yaml
labels:
  app.kubernetes.io/name: kube-state-metrics
  ksmscale.example.com: <resource-name>
  pod-index: "0"           # Sequential pod index (0, 1, 2, 3...)
  shard-id: "0"            # Assigned shard (0, 1, 2...)
  shard-replica: "0"       # Replica number within shard (0, 1, 2...)
  total-shards: "3"        # Total shard count
  sharding-enabled: "true" # Distinguishes from non-sharded pods
```

**Label Explanation:**

- **pod-index**: Global sequential index (0 to N-1) for all pods
- **shard-id**: Which shard this pod serves (calculated as `pod-index % shard-count`)
- **shard-replica**: Which replica number within the shard (`pod-index / shard-count`)
- **total-shards**: Total number of shards configured

**Example with 7 pods, 3 shards:**
| Pod Index | Shard ID | Shard Replica | Meaning |
|-----------|----------|---------------|---------|
| 0 | 0 | 0 | First replica in shard 0 |
| 1 | 1 | 0 | First replica in shard 1 |
| 2 | 2 | 0 | First replica in shard 2 |
| 3 | 0 | 1 | Second replica in shard 0 |
| 4 | 1 | 1 | Second replica in shard 1 |
| 5 | 2 | 1 | Second replica in shard 2 |
| 6 | 0 | 2 | Third replica in shard 0 |

**Note**: `shard-replica` is a **custom label specific to this operator**, not part of the official kube-state-metrics specification.

Container arguments include:
```bash
--shard=0
--total-shards=3
```

#### Round-Robin Distribution Algorithm
```go
// Pod assignment: podIndex % shardCount
func (r *KSMScaleReconciler) calculateShardDistribution(totalPods int32, shardCount int32) map[int32]int32 {
    assignments := make(map[int32]int32)
    for podIndex := int32(0); podIndex < totalPods; podIndex++ {
        assignments[podIndex] = podIndex % shardCount
    }
    return assignments
}
```

Example distribution:
- Pod 0 → Shard 0
- Pod 1 → Shard 1  
- Pod 2 → Shard 2
- Pod 3 → Shard 0 (second replica)
- Pod 4 → Shard 1 (second replica)

### Controller Logic

#### Reconciliation Flow
1. **Check mode**: Sharded vs non-sharded
2. **Calculate desired pods**: Based on nodes and minimum requirements
3. **Get current pods**: Filter by sharding labels
4. **Build shard assignments**: Round-robin distribution
5. **Reconcile pods**: Create/update/delete as needed
6. **Update PDB**: If enabled
7. **Update status**: With detailed shard information

#### Key Functions
- `reconcileShardedMode()`: Main sharding reconciliation
- `calculateShardDistribution()`: Round-robin assignment
- `createShardedPod()`: Pod creation with shard config
- `isShardConfigValid()`: Validates pod shard configuration
- `updateShardingStatus()`: Status updates with shard details

### Status and Monitoring

#### Enhanced Status
```go
type KSMScaleStatus struct {
    // Existing fields
    CurrentReplicas int32
    DesiredReplicas int32
    NodeCount       int32
    Phase           string  // "Ready", "Scaling", "Incomplete"
    
    // New: Sharding status
    ShardingStatus *ShardingStatus `json:"shardingStatus,omitempty"`
}

type ShardingStatus struct {
    TotalShards       int32                    `json:"totalShards"`
    ActiveShards      int32                    `json:"activeShards"`
    ShardDistribution map[string]ShardReplicas `json:"shardDistribution"`
    Warning           string                   `json:"warning,omitempty"`
}
```

#### Example Status Output
```yaml
status:
  currentReplicas: 5
  desiredReplicas: 5
  nodeCount: 75
  phase: Ready
  shardingStatus:
    totalShards: 3
    activeShards: 3
    shardDistribution:
      "0":
        replicas: 2
        ready: 2
        pods: ["kube-state-metrics-example-0", "kube-state-metrics-example-3"]
      "1":
        replicas: 2
        ready: 2
        pods: ["kube-state-metrics-example-1", "kube-state-metrics-example-4"]
      "2":
        replicas: 1
        ready: 1
        pods: ["kube-state-metrics-example-2"]
    warning: ""
```

#### Efficiency Warnings
The operator provides recommendations:
```
Over-sharded: 5 shards for 30 nodes (6.0 nodes/shard). Consider reducing to 2 shards.
Under-sharded: 2 shards for 200 nodes (100.0 nodes/shard). Consider increasing to 5 shards.
```

### Prometheus Integration

#### Handling Duplicate Metrics
With multiple pods per shard, Prometheus receives duplicate metrics. Solutions:

1. **Recording Rules (Recommended)**:
```yaml
groups:
- name: ksm_dedup
  rules:
  - record: kube_pod_info_dedup
    expr: max by (namespace, pod, uid, node) (kube_pod_info)
```

2. **ServiceMonitor Configuration**:
```yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
spec:
  endpoints:
  - port: http-metrics
    relabelings:
    - sourceLabels: [__meta_kubernetes_pod_label_shard_id]
      targetLabel: shard
```

### Operational Considerations

#### Resource Requirements
- **Small clusters**: 3 pods minimum (instead of 1)
- **Medium clusters**: Optimal resource usage
- **Large clusters**: Better than single instance

#### High Availability Features

**1. Multiple Replicas Per Shard**
```yaml
# Example: 3 shards, 6 pods total
Pod-0: shard-id=0, shard-replica=0  # First replica for shard 0
Pod-1: shard-id=1, shard-replica=0  # First replica for shard 1
Pod-2: shard-id=2, shard-replica=0  # First replica for shard 2
Pod-3: shard-id=0, shard-replica=1  # Second replica for shard 0
Pod-4: shard-id=1, shard-replica=1  # Second replica for shard 1
Pod-5: shard-id=2, shard-replica=1  # Second replica for shard 2
```

**Benefit**: If any pod fails, other replicas in the same shard continue serving metrics.

**2. PodDisruptionBudget Protection**
```yaml
# Automatically created PDB
apiVersion: policy/v1
kind: PodDisruptionBudget
metadata:
  name: ksm-<resource-name>-shards
spec:
  maxUnavailable: 1  # Only 1 pod can be disrupted at once
  selector:
    matchLabels:
      ksmscale.example.com: <resource-name>
      sharding-enabled: "true"
```

**3. Pod Anti-Affinity (Automatic)**
```yaml
# Applied when spreadAcrossNodes: true
affinity:
  podAntiAffinity:
    preferredDuringSchedulingIgnoredDuringExecution:
    - weight: 100
      podAffinityTerm:
        labelSelector:
          matchLabels:
            ksmscale.example.com: <resource-name>
            shard-id: "0"  # Prevents same-shard pods on same node
        topologyKey: kubernetes.io/hostname
```

**4. Health Probes (Built-in)**
```yaml
# Automatic liveness and readiness probes
livenessProbe:
  httpGet: {path: /healthz, port: 8080}
  initialDelaySeconds: 5
  timeoutSeconds: 5

readinessProbe:
  httpGet: {path: /, port: 8080}
  initialDelaySeconds: 5
  timeoutSeconds: 5
```

**5. Automatic Pod Recreation**
- Controller watches pod events and immediately recreates failed pods
- Reconciliation every 2 minutes as backup
- Zero-downtime rolling updates via PDB protection

#### Migration Path
1. **From non-sharded**: Add sharding config, pods recreated
2. **Rollback**: Remove sharding config, returns to dynamic mode
3. **Zero-downtime**: Use `maxUnavailable: 0` during migration

### RBAC Requirements

Sharding requires additional RBAC permissions for PodDisruptionBudgets:

```yaml
# ClusterRole additions
- apiGroups: ["policy"]
  resources: ["poddisruptionbudgets"]
  verbs: ["get", "list", "watch"]

# Role additions (namespaced)
- apiGroups: ["policy"]
  resources: ["poddisruptionbudgets"]
  verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
```

**Note**: Ensure these permissions are present in `config/rbac/role.yaml` before deploying sharded configurations.

### Security Features

All pods include enhanced security context:
```yaml
securityContext:
  allowPrivilegeEscalation: false
  capabilities:
    drop: [ALL]
  readOnlyRootFilesystem: true
  runAsNonRoot: true
  runAsUser: 65534  # nobody user
```

### Testing

#### Unit Tests
- Shard distribution calculation
- Minimum pod enforcement
- Round-robin assignment
- PDB creation
- Status updates
- Security context validation

#### Integration Tests
```bash
# Run sharding-specific tests
go test ./controllers -v -run "sharded"

# Test all scenarios
make test

# Development environment testing
make dev                    # Deploy with fresh build
kubectl apply -f test-sharded.yaml  # Apply sharded config
kubectl get ksmscales -o wide       # Verify scaling behavior
```

#### Live Testing Results
From actual Kind cluster testing:
```bash
# Non-sharded: 4 nodes → 1 pod (normal scaling)
kubectl get ksmscale dev-ksm -o wide
# NAME      NODES   DESIRED   CURRENT   PHASE   AGE
# dev-ksm   4       1         1         Ready   2m

# Sharded: 4 nodes → 3 pods (minimum enforced)
kubectl get ksmscale test-sharded-ksm -o wide  
# NAME               NODES   DESIRED   CURRENT   PHASE   AGE
# test-sharded-ksm   4       3         3         Ready   2m
```

### Best Practices

#### When to Use Sharding
- **Use sharding**: Clusters >60 nodes, need reliability
- **Use dynamic**: Clusters <60 nodes, cost-sensitive

#### Shard Count Recommendations
- **2 shards**: 30-60 nodes
- **3 shards**: 60-120 nodes
- **5 shards**: 120-200 nodes
- **7-10 shards**: 200+ nodes

#### Configuration Tips
1. **Start conservative**: Begin with fewer shards
2. **Monitor warnings**: Check status for recommendations
3. **Use PDB**: Always enable for production
4. **Plan resources**: Factor 3x minimum for small clusters

### Troubleshooting

#### Common Issues

1. **Incomplete Phase**:
```bash
# Check which shards are missing
kubectl get ksmscale <name> -o jsonpath='{.status.shardingStatus.shardDistribution}'

# Look for pods stuck in creation
kubectl get pods -l sharding-enabled=true
```

2. **Over/Under-sharded Warnings**:
```bash
# Check efficiency warning
kubectl get ksmscale <name> -o jsonpath='{.status.shardingStatus.warning}'

# Adjust shard count based on recommendation
```

3. **PDB Issues**:
```bash
# Check PDB status
kubectl get pdb ksm-<name>-shards

# Verify selector matches pods
kubectl get pods -l ksmscale.example.com=<name>,sharding-enabled=true
```

4. **RBAC Permission Errors**:
```bash
# Common error in operator logs:
# "failed to list *v1.PodDisruptionBudget: poddisruptionbudgets.policy is forbidden"

# Solution: Apply updated RBAC
kubectl apply -f config/rbac/role.yaml
kubectl rollout restart deployment/ksm-scale-operator -n monitoring
```

5. **CRD Field Rejection**:
```bash
# Error: "unknown field spec.sharding"
# Solution: Apply updated CRD
kubectl apply -f config/crd/bases/monitor.example.com_ksmscales.yaml
```

#### Debug Commands
```bash
# View shard distribution with all labels
kubectl get pods -l sharding-enabled=true -o custom-columns=\
NAME:.metadata.name,SHARD:.metadata.labels.shard-id,REPLICA:.metadata.labels.shard-replica,NODE:.spec.nodeName

# Check pod anti-affinity working
kubectl get pods -l sharding-enabled=true -o wide

# Validate pod arguments
kubectl get pod <pod-name> -o yaml | grep -A 5 "args:"

# Monitor reconciliation with context
kubectl logs deployment/ksm-scale-operator -f | grep -E "(sharded|shard|PDB)"

# Test pod health endpoints (from outside cluster)
kubectl port-forward pod/<pod-name> 8080:8080 &
curl http://localhost:8080/healthz
curl http://localhost:8080/
```

#### Performance Validation
```bash
# Verify each shard serves different data
for pod in $(kubectl get pods -l sharding-enabled=true -o name); do
  echo "=== $pod ==="
  kubectl port-forward $pod 8080:8080 &
  sleep 2
  curl -s http://localhost:8080/metrics | grep kube_pod_info | wc -l
  kill %1
done
```

This sharding implementation provides enterprise-grade scalability while maintaining operational simplicity and backward compatibility.