package main

import (
	"bytes"
	"compress/zlib"
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

	conn_pools            []net.Conn
	Security_password     string = "01dbc809-af12-5a28-b5b9-5341e2ca2198"
	Addr_websocket_listen string = ":3389"
	Addr_ss               string = "127.0.0.1:8135"

	//debug
	// key []byte = []byte("sdf44w5ef784478468sdf")
)

func main() {

	http.HandleFunc("/ws", wsHander)

	//未加密
	// http.ListenAndServe(ws_listen_addr, nil)

	//加密
	http.ListenAndServeTLS(Addr_websocket_listen, "server.crt", "server.key", nil)
}

func wsHander(w http.ResponseWriter, r *http.Request) {
	var (
		conn_websocket *websocket.Conn
		conn_server    net.Conn
		wg             sync.WaitGroup
		err            error
	)

	log.Println("等待客户端的接入")

	if conn_websocket, err = upgrader.Upgrade(w, r, nil); err != nil {
		log.Println(err)
		return
	}
	defer conn_websocket.Close()

	log.Println("有客户端连接成功:", &conn_websocket)

	//连接ss服务器
	if conn_server, err = net.Dial("tcp", Addr_ss); err != nil {
		log.Println(err)
		return
	}
	defer conn_server.Close()

	//读写数据
	wg.Add(2)
	go StreamClientToServer(conn_websocket, conn_server, &wg)
	go StreamServerToClient(conn_websocket, conn_server, &wg)
	wg.Wait()

	log.Println("------------ 释放内存 ----------------")

}

// 数据流 websocket -> ss
func StreamClientToServer(client *websocket.Conn, server net.Conn, wg *sync.WaitGroup) {
	var (
		buff  []byte
		cache []byte
		err   error
	)

	defer wg.Done()

	// log.Println("开始读数据 websocket->ss.....")
	for {
		//step1: 从 websocket 读取数据
		if _, buff, err = client.ReadMessage(); err != nil {
			log.Println("Websocket读数据错误:", err)
			break
		}

		//debug
		// log.Println("收到来自websocket ->ss消息:", buff)
		//数据处理
		cache = DoZlibUnCompress(buff)
		cache = AesDecryptECB(cache, GetNewPassword(Security_password))

		//step2: 将数据写入ss中
		if _, err = server.Write(cache); err != nil {
			log.Println(err)
			break
		}
	}

}

//数据流 ss -> websocket
func StreamServerToClient(client *websocket.Conn, server net.Conn, wg *sync.WaitGroup) {
	var (
		n     int = -1
		err   error
		buff  []byte
		cache []byte
	)

	defer wg.Done()
	// log.Println("开始写数据 ss->websocket.....")

	buff = make([]byte, 3*1024)

	//step1:从客户端读取数据
	for {
		if n, err = server.Read(buff); n == 0 || err == io.EOF {
			log.Println("SS 数据读取完成")
			time.Sleep(time.Second * 5)
			server.Close()
			client.Close()
			break
		}

		//debug:打印信息
		// log.Println("收到ss->shadowsocks的消息", buff[:n])
		//数据处理
		cache = AesEncryptECB(buff[:n], GetNewPassword(Security_password))
		cache = DoZlibCompress(cache)

		//step2:将数据写入服务端
		if err = client.WriteMessage(websocket.TextMessage, cache); err != nil {
			log.Println("写出现问题:", err)

		}
	}

}

// =================== 数据压缩 ======================
//进行zlib压缩
func DoZlibCompress(src []byte) []byte {
	var in bytes.Buffer
	w := zlib.NewWriter(&in)
	w.Write(src)
	w.Close()
	return in.Bytes()
}

//进行zlib解压缩
func DoZlibUnCompress(compressSrc []byte) []byte {
	b := bytes.NewReader(compressSrc)
	var out bytes.Buffer
	r, _ := zlib.NewReader(b)
	io.Copy(&out, r)
	return out.Bytes()
}

// =================== 动态密码 ======================

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
