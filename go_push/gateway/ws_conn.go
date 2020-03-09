package gateway

import (
	"errors"
	"github.com/gorilla/websocket"
	uuid "github.com/satori/go.uuid"
	"sync"
	"time"
)

type WsConnection struct {
	mu        sync.Mutex
	connId    string
	ws        *websocket.Conn
	readChan  chan *WSMessage
	writeChan chan *WSMessage
	closeChan chan bool
	isOpen    bool
	addRoom   *sync.Map
	clientIp  string
}

type WSMessage struct {
	Type int
	Data []byte
}

var (
	WsErrConnLoss = errors.New("conn already close")
)

func NewWsConnection(conn *websocket.Conn) *WsConnection {
	ws := &WsConnection{}
	ws.ws = conn
	ws.readChan = make(chan *WSMessage, 10)
	ws.writeChan = make(chan *WSMessage, 10)
	ws.closeChan = make(chan bool)
	ws.addRoom = new(sync.Map)
	ws.connId = uuid.NewV5(uuid.Must(uuid.NewV4()), "ws").String()
	go ws.read()
	go ws.send()
	return ws
}
func (w *WsConnection) SetIp(ip string) {
	w.clientIp = ip
}
func (w *WsConnection) GetIp() string {
	return w.clientIp
}

func (w *WsConnection) GetWsId() string {
	return w.connId
}

func (w *WsConnection) read() {
	var (
		Type int
		Data []byte
		err  error
	)
	w.ws.SetReadLimit(1024)
	_ = w.ws.SetReadDeadline(time.Now().Add(time.Second * 10))
	w.ws.SetPongHandler(func(string) error { _ = w.ws.SetReadDeadline(time.Now().Add(time.Second * 10)); return nil })
	defer w.close()
	for {
		if Type, Data, err = w.ws.ReadMessage(); err != nil {
			w.close()
		}
		message := &WSMessage{
			Type: Type,
			Data: Data,
		}
		select {
		case w.readChan <- message:
		case <-w.closeChan:
			return
		}
	}
}
func (w *WsConnection) send() {
	var (
		err     error
		message *WSMessage
	)
	for {
		select {
		case message = <-w.writeChan:
			if err = w.ws.WriteMessage(message.Type, message.Data); err != nil {
				w.close()
			}
		case <-w.closeChan:
			return
		}
	}
}

func (w *WsConnection) ReadMsg() (message *WSMessage, err error) {
	select {
	case message = <-w.readChan:
	case <-w.closeChan:
		err = WsErrConnLoss
	}
	return
}

func (w *WsConnection) SendMsg(msg *WSMessage) (err error) {
	select {
	case w.writeChan <- msg:
	case <-w.closeChan:
		err = WsErrConnLoss
	}
	return
}

func (w *WsConnection) close() {
	_ = w.ws.Close()
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.isOpen {
		w.isOpen = false
		close(w.closeChan)
	}
}