package smtp

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"fmt"
	"io"
	"math/big"
	"mime"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"net/textproto"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/miebyte/emailsender/internal/email"
)

func TestSenderDeliversContentTypesAndAcceptsFailedQuit(t *testing.T) {
	for _, tc := range []struct {
		name        string
		contentType string
		wantType    string
		body        string
	}{
		{"default plain text", "", "text/plain", "您好，<b>正文</b>\n第二行\n"},
		{"explicit plain text", "text/plain", "text/plain", "您好，正文\n第二行\n"},
		{"HTML", "text/html", "text/html", "<h1>您好</h1>\n<p>第二行 &amp; 正文</p>\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sender, server := newTestSMTP(t, smtpBehavior{rejectQuit: true})
			message := email.Message{
				From:        "通知 <visible@example.com>",
				To:          "收件人 <recipient@example.com>",
				Subject:     "测试主题",
				Body:        tc.body,
				ContentType: tc.contentType,
			}
			if err := sender.Send(context.Background(), message); err != nil {
				t.Fatal(err)
			}
			result := <-server.result
			if result.err != nil {
				t.Fatal(result.err)
			}
			if result.from != "MAIL FROM:<account@example.com>" || result.to != "RCPT TO:<recipient@example.com>" {
				t.Fatalf("unexpected envelope: from=%q to=%q", result.from, result.to)
			}
			parsed, err := mail.ReadMessage(bytes.NewReader(result.data))
			if err != nil {
				t.Fatal(err)
			}
			from, err := mail.ParseAddress(parsed.Header.Get("From"))
			if err != nil || from.Address != "visible@example.com" || from.Name != "通知" {
				t.Fatalf("unexpected From header: %q (%v)", parsed.Header.Get("From"), err)
			}
			subject, err := (&mime.WordDecoder{}).DecodeHeader(parsed.Header.Get("Subject"))
			if err != nil || subject != message.Subject {
				t.Fatalf("unexpected subject: %q (%v)", subject, err)
			}
			contentType, params, err := mime.ParseMediaType(parsed.Header.Get("Content-Type"))
			if err != nil || contentType != tc.wantType || params["charset"] != "UTF-8" {
				t.Fatalf("unexpected content type: %q (%v)", parsed.Header.Get("Content-Type"), err)
			}
			if parsed.Header.Get("MIME-Version") != "1.0" || parsed.Header.Get("Content-Transfer-Encoding") != "quoted-printable" {
				t.Fatal("missing MIME headers")
			}
			if _, err := parsed.Header.Date(); err != nil {
				t.Fatalf("invalid Date header: %v", err)
			}
			body, err := io.ReadAll(quotedprintable.NewReader(parsed.Body))
			if err != nil || strings.ReplaceAll(string(body), "\r\n", "\n") != message.Body {
				t.Fatalf("unexpected body: %q (%v)", body, err)
			}
		})
	}
}

func TestBuildMessageRejectsInvalidContentType(t *testing.T) {
	for _, contentType := range []string{"application/json", "text/html; charset=UTF-8", "text/html\r\nBcc: other@example.com"} {
		t.Run(contentType, func(t *testing.T) {
			message := testMessage()
			message.ContentType = contentType
			if _, _, err := buildMessage(message); !errors.Is(err, email.ErrInvalidMessage) {
				t.Fatalf("expected invalid message, got %v", err)
			}
		})
	}
}

func TestSenderReportsDataRejection(t *testing.T) {
	sender, server := newTestSMTP(t, smtpBehavior{rejectData: true})
	err := sender.Send(context.Background(), testMessage())
	if err == nil || !strings.Contains(err.Error(), "submit message") {
		t.Fatalf("expected final DATA rejection, got %v", err)
	}
	if strings.Contains(err.Error(), sender.config.Password) || strings.Contains(err.Error(), testMessage().Body) {
		t.Fatalf("error exposes email secrets: %v", err)
	}
	if result := <-server.result; result.err != nil {
		t.Fatal(result.err)
	}
}

func TestSenderCancellationInterruptsSMTP(t *testing.T) {
	sender, server := newTestSMTP(t, smtpBehavior{stallData: true})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	result := make(chan error, 1)
	go func() { result <- sender.Send(ctx, testMessage()) }()
	select {
	case <-server.atData:
	case <-time.After(3 * time.Second):
		t.Fatal("sender did not reach DATA")
	}
	cancel()
	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context cancellation, got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("SMTP exchange did not stop on cancellation")
	}
}

func TestSenderVerifiesTLSCertificate(t *testing.T) {
	sender, _ := newTestSMTP(t, smtpBehavior{})
	sender.tlsConfig.RootCAs = nil
	err := sender.Send(context.Background(), testMessage())
	var certError *tls.CertificateVerificationError
	if !errors.As(err, &certError) {
		t.Fatalf("expected certificate verification error, got %v", err)
	}
}

func TestSenderTimeoutInterruptsSMTP(t *testing.T) {
	sender, server := newTestSMTP(t, smtpBehavior{stallData: true})
	sender.config.TimeoutSeconds = 1
	err := sender.Send(context.Background(), testMessage())
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected timeout, got %v", err)
	}
	<-server.atData
}

func TestConfigRejectsInvalidValues(t *testing.T) {
	valid := Config{Host: "smtp.example.com", Port: 465, Username: "account@example.com", Password: "test-password"}
	for _, tc := range []struct {
		name   string
		change func(*Config)
	}{
		{"server", func(c *Config) { c.Host = "" }},
		{"port", func(c *Config) { c.Port = 65536 }},
		{"negative port", func(c *Config) { c.Port = -1 }},
		{"account", func(c *Config) { c.Username = "invalid" }},
		{"account newline", func(c *Config) { c.Username += "\r\n" }},
		{"password", func(c *Config) { c.Password = "" }},
		{"timeout", func(c *Config) { c.TimeoutSeconds = -1 }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			config := valid
			tc.change(&config)
			if _, err := NewSender(config); err == nil {
				t.Fatal("expected invalid configuration error")
			}
		})
	}
	sender, err := NewSender(valid)
	if err != nil || sender.config.TimeoutSeconds != 30 {
		t.Fatalf("default timeout was not applied: %v", err)
	}
	valid.Port = 0
	sender, err = NewSender(valid)
	if err != nil || sender.config.Port != 465 {
		t.Fatalf("default port was not applied: %v", err)
	}
}

func testMessage() email.Message {
	return email.Message{From: "from@example.com", To: "to@example.com", Subject: "Subject", Body: "private message body"}
}

type smtpBehavior struct {
	rejectData bool
	rejectQuit bool
	stallData  bool
}

type smtpResult struct {
	from string
	to   string
	data []byte
	err  error
}

type testSMTP struct {
	result chan smtpResult
	atData chan struct{}
}

func newTestSMTP(t *testing.T, behavior smtpBehavior) (*Sender, *testSMTP) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "SMTP test"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
		KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses: []net.IP{net.ParseIP("127.0.0.1")},
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	listener, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{Certificates: []tls.Certificate{{Certificate: [][]byte{der}, PrivateKey: key}}, MinVersion: tls.VersionTLS12})
	if err != nil {
		t.Fatal(err)
	}
	_, port, _ := net.SplitHostPort(listener.Addr().String())
	portNumber, err := strconv.Atoi(port)
	if err != nil {
		t.Fatal(err)
	}
	sender, err := NewSender(Config{Host: "127.0.0.1", Port: portNumber, Username: "Account <account@example.com>", Password: "test-password"})
	if err != nil {
		t.Fatal(err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	sender.tlsConfig.RootCAs = x509.NewCertPool()
	sender.tlsConfig.RootCAs.AddCert(cert)
	server := &testSMTP{result: make(chan smtpResult, 1), atData: make(chan struct{})}
	done := make(chan struct{})
	go func() {
		defer close(done)
		result := smtpResult{}
		defer func() { server.result <- result }()
		conn, err := listener.Accept()
		if err != nil {
			result.err = err
			return
		}
		defer conn.Close()
		_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
		client := textproto.NewConn(conn)
		if err := client.PrintfLine("220 localhost SMTP"); err != nil {
			result.err = err
			return
		}
		for {
			line, err := client.ReadLine()
			if err != nil {
				result.err = err
				return
			}
			switch {
			case strings.HasPrefix(line, "EHLO "):
				err = client.PrintfLine("250-localhost\r\n250 AUTH PLAIN")
			case strings.HasPrefix(line, "AUTH PLAIN "):
				err = client.PrintfLine("235 authenticated")
			case strings.HasPrefix(line, "MAIL FROM:"):
				result.from = line
				err = client.PrintfLine("250 sender accepted")
			case strings.HasPrefix(line, "RCPT TO:"):
				result.to = line
				err = client.PrintfLine("250 recipient accepted")
			case line == "DATA":
				close(server.atData)
				if behavior.stallData {
					_, _ = client.ReadLine()
					return
				}
				if err = client.PrintfLine("354 send message"); err == nil {
					result.data, err = client.ReadDotBytes()
				}
				if err == nil {
					if behavior.rejectData {
						err = client.PrintfLine("550 rejected")
						return
					}
					err = client.PrintfLine("250 accepted")
				}
			case line == "QUIT":
				if behavior.rejectQuit {
					err = client.PrintfLine("500 cannot quit")
				} else {
					err = client.PrintfLine("221 bye")
				}
				result.err = err
				return
			default:
				err = fmt.Errorf("unexpected SMTP command")
			}
			if err != nil {
				result.err = err
				return
			}
		}
	}()
	t.Cleanup(func() {
		_ = listener.Close()
		select {
		case <-done:
		case <-time.After(6 * time.Second):
			t.Error("SMTP test server did not stop")
		}
	})
	return sender, server
}
