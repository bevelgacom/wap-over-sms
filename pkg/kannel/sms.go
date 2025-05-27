package kannel

import (
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"time"
)

type SMSBox struct {
	KannelURL string
	Username  string
	Password  string

	SenderMSN string
	SMSC      string
}

func NewSMSBox(username, password, kannelURL, senderMSN, smsc string) *SMSBox {
	return &SMSBox{
		Username:  username,
		Password:  password,
		KannelURL: kannelURL,
		SenderMSN: senderMSN,
		SMSC:      smsc,
	}
}

func (s *SMSBox) SendUDHSMS(to, dataStr string) error {
	data, err := hex.DecodeString(dataStr)
	if err != nil {
		return err
	}

	if len(data) < 1 {
		return fmt.Errorf("invalid data: %s", dataStr)
	}

	udhHeaderLength := data[0] + 1
	udhHeader := data[:udhHeaderLength]
	content := data[udhHeaderLength:]

	return s.sendSMS(to, string(udhHeader), string(content))
}

func (s *SMSBox) sendSMS(to string, udhHeader string, content string) error {
	data := make(url.Values)

	data["username"] = []string{s.Username}
	data["password"] = []string{s.Password}
	data["from"] = []string{s.SenderMSN}
	data["smsc"] = []string{s.SMSC}
	data["to"] = []string{to}
	data["udh"] = []string{udhHeader}
	data["text"] = []string{content}

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	reqURL := fmt.Sprintf("http://%s/cgi-bin/sendsms?%s", s.KannelURL, url.Values(data).Encode())
	resp, err := client.Get(reqURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	log.Printf("Response from Kannel: %s\n", string(body))

	return nil
}
