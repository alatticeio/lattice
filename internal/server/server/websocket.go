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
//	// Subscribe to NATS (Agent feedback)
//	// Assuming you have already initialized natsConn
//	sub, _ := natsConn.Subscribe("feedback.firewall", func(m *nats.Msg) {
//		// When the Agent has updates, push them immediately to the Vue frontend
//		conn.WriteMessage(websocket.TextMessage, m.Data)
//	})
//	defer sub.Unsubscribe()
//
//	// Block until the frontend disconnects
//	for {
//		if _, _, err := conn.ReadMessage(); err != nil {
//			break
//		}
//	}
//}
