package server

//
//var upgrader = websocket.Upgrader{
//	CheckOrigin: func(r *http.Request) bool { return true },
//}
//
//func HandleStatusWS(c *gin.Context) {
//	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
//	if err != nil {
//		return
//	}
//	defer conn.Close()
//
//	// 订阅 NATS (Agent 的反馈)
//	// 假设你已经初始化了 natsConn
//	sub, _ := natsConn.Subscribe("feedback.firewall", func(m *nats.Msg) {
//		// 当 Agent 有更新时，立即推给 Vue 前端
//		conn.WriteMessage(websocket.TextMessage, m.Data)
//	})
//	defer sub.Unsubscribe()
//
//	// 阻塞直到前端断开连接
//	for {
//		if _, _, err := conn.ReadMessage(); err != nil {
//			break
//		}
//	}
//}
