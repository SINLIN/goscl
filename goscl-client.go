package main

import (
	"crypto/aes"
	"crypto/md5"
	"encoding/hex"
	"io"
	"log"
	"net"
	"strconv"
	"sync"
	"time"

	"./github.com/gorilla/websocket"
)

var (
	wg sync.WaitGroup
	// key []byte = []byte("sdf44w5ef784478468sdf")
	key         string = "sdf44w5ef784478468sdf"
	listen_addr string = "127.0.0.1:6000"
	ws_addr     string = "ws://162.209.149.81:3389/ws"
)

func main() {
	var (
		conn net.Conn

		listener net.Listener
		err      error
	)

	if listener, err = net.Listen("tcp", listen_addr); err != nil {
		log.Println(err)
		return
	}

	defer listener.Close()

	go lookupConnetct()

	for {
		if conn, err = listener.Accept(); err != nil {
			log.Println(err)
			continue
		}
		log.Println("有客户端接入:", &conn)
		go handleRequest(conn)
	}

}

//循环绑定
func lookupConnetct() {
	var (
		web_conn *websocket.Conn
		err      error
	)
	for {
		if web_conn, _, err = websocket.DefaultDialer.Dial(ws_addr, nil); err != nil {
			time.Sleep(time.Second * 3)
			continue
		}

		time.Sleep(time.Second * 3)
		web_conn.Close()
	}
}

//处理请求
func handleRequest(conn net.Conn) {

	var (
		web_conn *websocket.Conn
		err      error
	)
	defer conn.Close()

	log.Println("开始连接 websocket 服务器....")
	//建立websocket 连接
	if web_conn, _, err = websocket.DefaultDialer.Dial(ws_addr, nil); err != nil {
		log.Println(err)
		return
	}

	defer web_conn.Close()
	log.Println("连接 websocket 成功:", &web_conn)
	wg.Add(2)
	go readData(conn, web_conn)
	go writeData(conn, web_conn)
	wg.Wait()

}

//读客户端数据到服务端
func readData(client net.Conn, server *websocket.Conn) {

	var (
		n    int
		err  error
		buff []byte
	)
	defer wg.Done()

	buff = make([]byte, 2048)

	for {
		//step1:从客户端读取数据
		if n, err = client.Read(buff); n == 0 || err == io.EOF {
			log.Println("客户端信息读取完成")
			break
		}

		//debug:打印信息
		// log.Println("收到客户端的消息", string(buff[:n]))

		//step2:将数据写入服务端
		if err = server.WriteMessage(websocket.TextMessage, AesEncryptECB(buff[:n], GetNewPassword(key))); err != nil {
			log.Println("写出现问题:", err)
			break
		}

	}
}

//读服务端数据到客户端
func writeData(client net.Conn, server *websocket.Conn) {
	var (
		n    int
		err  error
		buff []byte
	)
	defer wg.Done()

	for {
		//step1: 从服务端读取数据
		if n, buff, err = server.ReadMessage(); n == 0 || err != nil {
			log.Println("读取来自 websocket 消息完成")
			return
		}

		//debug
		// log.Println("收到来自websocket的消息:", string(buff))

		//step2: 将数据写入客户端
		if len(buff) > 0 {
			if _, err = client.Write(AesDecryptECB(buff, GetNewPassword(key))); err != nil {
				log.Println(err)
				return
			}
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
