package main

import (
	"github.com/gorilla/websocket"
)

//客户端
//type Client struct {
//	R *Room   //属于哪个房间
//	Uid int64 //房间ID
//	Name string //房间名称
//	conn *websocket.Conn //websocket 连接
//	Send chan []byte //发送数据
//}





// Room 房间
type Room struct {
	Id         int64  //房间ID
	Name       string //房间名字
	Clients    map[*Client]bool
	Broadcast  chan []byte  //需要广播的消息
	Register   chan *Client //刚进入到这个房间的客户端
	UnRegister chan *Client //退出到这个房间的客户端
}

//func (h *Scheduler) Run() {
//	for {
//		select {
//		case cli := <-h.Register:
//			h.Clients[cli] = true
//		case cli := <-h.UnRegister:
//			if _, ok := h.Clients[cli]; ok {
//				delete(h.Clients, cli)
//				close(cli.Send)
//			}
//		case message := <-h.Broadcast:
//			for cli := range h.Clients {
//				select {
//				case cli.Send <- message:
//				default:
//					close(cli.Send)
//					delete(h.Clients, cli)
//				}
//			}
//		}
//	}
//}