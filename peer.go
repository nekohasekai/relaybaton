package relaybaton

import (
	"encoding/binary"
	"github.com/gorilla/websocket"
	"github.com/iyouport-org/relaybaton/config"
	log "github.com/sirupsen/logrus"
	"io"
	"sync"
)

type peer struct {
	connPool     *connectionPool
	mutexWsRead  sync.Mutex
	controlQueue chan *websocket.PreparedMessage
	messageQueue chan *websocket.PreparedMessage
	hasMessage   chan byte
	close        chan byte
	wsConn       *websocket.Conn
	conf         config.MainConfig
}

func (peer *peer) init(conf config.MainConfig) {
	peer.hasMessage = make(chan byte, 2^32+2^16)
	peer.controlQueue = make(chan *websocket.PreparedMessage, 2^16)
	peer.messageQueue = make(chan *websocket.PreparedMessage, 2^32)
	peer.connPool = newConnectionPool()
	peer.close = make(chan byte, 10)
	peer.conf = conf
}

func (peer *peer) forward(session uint16) {
	wsw := peer.getWebsocketWriter(session)
	conn := peer.connPool.get(session)
	_, err := io.Copy(wsw, *conn)
	if err != nil {
		log.Error(err)
	}
	peer.connPool.delete(session)
	_, err = wsw.writeClose()
	if err != nil {
		log.WithField("session", session).Error(err)
	}
}

func (peer *peer) receive(session uint16, data []byte) {
	wsw := peer.getWebsocketWriter(session)
	conn := peer.connPool.get(session)
	if conn == nil {
		if peer.connPool.isCloseSent(session) {
			return
		}
		log.WithField("session", session).Debug("deleted connection read")
		_, err := wsw.writeClose()
		if err != nil {
			log.Error(err)
		}
		return
	}
	_, err := (*conn).Write(data)
	if err != nil {
		log.WithField("session", session).Error(err)
		peer.connPool.delete(session)
		_, err = wsw.writeClose()
		if err != nil {
			log.Error(err)
		}
	}
}

func (peer *peer) delete(session uint16) {
	conn := peer.connPool.get(session)
	if conn != nil {
		peer.connPool.delete(session)
		log.Debugf("Port %d Deleted", session)
	}
	peer.connPool.setCloseSent(session)
}

func (peer *peer) getWebsocketWriter(session uint16) webSocketWriter {
	return webSocketWriter{
		session: session,
		peer:    peer,
	}
}

func (peer *peer) processQueue() {
	for {
		select {
		case <-peer.close:
			return
		default:
			<-peer.hasMessage
			if (len(peer.hasMessage)+1)%50 == 0 {
				log.WithField("len", len(peer.hasMessage)+1).Debug("Message Length") //test
			}
			if len(peer.controlQueue) > 0 {
				err := peer.wsConn.WritePreparedMessage(<-peer.controlQueue)
				if err != nil {
					log.Error(err)
					err = peer.Close()
					if err != nil {
						log.Debug(err)
					}
					return
				}
			} else {
				err := peer.wsConn.WritePreparedMessage(<-peer.messageQueue)
				if err != nil {
					log.Error(err)
					err = peer.Close()
					if err != nil {
						log.Debug(err)
					}
					return
				}
			}
		}
	}
}

func (peer *peer) Close() error {
	if len(peer.close) > 0 {
		return nil
	}
	log.Debug("closing peer")
	peer.close <- 0
	peer.close <- 1
	peer.close <- 2
	err := peer.wsConn.Close()
	if err != nil {
		log.Debug(err)
	}
	err = peer.conf.DB.DB.Close()
	if err != nil {
		log.Debug(err)
	}
	peer.close <- 1
	for i := uint16(0); i < 65535; i++ {
		if peer.connPool.get(i) != nil {
			peer.connPool.delete(i)
		}
	}
	return err
}

func uint16ToBytes(n uint16) []byte {
	buf := make([]byte, 2)
	binary.BigEndian.PutUint16(buf, n)
	return buf
}
