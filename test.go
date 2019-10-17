package main

import (
	"log"
	"net"
)

func main() {
	var (
		conn net.Conn
		err  error
	)
	if conn, err = net.Dial("tcp", "127.0.0.1:6000"); err != nil {
		log.Println(err)
	}
	defer conn.Close()

	log.Println("连接成功", &conn)

	buff := []byte("this thissdfdf ")

	conn.Write(buff)

}
