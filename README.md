# socks5

This is an extremely fast, clean and easy-to-use [SOCKS5](https://en.wikipedia.org/wiki/SOCKS) proxy server written in Go.

## Installation

It is assumed that you have set [GOPATH](https://github.com/golang/go/wiki/GOPATH) in your environment and added `$GOPATH/bin` to your **PATH**. Then you can use the following command to automatically download, compile and install this software:

    go get github.com/physacco/socks5

If all goes well, there will be a newly compiled executable file named _socks5_ (or _socks5.exe_ on Windows platform) in your `$GOPATH/bin` directory.

## Usage

Start a SOCKS5 server:

    socks5 [host]:port

To stop the server:
* just press _Ctrl-C_, or
* kill the process with this command: `pkill -f socks5`

## Examples

1. Listening on _\*:1080_:

    socks5 :1080

2. Listen on _localhost:1080_

    socks5 localhost:1080

