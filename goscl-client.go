package main

import (
	"crypto/aes"
	"crypto/md5"
	"encoding/hex"
	"io"
	"log"
	"net"

	// "net/http"
	// _ "net/http/pprof" // 性能分析包引入
	"strconv"
	"sync"
	"time"

	"./github.com/gorilla/websocket"
)

var (
	wss_conn_pools []*websocket.Conn

	key         string = "sdf44w5ef784478468sdf"
	listen_addr string = "127.0.0.1:6000"
	ws_addr     string = "wss://server.oneso.win:3389/ws"
	// ws_addr string = "wss://127.0.0.1:3389/ws"
)

func main() {
	var (
		conn net.Conn

		listener net.Listener
		err      error
	)

	//性能分析 http://127.0.0.1:6060/debug/pprof/
	// go func() {
	// 	log.Println(http.ListenAndServe("localhost:6060", nil))
	// }()

	if listener, err = net.Listen("tcp", listen_addr); err != nil {
		log.Println(err)
		return
	}

	defer listener.Close()

	//返回链接变脏数据
	// go reduceConn()

	// go lookupConnetct()

	for {
		if conn, err = listener.Accept(); err != nil {
			log.Println(err)
			continue
		}
		log.Println("有客户端接入:", &conn)
		go handleRequest(conn)
	}

}

//处理请求
func handleRequest(client_conn net.Conn) {

	var (
		wg       sync.WaitGroup
		web_conn *websocket.Conn
		err      error
	)
	defer client_conn.Close()

	log.Println()
	log.Println("开始连接 websocket 服务器....")

	//防止创建失败

	if web_conn, err = GetWebsocketConn(); err != nil {
		log.Println(err)

	}

	defer web_conn.Close()

	log.Println("连接 websocket 成功:", &web_conn)

	wg.Add(2)
	go readData(client_conn, web_conn, &wg)
	go writeData(client_conn, web_conn, &wg)
	wg.Wait()
	log.Println("................. 内存释放完成............")

}

//读数据 local_ss -> websocket
func readData(client net.Conn, server *websocket.Conn, wg *sync.WaitGroup) {
	var (
		n    int = -1
		err  error
		buff []byte
	)

	defer wg.Done()
	defer client.Close()
	defer server.Close()
	defer log.Println(&client, ":读数据结束")
	buff = make([]byte, 256)

	//step1:从客户端读取数据
	for {

		if n, err = client.Read(buff); n == 0 || err == io.EOF {
			log.Println(&client, ":客户端信息读取完成")
			break
		}

		//debug:打印调试信息
		// log.Println(&client, ":读取到字节数:", n)
		// log.Println(&client, ":收到客户端的消息", buff[:n])

		//step2:将数据写入服务端
		if err = server.WriteMessage(websocket.TextMessage, AesEncryptECB(buff[:n], GetNewPassword(key))); err != nil {
			log.Println("写出现问题:", err)
			break
		}

	}

}

//写数据 websocket -> local_ss
func writeData(client net.Conn, server *websocket.Conn, wg *sync.WaitGroup) {
	var (
		err  error
		buff []byte
	)
	defer wg.Done()
	defer client.Close()
	defer server.Close()
	defer log.Println(&client, ":写数据结束")
	for {

		//step1: 从服务端读取数据
		if _, buff, err = server.ReadMessage(); err != nil {
			log.Println(err)
			break
		}

		//debug:打印调试信息
		// log.Println(&server, ":收到来自websocket的消息:", buff)

		//step2: 将数据写入客户端
		if _, err = client.Write(AesDecryptECB(buff, GetNewPassword(key))); err != nil {
			log.Println(err)

			break
		}

	}

}

// =================== 缓存池 ======================
func GetWebsocketConn() (*websocket.Conn, error) {
	var (
		web_conn *websocket.Conn
		err      error
	)
	//自动创建连接池
	if len(wss_conn_pools) > 5 {
		wss_conn_pools = make([]*websocket.Conn, 0)
	}

	//如果没有马上创建一条
	if len(wss_conn_pools) == 0 {
		CreateWebsocket()
	}

	//为下次准备
	go CreateWebsocket()

	//取websocket conn
	web_conn = wss_conn_pools[0]
	wss_conn_pools = wss_conn_pools[1:]

	return web_conn, err
}

//减少连接
func reduceConn() {
	for {
		time.Sleep(10)
		_, _ = GetWebsocketConn()
	}
}

//创建websocket
func CreateWebsocket() {
	var (
		web_conn *websocket.Conn
		err      error
	)
	for {
		if web_conn, _, err = websocket.DefaultDialer.Dial(ws_addr, nil); err != nil {
			log.Println(err)
			continue
		}
		break
	}
	wss_conn_pools = append(wss_conn_pools, web_conn)
}

//创建连接池
func CreateWebsocketConnPools() {
	var (
		web_conn *websocket.Conn
		err      error
	)
	for i := 0; i < 10; i++ {
		if web_conn, _, err = websocket.DefaultDialer.Dial(ws_addr, nil); err != nil {
			log.Println(err)
			continue
		}
		wss_conn_pools = append(wss_conn_pools, web_conn)
	}
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
