// Package mailer sends verification messages over authenticated, encrypted SMTP.
package mailer

import (
	"context"
	"crypto/tls"
	"fmt"
	"mime"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"net/smtp"
	"strings"
	"time"
)

type Config struct {
	Host     string `json:"host"`
	Port     string `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
	From     string `json:"from"`
	TLSMode  string `json:"tls_mode"`
}

type Sender struct {
	config    Config
	tlsConfig *tls.Config
}

// New returns nil when SMTP is not configured. Partial configuration is an error.
func New(cfg Config) (*Sender, error) {
	if cfg.Host == "" && cfg.Username == "" && cfg.Password == "" && cfg.From == "" {
		return nil, nil
	}
	if cfg.Host == "" || cfg.Username == "" || cfg.Password == "" {
		return nil, fmt.Errorf("SMTP host, username and password are required")
	}
	if cfg.Port == "" {
		cfg.Port = "587"
	}
	if cfg.TLSMode == "" {
		cfg.TLSMode = "starttls"
	}
	if cfg.TLSMode != "starttls" && cfg.TLSMode != "tls" {
		return nil, fmt.Errorf("SMTP TLS mode must be starttls or tls")
	}
	if cfg.From == "" {
		cfg.From = cfg.Username
	}
	if !ValidAddress(cfg.From) {
		return nil, fmt.Errorf("SMTP from must be a single email address")
	}
	return &Sender{config: cfg, tlsConfig: &tls.Config{ServerName: cfg.Host, MinVersion: tls.VersionTLS12}}, nil
}

func ValidAddress(value string) bool {
	address, err := mail.ParseAddress(value)
	return err == nil && address.Address == value && len(value) <= 190 && !strings.ContainsAny(value, "\r\n")
}

func (s *Sender) SendCode(ctx context.Context, to, purpose, code string) error {
	if !ValidAddress(to) || len(code) != 6 || strings.Trim(code, "0123456789") != "" {
		return fmt.Errorf("invalid verification message")
	}
	action := "注册账号"
	if purpose == "reset" {
		action = "找回密码"
	} else if purpose != "register" {
		return fmt.Errorf("invalid verification purpose")
	}
	return s.send(ctx, to, "星域"+action+"验证码", fmt.Sprintf("您正在%s，验证码为：%s\r\n验证码 10 分钟内有效，请勿向他人透露。如非本人操作，请忽略此邮件。\r\n", action, code))
}

func (s *Sender) SendTest(ctx context.Context, to string) error {
	return s.send(ctx, to, "星域邮箱发送测试", "这是一封由管理员主动发送的测试邮件，用于确认邮箱发送配置。此邮件不包含登录或注册验证码。\r\n")
}

func (s *Sender) send(ctx context.Context, to, subject, text string) error {
	if !ValidAddress(to) {
		return fmt.Errorf("invalid recipient")
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", net.JoinHostPort(s.config.Host, s.config.Port))
	if err != nil {
		return fmt.Errorf("SMTP connect: %w", err)
	}
	defer conn.Close()
	rawConn := conn
	deadline, _ := ctx.Deadline()
	if err = conn.SetDeadline(deadline); err != nil {
		return err
	}
	stop := context.AfterFunc(ctx, func() { _ = rawConn.Close() })
	defer stop()
	if s.config.TLSMode == "tls" {
		tlsConn := tls.Client(conn, s.tlsConfig)
		if err = tlsConn.HandshakeContext(ctx); err != nil {
			return fmt.Errorf("SMTP TLS: %w", err)
		}
		conn = tlsConn
	}
	client, err := smtp.NewClient(conn, s.config.Host)
	if err != nil {
		return fmt.Errorf("SMTP greeting: %w", err)
	}
	defer client.Close()
	if s.config.TLSMode == "starttls" {
		if ok, _ := client.Extension("STARTTLS"); !ok {
			return fmt.Errorf("SMTP server does not offer required STARTTLS")
		}
		if err = client.StartTLS(s.tlsConfig); err != nil {
			return fmt.Errorf("SMTP STARTTLS: %w", err)
		}
	}
	if err = client.Auth(smtp.PlainAuth("", s.config.Username, s.config.Password, s.config.Host)); err != nil {
		return fmt.Errorf("SMTP authentication failed")
	}
	if err = client.Mail(s.config.From); err != nil {
		return fmt.Errorf("SMTP sender rejected")
	}
	if err = client.Rcpt(to); err != nil {
		return fmt.Errorf("SMTP recipient rejected")
	}
	writer, err := client.Data()
	if err != nil {
		return fmt.Errorf("SMTP DATA failed")
	}
	var body strings.Builder
	encodedBody := quotedprintable.NewWriter(&body)
	_, _ = encodedBody.Write([]byte(text))
	_ = encodedBody.Close()
	message := "From: " + (&mail.Address{Name: "星域", Address: s.config.From}).String() + "\r\n" +
		"To: " + (&mail.Address{Address: to}).String() + "\r\n" +
		"Subject: " + mime.BEncoding.Encode("UTF-8", subject) + "\r\n" +
		"Date: " + time.Now().Format(time.RFC1123Z) + "\r\n" +
		"MIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\nContent-Transfer-Encoding: quoted-printable\r\n\r\n" + body.String()
	if _, err = writer.Write([]byte(message)); err != nil {
		return fmt.Errorf("SMTP message write failed")
	}
	if err = writer.Close(); err != nil {
		return fmt.Errorf("SMTP message not accepted")
	}
	// DATA's final 250 is acceptance. A later QUIT failure must not cause a resend.
	_ = client.Quit()
	return nil
}
