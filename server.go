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
	key string = "sdf44w5ef784478468sdf"
)

func main() {
	http.HandleFunc("/ws", wsHander)
	http.ListenAndServe(":137", nil)
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
	if conn, err = net.Dial("tcp", "127.0.0.1:8135"); err != nil {
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
			if _, err = server.Write(AesDecryptECB(data, GetNewPassword(key))); err != nil {
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
		if err = client.WriteMessage(websocket.TextMessage, AesEncryptECB(buff[:n], GetNewPassword(key))); err != nil {
			log.Println("写出现问题:", err)
			break
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
