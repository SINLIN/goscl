package main

import (
	"crypto/aes"
	"crypto/md5"
	"encoding/hex"
	"io"
	"log"
	"net"
	"net/http"

	// "net/http"
	// _ "net/http/pprof" // 性能分析包引入
	"strconv"
	"sync"
	"time"

	"./github.com/gorilla/websocket"
)

var (
	//Websocket 连接池
	Conn_pools_websocket chan *websocket.Conn = make(chan *websocket.Conn, 5)

	//安全密钥 -> 用于再次加密和解密
	Security_password string = "01dbc809-af12-5a28-b5b9-5341e2ca2198"

	//本地监听地址
	Addr_socks_listen string = "127.0.0.1:6000"

	//远程 Webscoket 连接地址
	Addr_remote_websocket_path string = "wss://server.oneso.win:3389/ws?transport=websocket&uuid=32"

	//debug
	//ws_addr string = "wss://127.0.0.1:3389/ws"
)

func main() {
	var (
		listener    net.Listener
		conn_client net.Conn
		err         error
	)

	//性能分析 http://127.0.0.1:6060/debug/pprof/
	// go func() {
	// 	log.Println(http.ListenAndServe("localhost:6060", nil))
	// }()

	//绑定监听端口
	if listener, err = net.Listen("tcp", Addr_socks_listen); err != nil {
		log.Println(err)
		return
	}

	defer listener.Close()

	//创建 Websocket 连接：备用
	go CreateWebsocketConn()

	//接收来自客户端的连接并处理
	for {
		if conn_client, err = listener.Accept(); err != nil {
			log.Println(err)
			continue
		}
		log.Println("有客户端接入:", &conn_client)

		go HandleClientRequest(conn_client)

	}

}

//处理客户端请求
func HandleClientRequest(conn_client net.Conn) {
	var (
		wg             sync.WaitGroup
		conn_websocket *websocket.Conn
		err            error
	)
	defer conn_client.Close()

	log.Println()
	log.Println("开始连接 websocket 服务器....")

	//获取 Websocket conn
	if conn_websocket, err = GetWebsocketConn(); err != nil {
		log.Println(err)

	}
	defer conn_websocket.Close()

	log.Println("连接 websocket 成功:", &conn_websocket)

	//读写
	wg.Add(2)
	go StreamClientToServer(conn_client, conn_websocket, &wg)
	go StreamServerToClient(conn_client, conn_websocket, &wg)
	wg.Wait()

	//读写完成 -> 回收内存
	log.Println("................. 内存释放完成............")

}

//数据流 local_ss -> websocket
func StreamClientToServer(client net.Conn, server *websocket.Conn, wg *sync.WaitGroup) {
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
		if err = server.WriteMessage(websocket.TextMessage, AesEncryptECB(buff[:n], GetNewPassword(Security_password))); err != nil {
			log.Println("写出现问题:", err)
			break
		}

	}

}

//数据流 websocket -> local_ss
func StreamServerToClient(client net.Conn, server *websocket.Conn, wg *sync.WaitGroup) {
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
		//log.Println(&server, ":收到来自websocket的消息:", buff)

		//step2: 将数据写入客户端
		if _, err = client.Write(AesDecryptECB(buff, GetNewPassword(Security_password))); err != nil {
			log.Println(err)

			break
		}

	}

}

// =================== 缓存池 ======================

//创建 Websocket 连接
func CreateWebsocketConn() {
	var (
		conn_websocket        *websocket.Conn
		header_request        http.Header = make(map[string][]string)
		cookie_userinfo       string      = "__WebsocketUserid="
		addr_websocket_remote string
		data_random           string
		id_user               string
		err                   error
	)
	for {
		//获取时间戳数据
		data_random = string(GetNewPassword(strconv.FormatInt(time.Now().UnixNano(), 16)))

		//配置用户信息
		id_user = data_random
		cookie_userinfo = data_random
		addr_websocket_remote = Addr_remote_websocket_path + id_user

		//添加请求头
		header_request["Host"] = []string{"server.oneso.win:3389"}
		header_request["Origin"] = []string{"https://oneso.win"}
		header_request["user-agent"] = []string{"Mozilla/5.0 (Windows NT 10.0; WOW64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/69.0.3497.100 Safari/537.36"}
		header_request["Cookie"] = []string{cookie_userinfo}

		//获取 Websocket conn
		for {
			if conn_websocket, _, err = websocket.DefaultDialer.Dial(addr_websocket_remote, header_request); err != nil {
				log.Println("获取Websocket失败:", err)
				continue
			}
			break
		}

		//保存到连接池
		Conn_pools_websocket <- conn_websocket

	}
}

// 获取 Websocket 连接
func GetWebsocketConn() (*websocket.Conn, error) {
	var (
		conn_websocket *websocket.Conn
		err            error
	)

	conn_websocket = <-Conn_pools_websocket

	return conn_websocket, err
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
