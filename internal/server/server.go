// TODO: document the package.
package server

import (
	"io"
	"log"
	"net"
)

func Start(port string) error {
	l, err := net.Listen("tcp", ":1337")
	if err != nil {
		return err
	}
	defer l.Close()

	for {
		conn, err := l.Accept()
		if err != nil {
			return err
		}

		go func(c net.Conn) {
			if _, err := io.Copy(c, c); err != nil {
				log.Println(err)
			}
			c.Close()
		}(conn)
	}
}
