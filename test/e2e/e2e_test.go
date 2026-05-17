// Copyright 2024 The Lattice Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package e2e

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/alatticeio/lattice/api/v1alpha1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	sigclient "sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	podA = "pod-a"
	podB = "pod-b"
)

var _ = Describe("Lattice Core Connectivity E2E", Ordered, func() {
	var (
		testNS      string
		podAName    string
		podBName    string
		podAWGIP    string
		podBWGIP    string
		networkName string
		ctx         = context.Background()
	)

	BeforeAll(func() {
		accessToken := login(manageUrl)

		nsName := fmt.Sprintf("wf-e2e-core-%d", time.Now().UnixMilli())
		workspaceId := createWorkspace(manageUrl, accessToken, nsName)
		testNS = fmt.Sprintf("wf-%s", workspaceId)

		joinToken := generateJoinToken(manageUrl, accessToken, workspaceId)
		hostAliases := hostAliasesForNATS(clientset)

		for _, name := range []string{podA, podB} {
			deployAgentDeployment(clientset, testNS, name, agentImage, joinToken, hostAliases)
		}

		podAName = waitForPodRunningReady(clientset, testNS, podA, "180s")
		podBName = waitForPodRunningReady(clientset, testNS, podB, "180s")

		podAWGIP = waitForWGIP(latticeClient, testNS, podA, "90s")
		podBWGIP = waitForWGIP(latticeClient, testNS, podB, "90s")

		peerB := &v1alpha1.LatticePeer{}
		Expect(latticeClient.Get(ctx, sigclient.ObjectKey{Namespace: testNS, Name: podB}, peerB)).To(Succeed())
		networkName = getNetworkName(peerB)
		Expect(networkName).NotTo(BeEmpty(), "failed to get network name from LatticePeer")

		createAllowAllPolicy(latticeClient, testNS, "e2e-allow-all", networkName)
	})

	AfterAll(func() {
		if CurrentSpecReport().Failed() {
			collectDiagnostics(ctx, testNS)
		}
		cleanupWorkspace(clientset, testNS)
	})

	It("Full chain: verify WireGuard tunnel connectivity between pod-a and pod-b", func() {
		By(fmt.Sprintf("Verify tunnel: %s → %s @ %s", podAName, podBName, podBWGIP))
		pingWithRetry(clientset, restConfig, testNS, podAName, podBWGIP, "60s")
	})

	It("ACL rule verification: DENY blocking + port-level control + CIDR rules", func() {
		networkLabel := fmt.Sprintf("alattice.io/network-%s", networkName)
		peerNetSelector := metav1.LabelSelector{
			MatchLabels: map[string]string{networkLabel: "true"},
		}

		allowPolicy := &v1alpha1.LatticePolicy{}
		Expect(latticeClient.Get(ctx, sigclient.ObjectKey{Namespace: testNS, Name: "e2e-allow-all"}, allowPolicy)).To(Succeed())

		By("ACL-1: Verify ping works under e2e-allow-all policy (baseline)")
		output, err := execInPod(clientset, restConfig, testNS, podAName, []string{"ping", "-c", "3", "-W", "2", podBWGIP})
		Expect(err).NotTo(HaveOccurred(), "baseline ping failed")
		Expect(strings.Contains(output, "0% packet loss")).To(BeTrue(), "baseline ping has packet loss: %s", output)

		By("ACL-2: Update policy to DENY, verify pod-a → pod-b is blocked")
		allowPolicy.Spec.Action = "DENY"
		Expect(latticeClient.Update(ctx, allowPolicy)).To(Succeed(), "failed to update LatticePolicy to DENY")
		assertPingBlocked(clientset, restConfig, testNS, podAName, podBWGIP, "30s")

		By("ACL-3: Restore ALLOW + port-level rules, only allow TCP 8080")
		Expect(latticeClient.Get(ctx, sigclient.ObjectKey{Namespace: testNS, Name: "e2e-allow-all"}, allowPolicy)).To(Succeed())
		allowPolicy.Spec.Action = "ALLOW"
		allowPolicy.Spec.Ingress = []v1alpha1.IngressRule{
			{
				From:  []v1alpha1.PeerSelection{{PeerSelector: &peerNetSelector}},
				Ports: []v1alpha1.NetworkPolicyPort{{Port: 8080, Protocol: "tcp"}},
			},
		}
		allowPolicy.Spec.Egress = []v1alpha1.EgressRule{
			{
				To:    []v1alpha1.PeerSelection{{PeerSelector: &peerNetSelector}},
				Ports: []v1alpha1.NetworkPolicyPort{{Port: 8080, Protocol: "tcp"}},
			},
		}
		Expect(latticeClient.Update(ctx, allowPolicy)).To(Succeed(), "failed to update port-level policy")
		assertPingBlocked(clientset, restConfig, testNS, podAName, podBWGIP, "30s")

		By("ACL-4: Verify iptables rules allow TCP 8080 (in LATTICE-EGRESS chain)")
		Eventually(func() error {
			out, execErr := execInPod(clientset, restConfig, testNS, podAName, []string{"iptables", "-L", "LATTICE-EGRESS", "-n"})
			if execErr != nil {
				return execErr
			}
			if !strings.Contains(out, "tcp dpt:8080") {
				return fmt.Errorf("LATTICE-EGRESS does not contain TCP 8080 rule:\n%s", out)
			}
			return nil
		}, "15s", "2s").Should(Succeed(), "iptables should contain TCP 8080 allow rule")

		By("ACL-5: Verify iptables does not have TCP 9999 rule")
		out, err := execInPod(clientset, restConfig, testNS, podAName, []string{"sh", "-c", "iptables -L LATTICE-EGRESS -n 2>/dev/null | grep 9999 || true"})
		Expect(err).NotTo(HaveOccurred())
		Expect(out).To(BeEmpty(), "should not have TCP 9999 iptables rule")

		By("ACL-6: Verify IPBlock CIDR rule — only allow exact peer CIDRs")
		Expect(latticeClient.Get(ctx, sigclient.ObjectKey{Namespace: testNS, Name: "e2e-allow-all"}, allowPolicy)).To(Succeed())
		podACIDR := podAWGIP + "/32"
		podBCIDR := podBWGIP + "/32"
		allowPolicy.Spec.Action = "ALLOW"
		allowPolicy.Spec.Ingress = []v1alpha1.IngressRule{
			{From: []v1alpha1.PeerSelection{
				{IPBlock: &v1alpha1.IPBlock{CIDR: podACIDR}},
				{IPBlock: &v1alpha1.IPBlock{CIDR: podBCIDR}},
			}},
		}
		allowPolicy.Spec.Egress = []v1alpha1.EgressRule{
			{To: []v1alpha1.PeerSelection{
				{IPBlock: &v1alpha1.IPBlock{CIDR: podACIDR}},
				{IPBlock: &v1alpha1.IPBlock{CIDR: podBCIDR}},
			}},
		}
		Expect(latticeClient.Update(ctx, allowPolicy)).To(Succeed(), "failed to update CIDR policy")

		Eventually(func() error {
			out, execErr := execInPod(clientset, restConfig, testNS, podAName, []string{"iptables", "-L", "LATTICE-EGRESS", "-n"})
			if execErr != nil {
				return execErr
			}
			if !strings.Contains(out, podBWGIP) {
				return fmt.Errorf("LATTICE-EGRESS does not contain CIDR rule for %s:\n%s", podBWGIP, out)
			}
			return nil
		}, "15s", "2s").Should(Succeed(), "iptables should contain CIDR allow rule")

		By("ACL-6: Verify WireGuard handshake established under CIDR policy")
		Eventually(func() error {
			out, err := execInPod(clientset, restConfig, testNS, podAName, []string{"wg", "show"})
			if err != nil {
				return err
			}
			if !strings.Contains(out, "latest handshake") {
				return fmt.Errorf("WireGuard handshake not yet completed:\n%s", out)
			}
			return nil
		}, "15s", "2s").Should(Succeed(), "WG handshake should have completed under CIDR policy")

		Eventually(func() error {
			out, execErr := execInPod(clientset, restConfig, testNS, podAName, []string{"ping", "-c", "3", "-W", "2", podBWGIP})
			if execErr != nil {
				diag := collectBothPodsDiagnostics(clientset, restConfig, testNS, podAName, podBName, podBWGIP)
				return fmt.Errorf("ping execution failed: %w\nDiagnostic summary: %s", execErr, diag)
			}
			if !strings.Contains(out, "0% packet loss") {
				return fmt.Errorf("ping should work under CIDR rule: %s", out)
			}
			return nil
		}, "30s", "2s").Should(Succeed(), "CIDR policy should allow pod-b's IP")

		By("ACL-7: Replace CIDR with non-matching subnet, verify traffic is blocked")
		Expect(latticeClient.Get(ctx, sigclient.ObjectKey{Namespace: testNS, Name: "e2e-allow-all"}, allowPolicy)).To(Succeed())
		allowPolicy.Spec.Egress = []v1alpha1.EgressRule{
			{To: []v1alpha1.PeerSelection{{IPBlock: &v1alpha1.IPBlock{CIDR: "192.168.99.0/24"}}}},
		}
		Expect(latticeClient.Update(ctx, allowPolicy)).To(Succeed(), "failed to update CIDR policy")
		assertPingBlocked(clientset, restConfig, testNS, podAName, podBWGIP, "30s")

		By("ACL test completed, cleaning up policy")
		_ = latticeClient.Delete(ctx, allowPolicy)
	})
})

// collectDiagnostics prints key logs on test failure to aid CI debugging
func collectDiagnostics(ctx context.Context, namespace string) {
	w := GinkgoWriter
	fprintf := func(format string, args ...any) { fmt.Fprintf(w, format, args...) } //nolint:errcheck

	fprintf("\n========== E2E Diagnostic Logs [ns=%s] ==========\n", namespace)

	fprintf("\n[LatticePeer Status]\n")
	var peerList v1alpha1.LatticePeerList
	if err := latticeClient.List(ctx, &peerList, sigclient.InNamespace(namespace)); err != nil {
		fprintf("  [WARN] failed to list LatticePeer: %v\n", err)
	} else {
		for _, p := range peerList.Items {
			addr := "<nil>"
			if p.Status.AllocatedAddress != nil {
				addr = *p.Status.AllocatedAddress
			}
			activeNet := "<nil>"
			if p.Status.ActiveNetwork != nil {
				activeNet = *p.Status.ActiveNetwork
			}
			fprintf("  %-20s  phase=%-12s  ip=%-18s  activeNetwork=%-30s  hash=%s\n",
				p.Name, p.Status.Phase, addr, activeNet, p.Status.CurrentHash)
			for _, c := range p.Status.Conditions {
				fprintf("    condition %-25s  status=%-5s  reason=%-20s  msg=%s\n",
					c.Type, c.Status, c.Reason, c.Message)
			}
		}
	}

	fprintf("\n[LatticeNetwork Status]\n")
	var netList v1alpha1.LatticeNetworkList
	if err := latticeClient.List(ctx, &netList, sigclient.InNamespace(namespace)); err != nil {
		fprintf("  [WARN] failed to list LatticeNetwork: %v\n", err)
	} else {
		for _, n := range netList.Items {
			fprintf("  %-30s  phase=%-10s  activeCIDR=%-20s  allocatedCount=%d\n",
				n.Name, n.Status.Phase, n.Status.ActiveCIDR, n.Status.AllocatedCount)
		}
	}

	fprintf("\n[ConfigMap Contents]\n")
	cms, err := clientset.CoreV1().ConfigMaps(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: "app.kubernetes.io/managed-by=lattice-controller",
	})
	if err != nil {
		fprintf("  [WARN] failed to list ConfigMap: %v\n", err)
	} else {
		for _, cm := range cms.Items {
			fprintf("\n  --- ConfigMap: %s ---\n", cm.Name)
			for k, v := range cm.Data {
				fprintf("  [%s]\n%s\n", k, v)
			}
		}
	}

	fprintf("\n[Pod Logs and Network Status]\n")
	pods, err := clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		fprintf("  [WARN] failed to list Pod: %v\n", err)
	} else {
		for _, pod := range pods.Items {
			fprintf("\n--- Pod: %s  Phase: %s ---\n", pod.Name, pod.Status.Phase)
			for _, cs := range pod.Status.ContainerStatuses {
				fprintf("  Container %s: ready=%v restarts=%d\n", cs.Name, cs.Ready, cs.RestartCount)
			}

			if pod.Status.Phase == corev1.PodRunning {
				// Run network/status diagnostics in the first container.
				firstContainer := ""
				if len(pod.Spec.Containers) > 0 {
					firstContainer = pod.Spec.Containers[0].Name
				}
				// lattice status is only available in the regular agent container.
				if firstContainer == "agent" {
					if out, lerr := execInContainer(clientset, restConfig, namespace, pod.Name, firstContainer,
						[]string{"/app/lattice", "status"}); lerr != nil {
						fprintf("  [lattice status] execution failed: %v\n", lerr)
					} else {
						fprintf("  [lattice status]\n%s\n", out)
					}
				}
				if out, lerr := execInContainer(clientset, restConfig, namespace, pod.Name, firstContainer,
					[]string{"ip", "addr", "show"}); lerr != nil {
					fprintf("  [ip addr] execution failed: %v\n", lerr)
				} else {
					fprintf("  [ip addr]\n%s\n", out)
				}
				if out, lerr := execInContainer(clientset, restConfig, namespace, pod.Name, firstContainer,
					[]string{"ip", "route", "show"}); lerr != nil {
					fprintf("  [ip route] execution failed: %v\n", lerr)
				} else {
					fprintf("  [ip route]\n%s\n", out)
				}
			}

			// Collect logs from every container separately (multi-container pods
			// like the sandbox pod require an explicit container name).
			tailLines := int64(200)
			for _, c := range pod.Spec.Containers {
				logReq := clientset.CoreV1().Pods(namespace).GetLogs(pod.Name, &corev1.PodLogOptions{
					Container: c.Name,
					TailLines: &tailLines,
				})
				logStream, lerr := logReq.Stream(ctx)
				if lerr != nil {
					fprintf("  [WARN] failed to get logs for container %s: %v\n", c.Name, lerr)
				} else {
					var buf bytes.Buffer
					_, _ = buf.ReadFrom(logStream)
					_ = logStream.Close()
					fprintf("  [%s log]\n%s\n", c.Name, buf.String())
				}

				// Also collect previous-run logs for restarted containers.
				for _, cs := range pod.Status.ContainerStatuses {
					if cs.Name != c.Name || cs.RestartCount == 0 {
						continue
					}
					prevReq := clientset.CoreV1().Pods(namespace).GetLogs(pod.Name, &corev1.PodLogOptions{
						Container: c.Name,
						TailLines: &tailLines,
						Previous:  true,
					})
					prevStream, perr := prevReq.Stream(ctx)
					if perr != nil {
						fprintf("  [WARN] failed to get previous logs for container %s: %v\n", c.Name, perr)
						continue
					}
					var prevBuf bytes.Buffer
					_, _ = prevBuf.ReadFrom(prevStream)
					_ = prevStream.Close()
					fprintf("  [%s log (previous run)]\n%s\n", c.Name, prevBuf.String())
				}
			}
		}
	}

	fprintf("===========================================\n")
}

func collectPodDiagnostics(c *kubernetes.Clientset, config *rest.Config, namespace, podName, targetIP string) string {
	var buf bytes.Buffer
	cmds := []struct {
		name string
		args []string
	}{
		{"LATTICE-EGRESS", []string{"iptables", "-L", "LATTICE-EGRESS", "-n"}},
		{"LATTICE-INGRESS", []string{"iptables", "-L", "LATTICE-INGRESS", "-n"}},
		{"WG", []string{"wg", "show"}},
		{"WG dump", []string{"wg", "show", "wf0", "dump"}},
		{"WF0 stats", []string{"sh", "-c", "ip -s link show wf0"}},
		{"ROUTE", []string{"ip", "route", "get", targetIP}},
		{"ALL iptables", []string{"sh", "-c", "iptables -S; iptables -t nat -S"}},
		{"lattice status", []string{"/app/lattice", "status"}},
	}
	for _, cmd := range cmds {
		out, err := execInPod(c, config, namespace, podName, cmd.args)
		if err != nil {
			fmt.Fprintf(&buf, "[%s] error: %v\n", cmd.name, err)
		} else {
			fmt.Fprintf(&buf, "--- %s ---\n%s\n", cmd.name, out)
		}
	}

	tailLines := int64(50)
	logReq := c.CoreV1().Pods(namespace).GetLogs(podName, &corev1.PodLogOptions{
		TailLines: &tailLines,
	})
	logStream, err := logReq.Stream(context.Background())
	if err != nil {
		fmt.Fprintf(&buf, "[agent log] error: %v\n", err)
	} else {
		var logBuf bytes.Buffer
		_, _ = logBuf.ReadFrom(logStream)
		_ = logStream.Close()
		fmt.Fprintf(&buf, "--- agent log (last 50 lines) ---\n%s\n", logBuf.String())
	}

	return buf.String()
}

func collectBothPodsDiagnostics(c *kubernetes.Clientset, config *rest.Config, namespace, srcPod, dstPod, targetIP string) string {
	w := GinkgoWriter
	fmt.Fprintf(w, "\n========== Pod diagnostics: %s ==========\n", srcPod) //nolint:errcheck
	diagA := collectPodDiagnostics(c, config, namespace, srcPod, targetIP)
	fmt.Fprintf(w, "%s\n", diagA) //nolint:errcheck

	fmt.Fprintf(w, "\n========== Pod diagnostics: %s ==========\n", dstPod) //nolint:errcheck
	diagB := collectPodDiagnostics(c, config, namespace, dstPod, targetIP)
	fmt.Fprintf(w, "%s\n", diagB) //nolint:errcheck

	return fmt.Sprintf("diagnostics written to GinkgoWriter for pods %s and %s", srcPod, dstPod)
}
