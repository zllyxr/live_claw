package mailer

import (
	"bufio"
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"mime/quotedprintable"
	"net"
	"net/http/httptest"
	"net/mail"
	"net/textproto"
	"strings"
	"testing"
	"time"
)

func TestSMTPSendsOnlyAfterTLSAndRequiresMessageAcceptance(t *testing.T) {
	for _, mode := range []string{"tls", "starttls"} {
		for _, reject := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/reject=%t", mode, reject), func(t *testing.T) {
				certServer := httptest.NewTLSServer(nil)
				serverTLS := certServer.TLS.Clone()
				roots := x509.NewCertPool()
				roots.AddCert(certServer.Certificate())
				certServer.Close()
				listener, err := net.Listen("tcp", "127.0.0.1:0")
				if err != nil {
					t.Fatal(err)
				}
				defer listener.Close()
				done := make(chan error, 1)
				message := make(chan string, 1)
				go func() {
					conn, err := listener.Accept()
					if err != nil {
						done <- err
						return
					}
					defer conn.Close()
					_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
					encrypted := mode == "tls"
					if encrypted {
						conn = tls.Server(conn, serverTLS)
					}
					reader := textproto.NewReader(bufio.NewReader(conn))
					fmt.Fprint(conn, "220 test SMTP\r\n")
					for {
						line, err := reader.ReadLine()
						if err != nil {
							if reject && err == io.EOF {
								done <- nil
							} else {
								done <- err
							}
							return
						}
						switch {
						case strings.HasPrefix(line, "EHLO"):
							if encrypted {
								fmt.Fprint(conn, "250-test\r\n250 AUTH PLAIN\r\n")
							} else {
								fmt.Fprint(conn, "250-test\r\n250 STARTTLS\r\n")
							}
						case line == "STARTTLS":
							fmt.Fprint(conn, "220 ready\r\n")
							conn = tls.Server(conn, serverTLS)
							reader = textproto.NewReader(bufio.NewReader(conn))
							encrypted = true
						case strings.HasPrefix(line, "AUTH"):
							if !encrypted {
								done <- fmt.Errorf("credentials before TLS")
								return
							}
							fmt.Fprint(conn, "235 authenticated\r\n")
						case strings.HasPrefix(line, "MAIL FROM:<sender@example.com>"), line == "RCPT TO:<recipient@example.com>":
							fmt.Fprint(conn, "250 ok\r\n")
						case line == "DATA":
							fmt.Fprint(conn, "354 send data\r\n")
							body, err := reader.ReadDotBytes()
							if err != nil {
								done <- err
								return
							}
							message <- string(body)
							if reject {
								fmt.Fprint(conn, "550 rejected\r\n")
							} else {
								fmt.Fprint(conn, "250 accepted\r\n")
							}
						case line == "QUIT":
							fmt.Fprint(conn, "221 bye\r\n")
							done <- nil
							return
						default:
							done <- fmt.Errorf("unexpected SMTP command %q", line)
							return
						}
					}
				}()
				host, port, _ := net.SplitHostPort(listener.Addr().String())
				sender, err := New(Config{Host: host, Port: port, Username: "sender@example.com", Password: "test-password", TLSMode: mode})
				if err != nil {
					t.Fatal(err)
				}
				sender.tlsConfig.RootCAs = roots
				err = sender.SendCode(context.Background(), "recipient@example.com", "register", "042519")
				if (err != nil) != reject {
					t.Fatalf("send error=%v, reject=%t", err, reject)
				}
				if err = <-done; err != nil {
					t.Fatal(err)
				}
				raw := <-message
				parsed, err := mail.ReadMessage(strings.NewReader(raw))
				if err != nil {
					t.Fatal(err)
				}
				decoded, err := io.ReadAll(quotedprintable.NewReader(parsed.Body))
				if err != nil {
					t.Fatal(err)
				}
				body := string(decoded)
				if !strings.Contains(body, "042519") || !strings.Contains(body, "10 分钟") || strings.Contains(body, "test-password") {
					t.Fatalf("unexpected body: %s", body)
				}
			})
		}
	}
}

func TestSMTPRefusesUnencryptedServer(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	done := make(chan error, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			done <- err
			return
		}
		defer conn.Close()
		_ = conn.SetDeadline(time.Now().Add(3 * time.Second))
		reader := bufio.NewReader(conn)
		fmt.Fprint(conn, "220 test\r\n")
		_, _ = reader.ReadString('\n')
		fmt.Fprint(conn, "250 test\r\n")
		line, _ := reader.ReadString('\n')
		if strings.HasPrefix(line, "AUTH") {
			done <- fmt.Errorf("sent credentials without TLS")
			return
		}
		done <- nil
	}()
	host, port, _ := net.SplitHostPort(listener.Addr().String())
	sender, _ := New(Config{Host: host, Port: port, Username: "sender@example.com", Password: "test"})
	if err = sender.SendCode(context.Background(), "recipient@example.com", "reset", "123456"); err == nil || !strings.Contains(err.Error(), "STARTTLS") {
		t.Fatalf("expected STARTTLS rejection: %v", err)
	}
	if err = <-done; err != nil {
		t.Fatal(err)
	}
}

func TestSMTPConfigurationAndAddressValidation(t *testing.T) {
	if s, err := New(Config{}); err != nil || s != nil {
		t.Fatal("empty config should disable SMTP")
	}
	for _, cfg := range []Config{{Host: "smtp.gmail.com"}, {Host: "smtp.gmail.com", Username: "a@example.com", Password: "test", TLSMode: "none"}} {
		if _, err := New(cfg); err == nil {
			t.Fatal("incomplete or insecure config accepted")
		}
	}
	for _, address := range []string{"a@example.com\r\nBcc:evil@example.com", "Name <a@example.com>", "invalid"} {
		if ValidAddress(address) {
			t.Fatalf("accepted %q", address)
		}
	}
}
