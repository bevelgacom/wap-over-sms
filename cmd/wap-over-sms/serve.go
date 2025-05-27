package main

import (
	"encoding/hex"
	"io"
	"log"
	"net/http"
	"net/url"

	"github.com/bevelgacom/wap-over-sms/pkg/kannel"
	"github.com/bevelgacom/wap-over-sms/pkg/wdp"
	"github.com/labstack/echo/v4"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(NewPassDownloadCommand())
}

type serveCommand struct {
	Token string

	ListenAddr string

	WAPBoxHost string

	SMSBoxUsername  string
	SMSBoxPassword  string
	SMSBoxHost      string
	SMSBoxSenderMSN string
	SMSC            string

	wdpGateway *wdp.WDPGateway
}

func NewPassDownloadCommand() *cobra.Command {
	s := serveCommand{}
	c := &cobra.Command{
		Use:     "serve",
		Short:   "Run the server",
		PreRunE: s.Validate,
		RunE:    s.RunE,
	}

	c.Flags().StringVarP(&s.Token, "token", "t", "", "Token for kannel incoming auth")

	c.Flags().StringVarP(&s.WAPBoxHost, "wapbox-host", "", "127.0.0.1", "wapbox IP")

	c.Flags().StringVarP(&s.ListenAddr, "listen-addr", "", ":8080", "Listen address")

	c.Flags().StringVarP(&s.SMSBoxUsername, "smsbox-username", "", "", "SMSBox username")
	c.Flags().StringVarP(&s.SMSBoxPassword, "smsbox-password", "", "", "SMSBox password")
	c.Flags().StringVarP(&s.SMSBoxHost, "smsbox-host", "", "", "SMSBox host")
	c.Flags().StringVarP(&s.SMSBoxSenderMSN, "smsbox-sender-msn", "", "", "SMSBox sender MSN")
	c.Flags().StringVarP(&s.SMSC, "smsc", "", "", "Which Kannel SMSC to use for sending SMS (optional)")

	c.MarkFlagRequired("token")
	c.MarkFlagRequired("smsbox-username")
	c.MarkFlagRequired("smsbox-password")
	c.MarkFlagRequired("smsbox-host")
	c.MarkFlagRequired("smsbox-sender-msn")

	return c
}

func (s *serveCommand) Validate(cmd *cobra.Command, args []string) error {
	return nil
}

func (s *serveCommand) RunE(cmd *cobra.Command, args []string) error {
	smsbox := kannel.NewSMSBox(s.SMSBoxUsername, s.SMSBoxPassword, s.SMSBoxHost, s.SMSBoxSenderMSN, s.SMSC)
	s.wdpGateway = wdp.NewWDPGateway(s.WAPBoxHost, smsbox)
	e := echo.New()
	e.GET("/", func(c echo.Context) error {
		return c.String(http.StatusOK, "WAP over SMS Gateway")
	})
	e.POST("/kannel", s.handleKannel)
	e.Logger.Fatal(e.Start(s.ListenAddr))

	return nil
}

func (s *serveCommand) handleKannel(c echo.Context) error {
	log.Println("Received request from Kannel")
	if c.QueryParam("token") != s.Token {
		log.Println("Unauthorized request")
		return c.String(http.StatusUnauthorized, "Unauthorized")
	}

	var err error
	udh := c.Request().Header.Get("X-Kannel-Udh")
	if udh == "" {
		log.Println("No UDH found")
		return c.String(http.StatusBadRequest, "No UDH found")
	}
	udh, err = url.QueryUnescape(udh)
	if err != nil {
		log.Println("Error unescaping UDH:", err)
		return c.String(http.StatusBadRequest, "Invalid UDH")
	}

	smsFrom := c.Request().Header.Get("X-Kannel-From")
	if smsFrom == "" {
		log.Println("No sender found")
		return c.String(http.StatusBadRequest, "No sender found")
	}
	smsBody, err := io.ReadAll(c.Request().Body)
	if err != nil {
		log.Println("Error reading request body:", err)
		return c.String(http.StatusBadRequest, "Invalid request body")
	}

	if c.Request().Header.Get("X-Kannel-Compress") == "0" {
		smsBody, err = hex.DecodeString(string(smsBody))
		if err != nil {
			log.Println("Error decoding HEX data:", err)
			return c.String(http.StatusBadRequest, "Invalid HEX data")
		}
	}

	err = s.wdpGateway.HandleIncomingSMS(smsFrom, []byte(udh), smsBody)

	if err != nil {
		log.Println("Error handling incoming SMS:", err)
		return c.String(http.StatusInternalServerError, "Internal server error")
	}

	return c.String(http.StatusOK, "OK")
}
