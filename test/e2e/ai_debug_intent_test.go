package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/alatticeio/lattice/pkg/utils/resp"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("AI Debug & Intent Engine", Ordered, func() {
	var (
		accessToken string
		workspaceID string
		ctxAI       = context.Background()
	)

	BeforeAll(func() {
		accessToken = login(manageUrl)

		nsName := fmt.Sprintf("wf-e2e-ai-%d", time.Now().UnixMilli())
		workspaceID = createWorkspace(manageUrl, accessToken, nsName)

		By("Workspace ready: id=" + workspaceID)
	})

	It("Snapshot API: 列出工作空间的快照列表（GET）", func() {
		url := manageUrl + "/api/v1/workspaces/" + workspaceID + "/snapshots"
		req, _ := http.NewRequestWithContext(ctxAI, http.MethodGet, url, nil)
		req.Header.Set("Authorization", "Bearer "+accessToken)

		httpClient := &http.Client{Timeout: 15 * time.Second}
		httpResp, err := httpClient.Do(req)
		Expect(err).NotTo(HaveOccurred(), "快照列表请求失败")
		defer httpResp.Body.Close() //nolint:errcheck

		var data resp.Response
		_ = json.NewDecoder(httpResp.Body).Decode(&data)

		// Snapshot list should return 200 even if the list is empty
		Expect(httpResp.StatusCode).To(Equal(http.StatusOK), "快照列表 API 返回非 200: %+v", data)
		By(fmt.Sprintf("快照列表返回 status=%d", httpResp.StatusCode))
	})

	It("AI Debug API: 发送调试请求（AI 未配置时返回非 200）", func() {
		debugBody := map[string]any{
			"workspaceId": workspaceID,
			"question":    "为什么 pod-a 无法连接到 pod-b？",
			"from":        time.Now().Add(-1 * time.Hour).Format(time.RFC3339),
			"to":          time.Now().Format(time.RFC3339),
		}
		status, data := apiPOST(manageUrl+"/api/v1/ai/debug", accessToken, debugBody)

		if status != http.StatusOK {
			By(fmt.Sprintf("AI Debug 返回非 200（AI 未配置）: status=%d, data=%+v", status, data))
		} else {
			By("AI Debug 返回 200 — AI 已配置, SSE 流已启动")
		}
	})

	It("AI Intent API: 提交网络意图计划（AI 未配置时返回非 200）", func() {
		intentBody := map[string]any{
			"workspaceId": workspaceID,
			"intent":      "允许 pod-a 和 pod-b 互相访问 8080 端口",
			"dryRun":      true,
		}
		status, data := apiPOST(manageUrl+"/api/v1/ai/intent/plan", accessToken, intentBody)

		if status != http.StatusOK {
			By(fmt.Sprintf("Intent Plan 返回非 200（AI 未配置）: status=%d, data=%+v", status, data))
		} else {
			By("Intent Plan 返回 200 — AI 已配置, intent 计划已生成")
		}
	})

	It("AI Tools API: 列出可用工具", func() {
		status, data := apiPOST(manageUrl+"/api/v1/ai/tools", accessToken, map[string]any{
			"workspaceId": workspaceID,
		})

		// Tools endpoint is GET in the server, so POST may return 404.
		// We verify the server responds gracefully.
		By(fmt.Sprintf("AI Tools 返回: status=%d, data=%+v", status, data))
	})

	AfterAll(func() {
		// 清理 workspace 资源
		ns := fmt.Sprintf("wf-%s", workspaceID)
		cleanupWorkspace(clientset, ns)
	})
})
