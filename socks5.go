// This is a simple SOCKS5 proxy server.
// Copyright 2013-2015, physacco. Distributed under the MIT license.

package main

import (
    "io"
    "os"
    "fmt"
    "log"
    "net"
    "strings"
    "strconv"
)

var LISTEN string  // listen address, e.g. 0.0.0.0:1080

// Convert a IP:Port string to a byte array in network order.
// e.g.: 74.125.31.104:80 -> [74 125 31 104 0 80]
func packNetAddr(addr net.Addr, buf []byte) {
    ipport := addr.String()
    pair := strings.Split(ipport, ":")
    ipstr, portstr := pair[0], pair[1]
    port, err := strconv.Atoi(portstr)
    if err != nil {
        panic(fmt.Sprintf("invalid address %s", ipport))
    }

    copy(buf[:4], net.ParseIP(ipstr).To4())
    buf[4] = byte(port / 256)
    buf[5] = byte(port % 256)
}

func isUseOfClosedConn(err error) bool {
    operr, ok := err.(*net.OpError)
    return ok && operr.Err.Error() == "use of closed network connection"
}

func iobridge(src io.Reader, dst io.Writer, shutdown chan bool) {
    defer func() {
        shutdown <- true
    }()

    buf := make([]byte, 8192)
    for {
        n, err := src.Read(buf)
        if err != nil {
            if !(err == io.EOF || isUseOfClosedConn(err)) {
                log.Printf("error reading %s: %s\n", src, err)
            }
            break
        }

        _, err = dst.Write(buf[:n])
        if err != nil {
            log.Printf("error writing %s: %s\n", src, err)
            break
        }
    }
}

// Read a specified number of bytes.
func readBytes(conn io.Reader, count int) (buf []byte) {
    buf = make([]byte, count)
    if _, err := io.ReadFull(conn, buf); err != nil {
        panic(err)
    }
    return
}

func protocolCheck(assert bool) {
    if !assert {
        panic("protocol error")
    }
}

func errorReplyConnect(reason byte) []byte {
    return []byte{0x05, reason, 0x00, 0x01,
                  0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
}

func performConnect(backend string, frontconn net.Conn) {
    log.Printf("trying to connect to %s...\n", backend)
    backconn, err := net.Dial("tcp", backend)
    if err != nil {
        log.Printf("failed to connect to %s: %s\n", backend, err)
        frontconn.Write(errorReplyConnect(0x05))
        return
    }

    backaddr := backconn.RemoteAddr().String()
    log.Println("CONNECTED backend", backconn, backaddr)
    defer func() {
        backconn.Close()
        log.Println("DISCONNECTED backend", backconn, backaddr)
    }()

    // reply to the CONNECT command
    buf := make([]byte, 10)
    copy(buf, []byte{0x05, 0x00, 0x00, 0x01})
    packNetAddr(backconn.RemoteAddr(), buf[4:])
    frontconn.Write(buf)

    // bridge connection
    shutdown := make(chan bool, 2)
    go iobridge(frontconn, backconn, shutdown)
    go iobridge(backconn, frontconn, shutdown)

    // wait for either side to close
    <-shutdown
}

func handleConnection(frontconn net.Conn) {
    frontaddr := frontconn.RemoteAddr().String()
    log.Println("ACCEPTED frontend", frontconn, frontaddr)
    defer func() {
        if err := recover(); err != nil {
            log.Println("ERROR frontend", frontconn, frontaddr, err)
        }
        frontconn.Close()
        log.Println("DISCONNECTED frontend", frontconn, frontaddr)
    }()

    // receive auth packet
    buf1 := readBytes(frontconn, 2)
    protocolCheck(buf1[0] == 0x05)  // VER

    nom := int(buf1[1])  // number of methods
    methods := readBytes(frontconn, nom)

    var support bool
    for _, meth := range methods {
        if meth == 0x00 {
            support = true
            break
        }
    }
    if !support {
        // X'FF' NO ACCEPTABLE METHODS
        frontconn.Write([]byte{0x05, 0xff})
        return
    }

    // X'00' NO AUTHENTICATION REQUIRED
    frontconn.Write([]byte{0x05, 0x00})

    // recv command packet
    buf3 := readBytes(frontconn, 4)
    protocolCheck(buf3[0] == 0x05)  // VER
    protocolCheck(buf3[2] == 0x00)  // RSV

    command := buf3[1]
    if command != 0x01 {  // 0x01: CONNECT
        // X'07' Command not supported
        frontconn.Write(errorReplyConnect(0x07))
        return
    }

    addrtype := buf3[3]
    if addrtype != 0x01 && addrtype != 0x03 {
        // X'08' Address type not supported
        frontconn.Write(errorReplyConnect(0x08))
        return
    }

    var backend string
    if addrtype == 0x01 {  // 0x01: IP V4 address
        buf4 := readBytes(frontconn, 6)
        backend = fmt.Sprintf("%d.%d.%d.%d:%d", buf4[0], buf4[1],
            buf4[2], buf4[3], int(buf4[4]) * 256 + int(buf4[5]))
    } else {  // 0x03: DOMAINNAME
        buf4 := readBytes(frontconn, 1)
        nmlen := int(buf4[0])  // domain name length
        if nmlen > 253 {
            panic("domain name too long")  // will be recovered
        }

        buf5 := readBytes(frontconn, nmlen + 2)
        backend = fmt.Sprintf("%s:%d", buf5[0:nmlen],
            int(buf5[nmlen]) * 256 + int(buf5[nmlen+1]))
    }

    performConnect(backend, frontconn)
}

func ListenAndServe() {
    listener, err := net.Listen("tcp", LISTEN)
    if err != nil {
        log.Fatal("Listen error: ", err)
    }
    log.Printf("Listening on %s...\n", LISTEN)

    for {
        conn, err := listener.Accept()
        if err != nil {
            log.Println("Accept error:", err)
            continue
        }
        go handleConnection(conn)
    }
}

func main() {
    if len(os.Args) < 2 {
        fmt.Println("Usage: socks5 LISTEN")
        return
    }

    LISTEN = os.Args[1]

    ListenAndServe()
}
