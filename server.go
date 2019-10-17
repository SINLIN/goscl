package main

import (
	"io"
	"net"
	"sync"

	// "io"
	"log"
	"net/http"

	"./github.com/gorilla/websocket"
)

var (
	upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}

	wg sync.WaitGroup
)

func main() {
	http.HandleFunc("/ws", wsHander)
	http.ListenAndServe(":5000", nil)
}

func wsHander(w http.ResponseWriter, r *http.Request) {
	var (
		conn_web *websocket.Conn
		conn     net.Conn

		err error
	)

	log.Println("等待客户端的接入")
	if conn_web, err = upgrader.Upgrade(w, r, nil); err != nil {
		log.Println(err)
		return
	}

	defer conn_web.Close()
	log.Println("有客户端连接成功:", &conn)

	//连接服务器
	if conn, err = net.Dial("tcp", "192.168.1.80:8135"); err != nil {
		log.Println(err)
		return
	}
	defer conn.Close()

	//读写数据
	wg.Add(2)
	go readData(conn_web, conn)
	go writeData(conn_web, conn)
	wg.Wait()

}

// 读数据
func readData(client *websocket.Conn, server net.Conn) {
	var (
		n    int
		data []byte
		err  error
	)

	defer wg.Done()

	for {
		//读数据
		if n, data, err = client.ReadMessage(); n == 0 || err == io.EOF {
			log.Println("读数据错误", err)
			return
		}

		// log.Println("收到来自websocket消息:", string(data))
		if len(data) > 0 {
			if _, err = server.Write(data); err != nil {
				log.Println(err)
				return
			}
		}

	}
}

//写数据
func writeData(client *websocket.Conn, server net.Conn) {
	var (
		n    int
		err  error
		buff []byte
	)

	defer wg.Done()

	buff = make([]byte, 2048)

	for {
		//step1:从客户端读取数据
		if n, err = server.Read(buff); n == 0 || err == io.EOF {
			log.Println("客户端信息读取完成")
			break
		}

		//debug:打印信息
		// log.Println("收到shadowsocks的消息", string(buff[:n]))

		//step2:将数据写入服务端
		if err = client.WriteMessage(websocket.TextMessage, buff[:n]); err != nil {
			log.Println("写出现问题:", err)
			break
		}
	}
}
