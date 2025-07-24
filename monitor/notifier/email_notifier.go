package notifier

import (
	"fmt"
	"log"
	"net/smtp"
	"strings"
	"sync"
	"time"
)

// EmailNotifier 邮件通知器
type EmailNotifier struct {
	smtpServer  string
	smtpPort    int
	username    string
	password    string
	from        string
	to          []string
	tlsEnabled  bool
	lastCheckAt time.Time
	available   bool
	checkMutex  sync.Mutex
}

func NewEmailNotifier(smtpServer string, smtpPort int, username, password, from string, to []string, tlsEnabled bool) *EmailNotifier {
	notifier := &EmailNotifier{
		smtpServer: smtpServer,
		smtpPort:   smtpPort,
		username:   username,
		password:   password,
		from:       from,
		to:         to,
		tlsEnabled: tlsEnabled,
	}

	// 初始化时检查服务可用性
	notifier.checkAvailability()

	return notifier
}

func (n *EmailNotifier) Name() string {
	return "email"
}

func (n *EmailNotifier) IsAvailable() bool {
	n.checkMutex.Lock()
	defer n.checkMutex.Unlock()

	// 如果上次检查是在10分钟以内，则使用缓存的结果
	if time.Since(n.lastCheckAt) < 10*time.Minute {
		return n.available
	}

	return n.checkAvailability()
}

func (n *EmailNotifier) checkAvailability() bool {
	n.lastCheckAt = time.Now()

	// 创建连接到SMTP服务器的客户端
	client, err := smtp.Dial(fmt.Sprintf("%s:%d", n.smtpServer, n.smtpPort))
	if err != nil {
		log.Printf("SMTP连接失败: %v", err)
		n.available = false
		return false
	}
	defer client.Close()

	// 尝试认证
	if n.username != "" && n.password != "" {
		auth := smtp.PlainAuth("", n.username, n.password, n.smtpServer)
		if err := client.Auth(auth); err != nil {
			log.Printf("SMTP认证失败: %v", err)
			n.available = false
			return false
		}
	}

	n.available = true
	return true
}

func (n *EmailNotifier) Notify(alerts []Alert) error {
	if len(alerts) == 0 {
		return nil
	}

	if !n.IsAvailable() {
		return fmt.Errorf("邮件服务不可用")
	}

	// 构建邮件内容
	var body strings.Builder

	body.WriteString("<!DOCTYPE html><html><body>")
	body.WriteString("<h2>系统告警通知</h2>")
	body.WriteString("<table border='1' cellpadding='5' cellspacing='0' style='border-collapse:collapse'>")
	body.WriteString("<tr style='background-color:#f2f2f2'><th>告警名称</th><th>严重程度</th><th>指标</th><th>当前值</th><th>阈值</th><th>状态</th><th>开始时间</th><th>结束时间</th></tr>")

	for _, alert := range alerts {
		// 根据严重程度设置不同的颜色
		severityColor := "#000000"
		switch alert.Severity {
		case "critical":
			severityColor = "#ff0000" // 红色
		case "error":
			severityColor = "#ff9900" // 橙色
		case "warning":
			severityColor = "#ffcc00" // 黄色
		case "info":
			severityColor = "#0099cc" // 蓝色
		}

		endTimeStr := "-"
		if alert.EndTime != nil {
			endTimeStr = alert.EndTime.Format("2006-01-02 15:04:05")
		}

		body.WriteString(fmt.Sprintf("<tr>"))
		body.WriteString(fmt.Sprintf("<td>%s</td>", alert.Name))
		body.WriteString(fmt.Sprintf("<td style='color:%s;font-weight:bold'>%s</td>", severityColor, alert.Severity))
		body.WriteString(fmt.Sprintf("<td>%s</td>", alert.MetricName))
		body.WriteString(fmt.Sprintf("<td>%v</td>", alert.MetricValue))
		body.WriteString(fmt.Sprintf("<td>%v</td>", alert.Threshold))
		body.WriteString(fmt.Sprintf("<td>%s</td>", alert.Status))
		body.WriteString(fmt.Sprintf("<td>%s</td>", alert.StartTime.Format("2006-01-02 15:04:05")))
		body.WriteString(fmt.Sprintf("<td>%s</td>", endTimeStr))
		body.WriteString(fmt.Sprintf("</tr>"))
	}

	body.WriteString("</table>")
	body.WriteString("<p>详细信息请登录监控系统查看。</p>")
	body.WriteString("</body></html>")

	// 设置邮件标题
	subject := fmt.Sprintf("系统告警: %d条告警通知", len(alerts))

	// 构造邮件头部
	header := make(map[string]string)
	header["From"] = n.from
	header["To"] = strings.Join(n.to, ",")
	header["Subject"] = subject
	header["MIME-Version"] = "1.0"
	header["Content-Type"] = "text/html; charset=UTF-8"

	var message strings.Builder
	for k, v := range header {
		message.WriteString(fmt.Sprintf("%s: %s\r\n", k, v))
	}
	message.WriteString("\r\n")
	message.WriteString(body.String())

	// 发送邮件
	auth := smtp.PlainAuth("", n.username, n.password, n.smtpServer)
	err := smtp.SendMail(
		fmt.Sprintf("%s:%d", n.smtpServer, n.smtpPort),
		auth,
		n.from,
		n.to,
		[]byte(message.String()),
	)

	if err != nil {
		return fmt.Errorf("发送邮件失败: %v", err)
	}

	return nil
}
