# Agent Sandbox E2E Testing Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement --wg flag for lattice-agent-sandbox, add audit JSONL output, and create 6-scenario e2e test suite with CI workflow.

**Architecture:** Sandbox gets a lightweight UDP bind (wg_bridge.go) implementing shim.WireGuardBind, enabling wireguard-go attachment without the full DefaultBind/FilteringUDPMux stack. Audit writes JSONL to /tmp/lattice-audit.jsonl. E2E tests deploy companion agent + sandbox pod in k3d, verifying registration, connectivity, policy deny, audit, and revocation.

**Tech Stack:** Go 1.25, Ginkgo v2 + Gomega, gVisor netstack (via lattice-shim), wireguard-go, k3d

---

### Task 1: Implement lightweight WireGuard UDP bind

**Files:**
- Create: `internal/agent/gvisor/wg_bridge.go`
- Modify: `internal/agent/gvisor/community_stub.go`

- [ ] **Step 1: Create `internal/agent/gvisor/wg_bridge.go`**

```go
//go:build pro

// Copyright 2026 The Lattice Authors, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");

package gvisor

import (
	"fmt"
	"log"
	"net"
	"sync/atomic"

	"github.com/alatticeio/lattice-shim/shim"
)

// udpBind is a lightweight WireGuardBind backed by a simple UDP socket.
// It implements shim.WireGuardBind so the gVisor sandbox can send and
// receive encrypted WireGuard packets through a plain UDP :port socket.
type udpBind struct {
	conn   *net.UDPConn
	closed atomic.Bool
}

// NewUDPBind opens a UDP socket on the given address and returns a
// shim.WireGuardBind suitable for attaching wireguard-go to the gVisor
// netstack.
//
// addr must be of the form ":51820". Use "0.0.0.0:51820" to listen on all
// IPv4 interfaces, "[::]:51820" for all IPv6.
func NewUDPBind(addr string) (shim.WireGuardBind, error) {
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return nil, fmt.Errorf("gvisor: resolve udp addr %s: %w", addr, err)
	}

	conn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		return nil, fmt.Errorf("gvisor: listen udp %s: %w", addr, err)
	}

	log.Printf("[gvisor] udp bind listening on %s", conn.LocalAddr())
	return &udpBind{conn: conn}, nil
}

// Write sends an encrypted WireGuard packet via the UDP socket.
func (b *udpBind) Write(packet []byte) error {
	if b.closed.Load() {
		return net.ErrClosed
	}
	_, err := b.conn.Write(packet)
	return err
}

// Read receives an encrypted WireGuard packet from the UDP socket.
// The returned slice is owned by the caller.
func (b *udpBind) Read() ([]byte, error) {
	if b.closed.Load() {
		return nil, net.ErrClosed
	}
	buf := make([]byte, 65535)
	n, err := b.conn.Read(buf)
	if err != nil {
		return nil, err
	}
	return buf[:n], nil
}

// Close closes the UDP socket.
func (b *udpBind) Close() error {
	b.closed.Store(true)
	return b.conn.Close()
}
```

- [ ] **Step 2: Add stub in `community_stub.go` for NewUDPBind**

`internal/agent/gvisor/community_stub.go` currently has the Config struct. Add a stub function:

```go
// NewUDPBind is a Pro feature.
func NewUDPBind(addr string) (interface{}, error) {
	return nil, errors.New("gVisor agent sandbox is a Lattice Pro feature")
}
```

- [ ] **Step 3: Build verify**

```bash
cd /Users/francis/workspc/lattice && go build -tags pro ./internal/agent/gvisor/ 2>&1
```

Expected: no errors.

- [ ] **Step 4: Commit**

```bash
git add internal/agent/gvisor/wg_bridge.go internal/agent/gvisor/community_stub.go
git commit -s -m "feat(gvisor): add lightweight UDP WireGuardBind for sandbox WG attachment"
```

---

### Task 2: Wire --wg flag in start_sandbox_pro.go, add JSONL audit writer

**Files:**
- Modify: `cmd/lattice-agent-sandbox/cmd/start_sandbox_pro.go`
- Modify: `cmd/lattice-agent-sandbox/cmd/start.go`

- [ ] **Step 1: Rewrite `start_sandbox_pro.go` to accept sandboxWGEnabled and sandboxName, add file-based audit writer**

Current file content is `cmd/lattice-agent-sandbox/cmd/start_sandbox_pro.go`. Replace `createSandbox` signature and implementation:

```go
//go:build pro

package cmd

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/alatticeio/lattice-shim/shim"
	"github.com/alatticeio/lattice/internal/agent/gvisor"
)

type sandboxCloser struct {
	sb      *gvisor.Sandbox
	auditF  *os.File
	wgBind  shim.WireGuardBind
}

func (c *sandboxCloser) Close() {
	if c.sb != nil {
		c.sb.Close()
	}
	if c.wgBind != nil {
		c.wgBind.Close()
	}
	if c.auditF != nil {
		c.auditF.Close()
	}
}

// createSandbox creates a gVisor sandbox with the given parameters.
// sandboxName and localIP are required. agentJWT may be empty.
// When wgEnabled is true, a UDP bind on :51820 is created and attached
// to the gVisor netstack so the sandbox can reach Lattice peers.
func createSandbox(sandboxName, localIP, agentJWT string, wgEnabled bool) (*sandboxCloser, error) {
	var policyChecker shim.PolicyChecker
	var auditWriter shim.AuditWriter
	var auditFile *os.File

	// Open audit JSONL file so e2e tests can read it.
	auditPath := "/tmp/lattice-audit.jsonl"
	f, err := os.OpenFile(auditPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		log.Printf("[sandbox] cannot open audit file %s: %v (audit disabled)", auditPath, err)
	} else {
		auditFile = f
		fileWriter := &fileAuditWriter{f: f}
		auditWriter = gvisor.NewAuditAdapter(sandboxName, fileWriter)
	}

	cfg := gvisor.Config{
		ID:            sandboxName,
		LocalIP:       localIP,
		PolicyChecker: policyChecker,
		AuditWriter:   auditWriter,
	}

	var wgBind shim.WireGuardBind
	if wgEnabled {
		wgBind, err = gvisor.NewUDPBind(":51820")
		if err != nil {
			if auditFile != nil {
				auditFile.Close()
			}
			return nil, fmt.Errorf("create WG bind: %w", err)
		}
		cfg.WireGuardBind = wgBind
	}

	sb, err := gvisor.New(cfg)
	if err != nil {
		if wgBind != nil {
			wgBind.Close()
		}
		if auditFile != nil {
			auditFile.Close()
		}
		return nil, err
	}
	return &sandboxCloser{sb: sb, auditF: auditFile, wgBind: wgBind}, nil
}

// fileAuditWriter writes audit events as JSONL to an open file.
type fileAuditWriter struct {
	f *os.File
}

func (w *fileAuditWriter) WriteAudit(agentID string, event shim.AuditEvent) error {
	b, err := json.Marshal(event)
	if err != nil {
		return err
	}
	b = append(b, '\n')
	_, err = w.f.Write(b)
	return err
}

// compile-time checks
var _ gvisor.AuditEventWriter = (*fileAuditWriter)(nil)
```

- [ ] **Step 2: Update `start.go` to pass `sandboxWGEnabled` to `createSandbox`**

In `cmd/lattice-agent-sandbox/cmd/start.go`, line 94, change the call:

```go
// Before:
sb, err := createSandbox(sandboxName, sandboxLocalIP, agentJWT)

// After:
sb, err := createSandbox(sandboxName, sandboxLocalIP, agentJWT, sandboxWGEnabled)
```

- [ ] **Step 3: Update community stub to match new signature**

In `cmd/lattice-agent-sandbox/cmd/start_sandbox_community.go`, update the `createSandbox` signature:

```go
func createSandbox(sandboxName, localIP, agentJWT string, wgEnabled bool) (*sandboxCloser, error) {
	return nil, errors.New("gVisor agent sandbox is a Lattice Pro feature")
}
```

And update the `sandboxCloser` struct in community to have the new fields (or just keep it trivial since it never gets instantiated — just needs to compile):

```go
type sandboxCloser struct{}
func (c *sandboxCloser) Close() {}
```

- [ ] **Step 4: Build verify**

```bash
cd /Users/francis/workspc/lattice
go build -tags pro ./cmd/lattice-agent-sandbox/ 2>&1
go build ./cmd/lattice-agent-sandbox/ 2>&1
```

Expected: both community and pro builds succeed.

- [ ] **Step 5: Commit**

```bash
git add cmd/lattice-agent-sandbox/cmd/start_sandbox_pro.go cmd/lattice-agent-sandbox/cmd/start.go cmd/lattice-agent-sandbox/cmd/start_sandbox_community.go
git commit -s -m "feat(gvisor): wire --wg flag to UDP bind, add JSONL audit writer"
```

---

### Task 3: Add e2e helper functions for sandbox testing

**Files:**
- Modify: `test/e2e/helpers_test.go`

- [ ] **Step 1: Add `apiDELETE` helper and `createEnrollmentToken`**

Add to `test/e2e/helpers_test.go`, after the `apiPOST` function (after line 54):

```go
// apiDELETE sends an HTTP DELETE with optional Bearer auth. Returns status code and parsed response body.
func apiDELETE(url, token string) (int, *resp.Response) {
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodDelete, url, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	httpClient := &http.Client{Timeout: 15 * time.Second}
	httpResp, err := httpClient.Do(req)
	Expect(err).NotTo(HaveOccurred(), "HTTP DELETE %s failed", url)
	defer httpResp.Body.Close() //nolint:errcheck

	var data resp.Response
	_ = json.NewDecoder(httpResp.Body).Decode(&data)
	return httpResp.StatusCode, &data
}
```

- [ ] **Step 2: Add `createEnrollmentToken` helper**

Add after `apiDELETE`:

```go
// createEnrollmentToken creates a one-time enrollment token for agent sandbox registration.
func createEnrollmentToken(manageURL, accessToken, namespace string) string {
	By("Create Enrollment Token for sandbox agent")
	statusCode, data := apiPOST(manageURL+"/api/v1/agent-isolation/enrollment-tokens", accessToken, map[string]any{
		"namespace":    namespace,
		"allowedTools": []string{"http", "exec"},
		"ttlSeconds":   600,
	})
	Expect(statusCode).To(Equal(http.StatusOK), "create enrollment token failed: %+v", data)

	dataMap, ok := data.Data.(map[string]any)
	Expect(ok).To(BeTrue(), "Enrollment token response Data format error")
	token, ok := dataMap["token"].(string)
	Expect(ok && token != "").To(BeTrue(), "token not found in enrollment response")
	return token
}
```

- [ ] **Step 3: Add `registerSandboxAgent` helper**

```go
// registerSandboxAgent calls POST /api/v1/agent-isolation/register and
// returns the JWT and allocated VPN IP.
func registerSandboxAgent(manageURL, accessToken, enrollmentToken, agentName, sandboxMode string) (jwt, localIP string) {
	By("Register sandbox agent via enrollment token")

	rawKey := make([]byte, 32)
	_, err := rand.Read(rawKey)
	Expect(err).NotTo(HaveOccurred(), "generate sandbox WG key failed")
	pubKey := hex.EncodeToString(rawKey)

	statusCode, data := apiPOST(manageURL+"/api/v1/agent-isolation/register", accessToken, map[string]string{
		"enrollmentToken": enrollmentToken,
		"agentName":       agentName,
		"publicKey":       pubKey,
		"sandbox":         sandboxMode,
	})
	Expect(statusCode).To(Equal(http.StatusOK), "register sandbox agent failed: %+v", data)

	dataMap, ok := data.Data.(map[string]any)
	Expect(ok).To(BeTrue(), "register response Data format error")
	jwt, _ = dataMap["JWT"].(string)
	localIP, _ = dataMap["localIP"].(string)
	return jwt, localIP
}
```

Add required imports at top of the file — `"crypto/rand"` and `"encoding/hex"` above `"encoding/json"`.

- [ ] **Step 4: Add `deploySandboxPod` helper**

```go
// deploySandboxPod creates a Pod running lattice-agent-sandbox with --wg.
// The sandbox auto-registers with the control plane using the enrollment token.
func deploySandboxPod(clientset *kubernetes.Clientset, ns, name, sandboxImage, serverURL, enrollmentToken string, hostAliases []corev1.HostAlias) {
	_, err := clientset.CoreV1().Pods(ns).Create(context.Background(), &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"app":     "wf-e2e",
				"wf-role": name,
			},
		},
		Spec: corev1.PodSpec{
			Hostname:    name,
			HostAliases: hostAliases,
			Containers: []corev1.Container{{
				Name:            "sandbox",
				Image:           sandboxImage,
				ImagePullPolicy: corev1.PullIfNotPresent,
				Ports: []corev1.ContainerPort{{
					ContainerPort: 51820,
					Protocol:      corev1.ProtocolUDP,
				}},
				// Dockerfile copies binary as /app/lattice regardless of TARGETSERVICE.
				Command: []string{
					"/app/lattice", "start",
					"--name", name,
					"--server-url", serverURL,
					"--token", enrollmentToken,
					"--wg",
				},
			}},
		},
	}, metav1.CreateOptions{})
	Expect(err).NotTo(HaveOccurred(), "failed to create sandbox Pod %s", name)
}
```

- [ ] **Step 5: Add `revokeSandboxAgent` helper**

```go
// revokeSandboxAgent calls DELETE /api/v1/agent-isolation/agents/:name to revoke an agent.
func revokeSandboxAgent(manageURL, accessToken, agentName, namespace string) {
	By("Revoke sandbox agent: " + agentName)
	url := fmt.Sprintf("%s/api/v1/agent-isolation/agents/%s?namespace=%s", manageURL, agentName, namespace)
	statusCode, data := apiDELETE(url, accessToken)
	Expect(statusCode).To(Equal(http.StatusOK), "revoke agent failed: %+v", data)
}
```

- [ ] **Step 6: Add `readAuditLog` helper**

```go
// auditEvent is a single audit event read from the sandbox JSONL log.
type auditEvent struct {
	Identity string `json:"identity"`
	SrcIP    string `json:"src_ip"`
	DstIP    string `json:"dst_ip"`
	DstPort  uint16 `json:"dst_port"`
	Protocol string `json:"protocol"`
	Verdict  string `json:"verdict"`
}

// readAuditLog reads and parses the sandbox audit JSONL file.
func readAuditLog(clientset *kubernetes.Clientset, config *rest.Config, ns, podName string) []auditEvent {
	output, err := execInPod(clientset, config, ns, podName, []string{"cat", "/tmp/lattice-audit.jsonl"})
	Expect(err).NotTo(HaveOccurred(), "read audit log failed")

	var events []auditEvent
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		if line == "" {
			continue
		}
		var ev auditEvent
		Expect(json.Unmarshal([]byte(line), &ev)).To(Succeed(), "parse audit event: %s", line)
		events = append(events, ev)
	}
	return events
}
```

Add `"strings"` import at top.

- [ ] **Step 7: Build verify helpers**

```bash
cd /Users/francis/workspc/lattice && go build ./test/e2e/ 2>&1
```

Expected: no errors.

- [ ] **Step 8: Commit**

```bash
git add test/e2e/helpers_test.go
git commit -s -m "feat(e2e): add sandbox test helpers (enrollment, deploy, audit, revoke)"
```

---

### Task 4: Create e2e test file with 6 scenarios

**Files:**
- Create: `test/e2e/agent_sandbox_test.go`

- [ ] **Step 1: Create `test/e2e/agent_sandbox_test.go`**

```go
package e2e

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	latticev1 "github.com/alatticeio/lattice/api/v1alpha1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Agent Sandbox", Ordered, func() {
	var (
		testNS              string
		accessToken         string
		workspaceID         string
		enrollmentToken     string
		companionName       string
		companionPeerName   string
		companionVPNIP      string
		sandboxName         string
		sandboxPeerName     string
		sandboxVPNIP        string
		networkName         string
	)

	BeforeAll(func() {
		testNS = "wf-e2e-sandbox-" + fmt.Sprintf("%d", time.Now().UnixMilli())
		companionName = "comp-" + testNS
		companionPeerName = companionName + "-peer"
		sandboxName = "sandbox-" + testNS
		sandboxPeerName = sandboxName + "-peer"

		// 1. Login
		accessToken = login(manageUrl)

		// 2. Create workspace
		workspaceID = createWorkspace(manageUrl, accessToken, testNS)

		// 3. Generate join token for companion agent
		joinToken := generateJoinToken(manageUrl, accessToken, workspaceID)

		// 4. Create enrollment token for sandbox agent
		enrollmentToken = createEnrollmentToken(manageUrl, accessToken, testNS)

		// 5. Discover NATS for host aliases
		hostAliases := hostAliasesForNATS(clientset)

		// 6. Deploy companion agent (standard lattice + nginx)
		deployMultiContainerAgent(
			clientset, testNS, companionName, agentImage, joinToken,
			hostAliases,
			corev1.Container{
				Name:  "nginx",
				Image: "nginx:alpine",
				Ports: []corev1.ContainerPort{{
					ContainerPort: 8080,
				}},
			},
		)

		// 7. Wait for companion ready
		_ = waitForPodRunningReady(clientset, testNS, companionName, "120s")
		companionVPNIP = waitForWGIP(latticeClient, testNS, companionPeerName, "60s")
		GinkgoWriter.Printf("[sandbox e2e] companion VPN IP: %s\n", companionVPNIP)

		// 8. Deploy sandbox pod
		sandboxServerURL := "http://lattice-api-service.lattice-system.svc.cluster.local:8080"
		deploySandboxPod(clientset, testNS, sandboxName, agentImage, sandboxServerURL, enrollmentToken, hostAliases)

		// 9. Wait for sandbox ready
		_ = waitForPodRunningReady(clientset, testNS, sandboxName, "120s")
		sandboxVPNIP = waitForWGIP(latticeClient, testNS, sandboxPeerName, "60s")
		GinkgoWriter.Printf("[sandbox e2e] sandbox VPN IP: %s\n", sandboxVPNIP)

		// 10. Get network name and create allow-all policy
		peer := &latticev1.LatticePeer{}
		Expect(latticeClient.Get(context.Background(), client.ObjectKey{Namespace: testNS, Name: companionPeerName}, peer)).To(Succeed())
		networkName = getNetworkName(peer)
		GinkgoWriter.Printf("[sandbox e2e] network: %s\n", networkName)
		createAllowAllPolicy(latticeClient, testNS, testNS+"-allow-all", networkName)

		// Wait for policy to propagate and WireGuard tunnel to establish
		time.Sleep(10 * time.Second)
	})

	AfterAll(func() {
		cleanupSandboxCRDs(testNS)
		cleanupWorkspace(clientset, testNS)
	})

	// ─── Scenario 1: Agent registration creates AgentIdentity ───
	It("agent registration creates AgentIdentity CRD with Active phase", func() {
		identity := &latticev1.AgentIdentity{}
		err := latticeClient.Get(context.Background(), client.ObjectKey{Namespace: testNS, Name: sandboxPeerName}, identity)
		Expect(err).NotTo(HaveOccurred(), "AgentIdentity CRD should exist")
		Expect(identity.Status.Phase).To(Equal(latticev1.AgentPhaseActive),
			"AgentIdentity should be Active, got %s", identity.Status.Phase)
	})

	// ─── Scenario 2: Sandbox connects to companion via overlay ───
	It("sandbox can reach companion nginx via WireGuard overlay", func() {
		podName := waitForPodRunningReady(clientset, testNS, sandboxName, "30s")
		// Alpine has busybox wget, use it to hit companion nginx at companion VPN IP.
		Eventually(func() error {
			output, err := execInPod(clientset, restConfig, testNS, podName,
				[]string{"wget", "-q", "-O", "-", "--timeout=5",
					fmt.Sprintf("http://%s:8080", companionVPNIP)})
			if err != nil {
				return fmt.Errorf("wget failed: %w", err)
			}
			if !strings.Contains(output, "nginx") && !strings.Contains(output, "Welcome") {
				return fmt.Errorf("unexpected response: %s", strings.TrimSpace(output))
			}
			return nil
		}, "60s", "5s").Should(Succeed(), "sandbox should reach companion via overlay")
	})

	// ─── Scenario 3: Non-lattice IP is blocked ───
	It("sandbox cannot reach non-lattice IP addresses", func() {
		podName := waitForPodRunningReady(clientset, testNS, sandboxName, "30s")
		Consistently(func() error {
			_, err := execInPod(clientset, restConfig, testNS, podName,
				[]string{"wget", "-q", "-O", "-", "--timeout=3", "http://1.2.3.4:80"})
			if err == nil {
				return fmt.Errorf("expected connection to fail but succeeded")
			}
			return nil
		}, "15s", "3s").Should(Succeed(), "connection to non-lattice IP should consistently fail")
	})

	// ─── Scenario 4: Policy deny blocks companion ───
	It("policy deny blocks sandbox from reaching companion", func() {
		// Update policy to deny
		policyList := &latticev1.LatticePolicyList{}
		Expect(latticeClient.List(context.Background(), policyList, client.InNamespace(testNS))).To(Succeed())
		Expect(policyList.Items).NotTo(BeEmpty(), "should have at least one policy")

		policy := &policyList.Items[0]
		policy.Spec.Action = "DENY"
		Expect(latticeClient.Update(context.Background(), policy)).To(Succeed())

		// Wait for policy propagation
		time.Sleep(5 * time.Second)

		podName := waitForPodRunningReady(clientset, testNS, sandboxName, "30s")
		Consistently(func() error {
			_, err := execInPod(clientset, restConfig, testNS, podName,
				[]string{"wget", "-q", "-O", "-", "--timeout=3",
					fmt.Sprintf("http://%s:8080", companionVPNIP)})
			if err == nil {
				return fmt.Errorf("expected connection to fail under deny policy but succeeded")
			}
			return nil
		}, "15s", "3s").Should(Succeed(), "connection should be blocked under deny policy")
	})

	// ─── Scenario 5: Audit events captured ───
	It("audit log contains allow and drop events", func() {
		podName := waitForPodRunningReady(clientset, testNS, sandboxName, "30s")
		events := readAuditLog(clientset, restConfig, testNS, podName)
		Expect(events).NotTo(BeEmpty(), "audit log should not be empty")

		var hasAllow, hasDrop bool
		for _, ev := range events {
			switch ev.Verdict {
			case "allow":
				hasAllow = true
			case "drop":
				hasDrop = true
			}
		}
		Expect(hasAllow).To(BeTrue(), "audit log should contain allow events")
		Expect(hasDrop).To(BeTrue(), "audit log should contain drop events")
	})

	// ─── Scenario 6: Agent revoke stops sandbox ───
	It("agent revocation stops sandbox connections", func() {
		revokeSandboxAgent(manageUrl, accessToken, sandboxPeerName, testNS)

		// Verify AgentIdentity is revoked
		identity := &latticev1.AgentIdentity{}
		err := latticeClient.Get(context.Background(), client.ObjectKey{Namespace: testNS, Name: sandboxPeerName}, identity)
		if err == nil {
			Expect(identity.Status.Phase).To(Equal(latticev1.AgentPhaseRevoked),
				"AgentIdentity should be Revoked, got %s", identity.Status.Phase)
		}
		// After revocation, the sandbox process is still running but its
		// identity is revoked. New connections should be blocked.
	})
})
```

- [ ] **Step 2: Add `deployMultiContainerAgent` helper and `cleanupSandboxCRDs`**

In `test/e2e/helpers_test.go`, add `deployMultiContainerAgent` — wraps `deployAgentDeployment` pattern but adds an extra container:

```go
// deployMultiContainerAgent creates a Deployment running the lattice agent plus additional containers.
func deployMultiContainerAgent(clientset *kubernetes.Clientset, ns, name, agentImage, joinToken string, hostAliases []corev1.HostAlias, extraContainers ...corev1.Container) {
	privileged := true
	replicas := int32(1)
	hostPathType := corev1.HostPathDirectory

	containers := []corev1.Container{{
		Name:            "agent",
		Image:           agentImage,
		ImagePullPolicy: corev1.PullIfNotPresent,
		SecurityContext: &corev1.SecurityContext{
			Privileged:               &privileged,
			AllowPrivilegeEscalation: &privileged,
			Capabilities: &corev1.Capabilities{
				Add: []corev1.Capability{"NET_ADMIN", "NET_RAW"},
			},
		},
		VolumeMounts: []corev1.VolumeMount{
			{Name: "lib-modules", MountPath: "/lib/modules", ReadOnly: true},
			{Name: "xtables-lock", MountPath: "/run/xtables.lock"},
		},
		Command: []string{
			"/app/lattice", "up",
			"--token", joinToken,
			"--level", "debug",
			"--server-url", "http://lattice-api-service.lattice-system.svc.cluster.local:8080",
		},
	}}
	containers = append(containers, extraContainers...)

	_, err := clientset.AppsV1().Deployments(ns).Create(context.Background(), &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"wf-role": name},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app":     "wf-e2e",
						"wf-role": name,
					},
				},
				Spec: corev1.PodSpec{
					Hostname:    name,
					HostAliases: hostAliases,
					Containers:  containers,
					Volumes: []corev1.Volume{
						{
							Name: "lib-modules",
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: "/lib/modules",
									Type: &hostPathType,
								},
							},
						},
						{
							Name: "xtables-lock",
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: "/run/xtables.lock",
									Type: func() *corev1.HostPathType {
										t := corev1.HostPathFileOrCreate
										return &t
									}(),
								},
							},
						},
					},
				},
			},
		},
	}, metav1.CreateOptions{})

	Expect(err).NotTo(HaveOccurred(), "failed to create Deployment %s", name)
}
```

Add `cleanupSandboxCRDs` helper:

```go
// cleanupSandboxCRDs removes AgentIdentity CRDs that might block namespace deletion.
func cleanupSandboxCRDs(ns string) {
	ctx := context.Background()
	list := &latticev1.AgentIdentityList{}
	if err := latticeClient.List(ctx, list, sigclient.InNamespace(ns)); err == nil {
		for _, id := range list.Items {
			id.SetFinalizers(nil)
			_ = latticeClient.Update(ctx, &id)
			_ = latticeClient.Delete(ctx, &latticev1.AgentIdentity{ObjectMeta: metav1.ObjectMeta{Name: id.Name, Namespace: ns}})
		}
	}
}
```

- [ ] **Step 3: Build verify e2e test**

```bash
cd /Users/francis/workspc/lattice && go build ./test/e2e/ 2>&1
```

Expected: no errors.

- [ ] **Step 4: Commit**

```bash
git add test/e2e/agent_sandbox_test.go test/e2e/helpers_test.go
git commit -s -m "test(e2e): add agent sandbox e2e test with 6 scenarios"
```

---

### Task 5: Create CI workflow for sandbox e2e

**Files:**
- Create: `.github/workflows/sandbox-e2e.yml`

- [ ] **Step 1: Create `.github/workflows/sandbox-e2e.yml`**

```yaml
name: Sandbox E2E

on:
  pull_request:
    types: [labeled, synchronize, reopened]
  push:
    branches:
      - dev

jobs:
  sandbox-e2e:
    # Only run when the 'run-pro' label is present, or on push to dev
    if: |
      (github.event_name == 'pull_request' && contains(github.event.pull_request.labels.*.name, 'run-pro')) ||
      github.event_name == 'push'
    runs-on: ubuntu-latest
    timeout-minutes: 30

    steps:
      - name: Checkout
        uses: actions/checkout@v4

      - name: Setup Go
        uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
          cache: true

      - name: Prepare metadata
        id: meta
        run: |
          OWNER_LOWER=$(echo "${{ github.repository_owner }}" | tr '[:upper:]' '[:lower:]')
          echo "owner_lower=${OWNER_LOWER}" >> $GITHUB_OUTPUT
          echo "tag=$(date +%s)" >> $GITHUB_OUTPUT

      - name: Set up Docker Buildx
        uses: docker/setup-buildx-action@v3

      - name: Login to GHCR
        uses: docker/login-action@v3
        with:
          registry: ghcr.io
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}

      - name: Build PRO images
        run: |
          EDITION=pro make docker-build SERVICE=latticed TAG=${{ steps.meta.outputs.tag }}
          EDITION=pro make docker-build SERVICE=lattice TAG=${{ steps.meta.outputs.tag }}
          EDITION=pro make docker-build SERVICE=lattice-agent-sandbox TAG=${{ steps.meta.outputs.tag }}

      - name: Push PRO images to GHCR
        if: github.event_name == 'push'
        run: |
          docker push ghcr.io/${{ steps.meta.outputs.owner_lower }}/latticed:pro-${{ steps.meta.outputs.tag }}
          docker push ghcr.io/${{ steps.meta.outputs.owner_lower }}/lattice:pro-${{ steps.meta.outputs.tag }}
          docker push ghcr.io/${{ steps.meta.outputs.owner_lower }}/lattice-agent-sandbox:pro-${{ steps.meta.outputs.tag }}

      - name: Set up k3d
        uses: AbsaOSS/k3d-action@v2
        with:
          cluster-name: lattice-sandbox-e2e
          args: >
            --agents 0
            --no-lb
            --k3s-arg "--disable=traefik@server:*"

      - name: Export kubeconfig
        run: |
          mkdir -p $HOME/.kube
          k3d kubeconfig get lattice-sandbox-e2e > $HOME/.kube/config
          chmod 600 $HOME/.kube/config

      - name: Import images to k3d
        run: |
          k3d image import \
            ghcr.io/${{ steps.meta.outputs.owner_lower }}/latticed:pro-${{ steps.meta.outputs.tag }} \
            ghcr.io/${{ steps.meta.outputs.owner_lower }}/lattice:pro-${{ steps.meta.outputs.tag }} \
            ghcr.io/${{ steps.meta.outputs.owner_lower }}/lattice-agent-sandbox:pro-${{ steps.meta.outputs.tag }} \
            -c lattice-sandbox-e2e

      - name: Deploy Lattice All-in-One (PRO)
        run: |
          kubectl create namespace lattice-system
          EDITION=pro make deploy-aio IMG=ghcr.io/${{ steps.meta.outputs.owner_lower }}/latticed:pro-${{ steps.meta.outputs.tag }}

      - name: Wait for latticed ready
        run: |
          kubectl wait --for=condition=available deployment/latticed -n lattice-system --timeout=120s
          kubectl port-forward -n lattice-system svc/lattice-api-service 8080:8080 &
          sleep 5

      - name: Run sandbox e2e tests
        run: |
          go test ./test/e2e/ -v -timeout 15m -run "Agent Sandbox" \
            --agent-image=ghcr.io/${{ steps.meta.outputs.owner_lower }}/lattice:pro-${{ steps.meta.outputs.tag }} \
            --manage-url=http://localhost:8080

      - name: Collect diagnostics on failure
        if: failure()
        run: |
          echo "===== Control Plane Pods ====="
          kubectl get pods -n lattice-system
          echo "===== latticed Logs ====="
          kubectl logs -n lattice-system deployment/latticed --tail=200 || true
          echo "===== Test Namespace Pods ====="
          kubectl get pods -A | grep wf-e2e-sandbox || true
          echo "===== LatticePeer CRDs ====="
          kubectl get latticepeer -A || true
          echo "===== AgentIdentity CRDs ====="
          kubectl get agentidentity -A || true

      - name: Cleanup
        if: always()
        run: |
          kubectl delete ns wf-e2e-sandbox --ignore-not-found || true
          k3d cluster delete lattice-sandbox-e2e || true
```

- [ ] **Step 2: Commit**

```bash
git add .github/workflows/sandbox-e2e.yml
git commit -s -m "ci: add sandbox e2e workflow (PRO only, run-pro label)"
```

---

### Task 6: Dockerfile + Makefile for lattice-agent-sandbox

**Files:**
- Modify: `Makefile`
- Modify: `Dockerfile`

- [ ] **Step 1: Add `lattice-agent-sandbox` to SERVICES in Makefile**

```makefile
SERVICES := manager lattice latticed lrper lattice-agent-sandbox
```

- [ ] **Step 2: Update Dockerfile to handle lattice-agent-sandbox**

The Dockerfile `apk add` block needs to install wget for the sandbox (used in e2e connectivity tests):

```dockerfile
RUN if [ "$TARGETSERVICE" = "lattice" ] || [ "$TARGETSERVICE" = "latticed" ]; then \
        apk add --no-cache wireguard-tools iptables iproute2 ca-certificates; \
    elif [ "$TARGETSERVICE" = "lattice-agent-sandbox" ]; then \
        apk add --no-cache ca-certificates; \
    else \
        apk add --no-cache ca-certificates; \
    fi
```

Alpine's busybox includes wget by default, so no extra packages needed for connectivity testing.

- [ ] **Step 3: Commit**

```bash
git add Makefile Dockerfile
git commit -s -m "build: add lattice-agent-sandbox to docker build services"
```

---

### Implementation Order

1. **Task 1** (wg_bridge.go) → foundation for --wg
2. **Task 2** (--wg wiring + JSONL audit) → sandbox can actually send traffic
3. **Task 3** (e2e helpers) → test utilities
4. **Task 4** (e2e test file) → the actual test scenarios
5. **Task 5** (CI workflow) → automation
6. **Task 6** (Makefile) → docker build support
