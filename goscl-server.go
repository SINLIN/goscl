package main

import (
	"crypto/md5"
	"encoding/hex"
	"io"
	"net"
	"strconv"
	"sync"
	"time"

	"crypto/aes"
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

	// key []byte = []byte("sdf44w5ef784478468sdf")
	key            string = "sdf44w5ef784478468sdf"
	ws_listen_addr string = ":3389"
	ss_addr        string = "127.0.0.1:8135"
)

func main() {
	http.HandleFunc("/ws", wsHander)
	// http.ListenAndServe(ws_listen_addr, nil)
	http.ListenAndServeTLS(ws_listen_addr, "server.crt", "server.key", nil)
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
	if conn, err = net.Dial("tcp", ss_addr); err != nil {
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

// 读数据 websocket -> ss
func readData(client *websocket.Conn, server net.Conn) {
	var (
		buff []byte
		err  error
	)

	defer wg.Done()

	// log.Println("开始读数据 websocket->ss.....")
	for {
		//step1: 从 websocket 读取数据
		if _, buff, err = client.ReadMessage(); err != nil {
			log.Println("读数据错误", err)
			return
		}

		// log.Println("收到来自websocket ->ss消息:", buff)

		//step2: 将数据写入ss中
		if _, err = server.Write(AesDecryptECB(buff, GetNewPassword(key))); err != nil {
			log.Println(err)
			return
		}
	}

}

//写数据 ss -> websocket
func writeData(client *websocket.Conn, server net.Conn) {
	var (
		n    int = -1
		err  error
		buff []byte
	)

	defer wg.Done()
	// log.Println("开始写数据 ss->websocket.....")

	buff = make([]byte, 20*1024)

	//step1:从客户端读取数据
	for {
		if n, err = server.Read(buff); n == 0 || err == io.EOF {
			log.Println("客户端信息读取完成")
			break
		}

		//debug:打印信息
		// log.Println("收到ss->shadowsocks的消息", buff[:n])

		//step2:将数据写入服务端
		if err = client.WriteMessage(websocket.TextMessage, AesEncryptECB(buff[:n], GetNewPassword(key))); err != nil {
			log.Println("写出现问题:", err)

		}
	}

}

//每天生成新的密码
func GetNewPassword(key string) []byte {
	str := strconv.Itoa(time.Now().Day()) + key
	h := md5.New()
	h.Write([]byte(str))
	return []byte(hex.EncodeToString(h.Sum(nil)))

}

// =================== ECB ======================
func AesEncryptECB(origData []byte, key []byte) (encrypted []byte) {
	cipher, _ := aes.NewCipher(generateKey(key))
	length := (len(origData) + aes.BlockSize) / aes.BlockSize
	plain := make([]byte, length*aes.BlockSize)
	copy(plain, origData)
	pad := byte(len(plain) - len(origData))
	for i := len(origData); i < len(plain); i++ {
		plain[i] = pad
	}
	encrypted = make([]byte, len(plain))
	// 分组分块加密
	for bs, be := 0, cipher.BlockSize(); bs <= len(origData); bs, be = bs+cipher.BlockSize(), be+cipher.BlockSize() {
		cipher.Encrypt(encrypted[bs:be], plain[bs:be])
	}

	return encrypted
}
func AesDecryptECB(encrypted []byte, key []byte) (decrypted []byte) {
	cipher, _ := aes.NewCipher(generateKey(key))
	decrypted = make([]byte, len(encrypted))
	//
	for bs, be := 0, cipher.BlockSize(); bs < len(encrypted); bs, be = bs+cipher.BlockSize(), be+cipher.BlockSize() {
		cipher.Decrypt(decrypted[bs:be], encrypted[bs:be])
	}

	trim := 0
	if len(decrypted) > 0 {
		trim = len(decrypted) - int(decrypted[len(decrypted)-1])
	}

	return decrypted[:trim]
}
func generateKey(key []byte) (genKey []byte) {
	genKey = make([]byte, 16)
	copy(genKey, key)
	for i := 16; i < len(key); {
		for j := 0; j < 16 && i < len(key); j, i = j+1, i+1 {
			genKey[j] ^= key[i]
		}
	}
	return genKey
}
