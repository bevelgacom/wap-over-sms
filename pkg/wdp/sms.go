package wdp

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"math"
	"net"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/bevelgacom/wap-over-sms/pkg/kannel"
	"golang.org/x/sys/unix"
)

// right now this is one per process
// the Nokia 7110 is terrible at chosing random source ports that is always the same
// in the future we might consier using virtual IPs to get around this
var udpMutex = &sync.Mutex{}

type WDPGateway struct {
	WapBoxHost string

	smsBox *kannel.SMSBox
}

type UDH struct {
	HeaderLen byte
	Ei        byte
	EiLength  byte
	Source    uint16
	Dest      uint16
}

func NewWDPGateway(wapBoxHost string, smsbox *kannel.SMSBox) *WDPGateway {
	return &WDPGateway{
		WapBoxHost: wapBoxHost,

		smsBox: smsbox,
	}
}

func (w *WDPGateway) parseUDH(udh []byte) (UDH, error) {
	if len(udh) < 7 {
		log.Println(hex.EncodeToString(udh))
		log.Println(len(udh))

		return UDH{}, errors.New("invalid udh")
	}

	return UDH{
		HeaderLen: udh[0],
		Ei:        udh[1],
		EiLength:  udh[2],
		Source:    binary.BigEndian.Uint16([]byte{udh[3], udh[4]}),
		Dest:      binary.BigEndian.Uint16([]byte{udh[5], udh[6]}),
	}, nil
}

func (w *WDPGateway) HandleIncomingSMS(from string, udh []byte, body []byte) error {
	udhData, err := w.parseUDH(udh)
	if err != nil {
		return err
	}

	udpMutex.Lock()
	conn, err := w.spawnWDPConnection(udhData, from, 60*time.Second)
	if err != nil {
		return err
	}

	destAddr := net.UDPAddr{
		Port: int(udhData.Source),
		IP:   net.ParseIP(w.WapBoxHost),
	}

	_, err = conn.WriteToUDP(body, &destAddr)
	if err != nil {
		return err
	}
	log.Printf("Sent %d bytes to %s:%d\n", len(body), w.WapBoxHost, destAddr.Port)

	return nil
}

var lc = net.ListenConfig{
	Control: func(network, address string, c syscall.RawConn) error {
		var opErr error
		err := c.Control(func(fd uintptr) {
			opErr = unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_REUSEPORT, 1)
		})
		if err != nil {
			return err
		}
		return opErr
	},
}

func (w *WDPGateway) spawnWDPConnection(udhData UDH, phoneNumber string, ttl time.Duration) (*net.UDPConn, error) {
	ctx, _ := context.WithTimeout(context.Background(), ttl)

	conn, err := net.ListenUDP("udp", &net.UDPAddr{
		IP:   net.IPv4zero,
		Port: int(udhData.Source),
	})
	if err != nil {
		// If we can't dial, we use SO_REUSEPORT to listen on the same port
		lp, err := lc.ListenPacket(context.Background(), "udp", fmt.Sprintf("0.0.0.0:%d", udhData.Dest))
		if err != nil {
			return nil, err
		}
		conn = lp.(*net.UDPConn)
	}
	conn.SetDeadline(time.Now().Add(3 * time.Second))

	go w.listenAndRelay(ctx, conn, udhData, phoneNumber)

	return conn, nil
}

func (w *WDPGateway) listenAndRelay(ctx context.Context, conn *net.UDPConn, udhData UDH, phoneNumber string) {
	defer udpMutex.Unlock()
	defer conn.Close()
	for {
		select {
		case <-ctx.Done():
			return
		default:
			buf := make([]byte, 65535)
			n, _, err := conn.ReadFromUDP(buf)
			if err != nil {
				log.Printf("Error reading from UDP: %v, closing socket\n", err)
				return
			}

			data := buf[:n]
			sms := w.generateUDHWapOverSMS(udhData.Source, udhData.Dest, data)
			log.Printf("Sending %d bytes to %s\n", len(data), phoneNumber)

			c := 0
			for _, msg := range strings.Split(sms, "\n") {
				if msg == "" {
					continue
				}
				c++
				err = w.smsBox.SendUDHSMS(phoneNumber, msg)
				if err != nil {
					log.Printf("Error sending SMS to %s: %v\n", phoneNumber, err)
					return
				}
			}

			log.Printf("Sent %d SMS to %s\n", c, phoneNumber)
		}

	}
}

func (w *WDPGateway) generateUDHWapOverSMS(sourcePort, destinationPort uint16, data []byte) string {
	const maxBytesPerMessage = 110
	var sms string

	if len(data) > maxBytesPerMessage {
		totalSMS := int(math.Ceil(float64(len(data)) / maxBytesPerMessage))

		for currentSMS := 1; currentSMS <= totalSMS; currentSMS++ {
			header := []byte{
				0x0b,             // UDH length
				0x00,             // Information Element Identifier: Concatenated short message, 8bit reference number
				0x03,             // Information Element Data Length (always 03 for this UDH)
				0x01,             // Information Element Data: Concatenated short message reference, should be same for all parts of a message
				byte(totalSMS),   // Information Element Data: Total number of parts
				byte(currentSMS), // Information Element Data: Number of this part
				0x05,             // Application Port Addressing, 16 bit address
				0x04,             // content length
			}
			header = append(header, byte(destinationPort>>8), byte(destinationPort&0xff)) // Destination port number
			header = append(header, byte(sourcePort>>8), byte(sourcePort&0xff))           // Source port number

			start := (currentSMS - 1) * maxBytesPerMessage
			end := start + maxBytesPerMessage
			if end > len(data) {
				end = len(data)
			}

			bindata := append(header, data[start:end]...)
			sms += hex.EncodeToString(bindata) + "\n"
		}
	} else {
		header := []byte{
			0x06, // UDH length
			0x05, // Application Port Addressing, 16 bit address
			0x04, // content length
		}
		header = append(header, byte(destinationPort>>8), byte(destinationPort&0xff)) // Destination port number
		header = append(header, byte(sourcePort>>8), byte(sourcePort&0xff))           // Source port number

		bindata := append(header, data...)
		sms = hex.EncodeToString(bindata)
	}

	return sms
}
