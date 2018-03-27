package main

import (
	"net/http"
	"fmt"
	"github.com/gorilla/websocket"
	"time"
	"bytes"
	"log"
)
const (
	// 等待写消息超时设置
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// 允许最大连接数
	maxMessageSize = 512
)

var (
	newline = []byte{'\n'}
	space   = []byte{' '}
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}


type Client struct {
	Scheduler *Scheduler

	// websocket 连接
	Conn *websocket.Conn

	// Buffer chan 存储消息.
	Send chan []byte
}

type Scheduler struct {
	Clients    map[*Client]bool
	Broadcast  chan []byte  //需要广播的消息
	Register   chan *Client //刚进入到这个房间的客户端
	UnRegister chan *Client //退出到这个房间的客户端
}


//调度chan
func (h *Scheduler) Run() {
	for {
		select {
		//如果有人进入房间,加入到客户端列表
		case cli := <-h.Register:
			h.Clients[cli] = true
		//如果有人离开房间，删除信息并关闭发送
		case cli := <-h.UnRegister:
			if _, ok := h.Clients[cli]; ok {
				delete(h.Clients, cli)
				close(cli.Send)
			}
		//当有消息的时候，发送消息
		case message := <-h.Broadcast:
			for cli := range h.Clients {
				select {
				case cli.Send <- message:
				default:
					close(cli.Send)
					delete(h.Clients, cli)
				}
			}
		}
	}
}

/**
初始化 调度器
 */
func NewScheduler() *Scheduler{
	return &Scheduler{
		Broadcast:  make(chan []byte),
		Register:   make(chan *Client),
		UnRegister: make(chan *Client),
		Clients:    make(map[*Client]bool),
	}
}

func main() {
	sch := NewScheduler()
	go sch.Run()
	//获取http包中的路由分发
	mux := http.NewServeMux()
	//加入路由
	mux.HandleFunc("/",home)
	mux.HandleFunc("/ws",func (w http.ResponseWriter,r *http.Request){
	// 创建一个客户端，处理消息
	 	ServeWs(sch,w,r)
	})

	//监听8888端口，起服务
	err:=http.ListenAndServe(":8888",mux)

	if err != nil{
		panic(err.Error())
	}
}


//加载首页模板
func home(w http.ResponseWriter,r *http.Request){
	fmt.Println(r.URL.Path)
	if r.URL.Path != "/" {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	http.ServeFile(w,r,"./home.html")
}




//下边的是客户端的操作
//readMessage 客户端从websocket中读取消息并推送给其他客户端(我发的消息的时候)
func(cli *Client) readMessage() {
	//如果客户端报错,那么就直接退出并且关闭客户端
	defer func() {
		cli.Scheduler.UnRegister <-cli
		cli.Conn.Close()
	}()
	//设置读取消息的最大限制
	cli.Conn.SetReadLimit(maxMessageSize)
	//设置读超时
	cli.Conn.SetReadDeadline(time.Now().Add(pongWait))
	cli.Conn.SetPongHandler(func(string) error { cli.Conn.SetReadDeadline(time.Now().Add(pongWait)); return nil })
	for {
		_, message, err := cli.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("error: %v", err)
			}
			break
		}
		//把换行符转换成' '
		message = bytes.TrimSpace(bytes.Replace(message, newline, space, -1))
		//然后把数据发送到Scheduler的broadcast
		cli.Scheduler.Broadcast <- message
	}
}

// WriteMessage 当我获取消息的时候 是我需要发送的列表中 写到我自己的客户端
func(cli *Client) WriteMessage() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		//如果客户端报错,那么就直接退出并且关闭客户端连接
		ticker.Stop()
		cli.Conn.Close()
	}()
	for {
		select {
		case message, ok := <-cli.Send:
			//设置写超时
			cli.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				cli.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := cli.Conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			// Add queued chat messages to the current websocket message.
			n := len(cli.Send)
			for i := 0; i < n; i++ {
				w.Write(newline)
				w.Write(<-cli.Send)
			}

			if err := w.Close(); err != nil {
				return
			}
		case <-ticker.C:
			cli.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := cli.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// serveWs handles websocket requests from the peer.
func ServeWs(sch *Scheduler, w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}
	cli := &Client{Scheduler: sch, Conn: conn, Send: make(chan []byte, 256)}
	cli.Scheduler.Register <- cli

	// Allow collection of memory referenced by the caller by doing all work in
	// new goroutines.
	//go cli
	go cli.readMessage()
	go cli.WriteMessage()
}
